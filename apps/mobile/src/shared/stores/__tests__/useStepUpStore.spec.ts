import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { runMoneyAction } from '@/shared/stepup/moneyAction';

import { DISMISS_FALLBACK_MS, PRESENT_TIMEOUT_MS, requestStepUp, useStepUpStore } from '../useStepUpStore';

/** Records whether a promise settled, without awaiting it (awaiting would hide a pending one). */
function track<T>(promise: Promise<T>) {
  const state: { settled: boolean; value?: T; error?: unknown } = { settled: false };
  promise.then(
    (value) => Object.assign(state, { settled: true, value }),
    (error: unknown) => Object.assign(state, { settled: true, error }),
  );
  return state;
}

const store = () => useStepUpStore.getState();

// Why: a money mutation must never run without a step-up token, and a closed sheet must
// fail the mutation instead of leaving it hanging forever (R-ID-3).
describe('useStepUpStore', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    // Leaves no request behind for the next case.
    store().cancel();
    store().dismissed();
    vi.useRealTimers();
  });

  it('opens the sheet and resolves the waiting mutation with the token once the sheet is gone', async () => {
    const pending = track(requestStepUp());
    expect(store().open).toBe(true);
    store().complete('su-1');
    expect(store().open).toBe(false);
    store().dismissed();
    await vi.advanceTimersByTimeAsync(0);
    expect(pending).toMatchObject({ settled: true, value: 'su-1' });
  });

  it('rejects with stepup.cancelled when the person closes the sheet', async () => {
    const pending = requestStepUp();
    store().cancel();
    store().dismissed();
    await expect(pending).rejects.toMatchObject({ code: 'stepup.cancelled' });
  });

  it('cancels an older request when a new one opens', async () => {
    const first = requestStepUp();
    const second = requestStepUp();
    await expect(first).rejects.toMatchObject({ code: 'stepup.cancelled' });
    store().complete('su-2');
    store().dismissed();
    await expect(second).resolves.toBe('su-2');
  });

  // Why: the freeze on iOS. The token used to resolve while the Modal was still sliding out,
  // so the mutation's router.back() dismissed the "move" native modal underneath a Modal that
  // was mid-dismissal; UIKit left an invisible presentation on top and every touch was lost.
  it('keeps the token until the sheet reports it is fully dismissed', async () => {
    const pending = track(requestStepUp());
    store().complete('su-3');
    await vi.advanceTimersByTimeAsync(DISMISS_FALLBACK_MS - 1);
    expect(pending.settled).toBe(false);
    store().dismissed();
    await vi.advanceTimersByTimeAsync(0);
    expect(pending).toMatchObject({ settled: true, value: 'su-3' });
  });

  it('keeps the cancellation until the sheet is fully dismissed', async () => {
    const pending = track(requestStepUp());
    store().cancel();
    await vi.advanceTimersByTimeAsync(0);
    expect(pending.settled).toBe(false);
    store().dismissed();
    await vi.advanceTimersByTimeAsync(0);
    expect(pending.error).toMatchObject({ code: 'stepup.cancelled' });
  });

  // Why: Android never reports the dismissal and a Modal can vanish without it; the money
  // action must still settle instead of leaving the button spinning forever.
  it('settles by itself when the dismissal is never reported, and not a moment earlier', async () => {
    const completed = track(requestStepUp());
    store().complete('su-4');
    await vi.advanceTimersByTimeAsync(DISMISS_FALLBACK_MS - 1);
    expect(completed.settled).toBe(false);
    await vi.advanceTimersByTimeAsync(1);
    expect(completed).toMatchObject({ settled: true, value: 'su-4' });

    const cancelled = track(requestStepUp());
    store().cancel();
    await vi.advanceTimersByTimeAsync(DISMISS_FALLBACK_MS);
    expect(cancelled.error).toMatchObject({ code: 'stepup.cancelled' });
  });

  it('ignores a dismissal while the sheet is open waiting for the person', async () => {
    const pending = track(requestStepUp());
    store().dismissed();
    await vi.advanceTimersByTimeAsync(DISMISS_FALLBACK_MS);
    expect(pending.settled).toBe(false);
    expect(store().open).toBe(true);
  });

  it('hands an already confirmed token to its mutation when a new request opens during the dismissal', async () => {
    const first = track(requestStepUp());
    store().complete('su-5');
    const second = track(requestStepUp());
    await vi.advanceTimersByTimeAsync(0);
    expect(first).toMatchObject({ settled: true, value: 'su-5' });
    expect(second.settled).toBe(false);
    expect(store().open).toBe(true);
  });

  it('settles only once even if the dismissal arrives after the fallback', async () => {
    const pending = track(requestStepUp());
    store().complete('su-6');
    await vi.advanceTimersByTimeAsync(DISMISS_FALLBACK_MS);
    const next = track(requestStepUp());
    store().dismissed();
    await vi.advanceTimersByTimeAsync(0);
    expect(pending.value).toBe('su-6');
    expect(next.settled).toBe(false);
  });

  // Why: the API call and the navigation of its onSuccess must start only after the sheet left
  // the screen; this pins the whole money path, not just the store.
  it('does not call the API of a money action before the sheet is dismissed', async () => {
    const call = vi.fn(async () => 'ok');
    const action = track(runMoneyAction({ scope: 'transfer:org-1', payload: { amount: 100_050 }, call }));
    store().complete('su-7');
    await vi.advanceTimersByTimeAsync(0);
    expect(call).not.toHaveBeenCalled();
    store().dismissed();
    await vi.advanceTimersByTimeAsync(0);
    expect(call).toHaveBeenCalledWith({ stepUpToken: 'su-7', idempotencyKey: expect.any(String) });
    expect(action).toMatchObject({ settled: true, value: 'ok' });
  });

  // Why: the iOS freeze on Mover entre cuentas. The sheet asked to open but UIKit never put it
  // on screen (a root Modal cannot present over a native modal screen), so the money button
  // spun forever and nothing reached the API. A sheet that does not confirm it is visible must
  // fail the action with a code the toast can explain, and reset the button.
  it('fails with stepup.not_shown and closes when the sheet never confirms it is on screen', async () => {
    const pending = track(requestStepUp());
    await vi.advanceTimersByTimeAsync(PRESENT_TIMEOUT_MS - 1);
    expect(pending.settled).toBe(false);
    await vi.advanceTimersByTimeAsync(1);
    expect(pending.error).toMatchObject({ code: 'stepup.not_shown' });
    expect(store().open).toBe(false);
    expect(store().pending).toBeNull();
  });

  it('keeps waiting for the person once the sheet confirmed it is on screen', async () => {
    const pending = track(requestStepUp());
    store().presented();
    await vi.advanceTimersByTimeAsync(PRESENT_TIMEOUT_MS * 3);
    expect(pending.settled).toBe(false);
    expect(store().open).toBe(true);
    store().complete('su-8');
    store().dismissed();
    await vi.advanceTimersByTimeAsync(0);
    expect(pending.value).toBe('su-8');
  });

  it('does not let the presentation timer of a replaced request fail the new one', async () => {
    track(requestStepUp());
    await vi.advanceTimersByTimeAsync(PRESENT_TIMEOUT_MS - 100);
    const second = track(requestStepUp());
    await vi.advanceTimersByTimeAsync(200);
    expect(second.settled).toBe(false);
    expect(store().open).toBe(true);
  });

  it('keeps the presentation window around 2 s', () => {
    expect(PRESENT_TIMEOUT_MS).toBeGreaterThanOrEqual(1_500);
    expect(PRESENT_TIMEOUT_MS).toBeLessThanOrEqual(3_000);
  });
});
