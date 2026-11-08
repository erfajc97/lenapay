import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { KEYBOARD_HIDE_MAX_MS, type KeyboardLike, waitForKeyboardHidden } from '../keyboard';

// Why: the step-up sheet opened while the numeric keypad of the amount field was still up;
// the sheet must present only once the keyboard is gone, but must never wait forever for a
// keyboardDidHide event that may not come.
function fakeKeyboard(visible: boolean) {
  const listeners = new Set<() => void>();
  const keyboard: KeyboardLike = {
    isVisible: () => visible,
    dismiss: vi.fn(),
    addListener: (_event, listener) => {
      listeners.add(listener);
      return { remove: () => listeners.delete(listener) };
    },
  };
  return { keyboard, hide: () => listeners.forEach((listener) => listener()), listeners };
}

function track(promise: Promise<void>) {
  const state = { settled: false };
  void promise.then(() => {
    state.settled = true;
  });
  return state;
}

describe('waitForKeyboardHidden', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('resolves at once when no keyboard is up', async () => {
    const { keyboard } = fakeKeyboard(false);
    const done = track(waitForKeyboardHidden(keyboard));
    await vi.advanceTimersByTimeAsync(0);
    expect(done.settled).toBe(true);
  });

  it('dismisses the keyboard and resolves on keyboardDidHide, removing its listener', async () => {
    const fake = fakeKeyboard(true);
    const done = track(waitForKeyboardHidden(fake.keyboard));
    expect(fake.keyboard.dismiss).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(0);
    expect(done.settled).toBe(false);
    fake.hide();
    await vi.advanceTimersByTimeAsync(0);
    expect(done.settled).toBe(true);
    expect(fake.listeners.size).toBe(0);
  });

  it('gives up waiting after the cap when the event never comes, and not a moment earlier', async () => {
    const fake = fakeKeyboard(true);
    const done = track(waitForKeyboardHidden(fake.keyboard));
    await vi.advanceTimersByTimeAsync(KEYBOARD_HIDE_MAX_MS - 1);
    expect(done.settled).toBe(false);
    await vi.advanceTimersByTimeAsync(1);
    expect(done.settled).toBe(true);
    expect(fake.listeners.size).toBe(0);
  });
});
