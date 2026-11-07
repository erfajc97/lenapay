import { describe, expect, it } from 'vitest';

import { requestStepUp, useStepUpStore } from '../useStepUpStore';

// Why: a money mutation must never run without a step-up token, and a closed sheet must
// fail the mutation instead of leaving it hanging forever (R-ID-3).
describe('useStepUpStore', () => {
  it('opens the sheet and resolves the waiting mutation with the token', async () => {
    const pending = requestStepUp();
    expect(useStepUpStore.getState().open).toBe(true);
    useStepUpStore.getState().complete('su-1');
    await expect(pending).resolves.toBe('su-1');
    expect(useStepUpStore.getState().open).toBe(false);
  });

  it('rejects with stepup.cancelled when the person closes the sheet', async () => {
    const pending = requestStepUp();
    useStepUpStore.getState().cancel();
    await expect(pending).rejects.toMatchObject({ code: 'stepup.cancelled' });
  });

  it('cancels an older request when a new one opens', async () => {
    const first = requestStepUp();
    const second = requestStepUp();
    await expect(first).rejects.toMatchObject({ code: 'stepup.cancelled' });
    useStepUpStore.getState().complete('su-2');
    await expect(second).resolves.toBe('su-2');
  });
});
