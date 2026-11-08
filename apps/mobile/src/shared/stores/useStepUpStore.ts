import { create } from 'zustand';

import { ApiError } from '@/shared/api/errors';
import { devLog } from '@/shared/reporting/reporting';

/**
 * The step-up sheet as a promise (R-ID-3): a money mutation awaits `requestStepUp()`, the
 * sheet resolves it with a fresh step-up token or rejects it with `stepup.cancelled`.
 *
 * The answer is held until the sheet reports it is fully off screen (`dismissed`), because the
 * mutation navigates on success: on iOS, popping or dismissing a screen while a Modal is still
 * sliding out leaves an invisible presentation on top and the app stops taking touches.
 */
type Outcome = { kind: 'token'; token: string } | { kind: 'cancelled' };

interface PendingStepUp {
  resolve(token: string): void;
  reject(error: ApiError): void;
  /** Set when the sheet closes; delivered on dismissal (or by the fallback timer). */
  outcome: Outcome | null;
}

interface StepUpState {
  open: boolean;
  pending: PendingStepUp | null;
  request(): Promise<string>;
  complete(token: string): void;
  cancel(): void;
  /** The sheet left the screen: deliver the held outcome to the waiting mutation. */
  dismissed(): void;
  /** The sheet is visibly on screen: stop the presentation timer. */
  presented(): void;
}

/** Android never reports a Modal dismissal and a Modal can vanish without one: settle anyway. */
export const DISMISS_FALLBACK_MS = 800;

/**
 * A sheet that is not on screen this long after the request never will be (UIKit refused to
 * present it): the action fails with `stepup.not_shown` instead of spinning forever.
 */
export const PRESENT_TIMEOUT_MS = 2_000;

let fallbackTimer: ReturnType<typeof setTimeout> | null = null;
let presentTimer: ReturnType<typeof setTimeout> | null = null;
/** The sheet confirmed it is on screen and has not closed since. */
let sheetShown = false;

function clearPresentTimer(): void {
  if (presentTimer !== null) clearTimeout(presentTimer);
  presentTimer = null;
}

function deliver(pending: PendingStepUp, outcome: Outcome): void {
  if (outcome.kind === 'token') pending.resolve(outcome.token);
  else pending.reject(new ApiError('stepup.cancelled', 0));
}

export const useStepUpStore = create<StepUpState>()((set, get) => {
  function settleClosed(): void {
    const pending = get().pending;
    if (!pending?.outcome) return;
    if (fallbackTimer !== null) clearTimeout(fallbackTimer);
    fallbackTimer = null;
    set({ pending: null });
    deliver(pending, pending.outcome);
  }

  function close(outcome: Outcome): void {
    clearPresentTimer();
    sheetShown = false;
    const pending = get().pending;
    if (!pending || pending.outcome) {
      set({ open: false });
      return;
    }
    pending.outcome = outcome;
    set({ open: false });
    fallbackTimer = setTimeout(settleClosed, DISMISS_FALLBACK_MS);
  }

  return {
    open: false,
    pending: null,
    request: () =>
      new Promise<string>((resolve, reject) => {
        // A request still closing gets its answer now; one still waiting for the person is cancelled.
        if (get().pending?.outcome) settleClosed();
        get().pending?.reject(new ApiError('stepup.cancelled', 0));
        // A sheet already on screen stays there for the new request: it will not present again.
        const alreadyShown = get().open && sheetShown;
        const pending: PendingStepUp = { resolve, reject, outcome: null };
        set({ open: true, pending });
        devLog('stepup.requested');
        clearPresentTimer();
        if (alreadyShown) return;
        presentTimer = setTimeout(() => {
          presentTimer = null;
          if (get().pending !== pending) return;
          devLog('stepup.not_shown');
          sheetShown = false;
          set({ open: false, pending: null });
          reject(new ApiError('stepup.not_shown', 0));
        }, PRESENT_TIMEOUT_MS);
      }),
    complete: (token) => close({ kind: 'token', token }),
    cancel: () => close({ kind: 'cancelled' }),
    dismissed: settleClosed,
    presented: () => {
      if (!get().open) return;
      sheetShown = true;
      clearPresentTimer();
      devLog('stepup.presented');
    },
  };
});

export const requestStepUp = () => useStepUpStore.getState().request();
