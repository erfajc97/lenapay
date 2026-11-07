import { create } from 'zustand';

import { ApiError } from '@/shared/api/errors';

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
}

/** Android never reports a Modal dismissal and a Modal can vanish without one: settle anyway. */
export const DISMISS_FALLBACK_MS = 800;

let fallbackTimer: ReturnType<typeof setTimeout> | null = null;

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
        set({ open: true, pending: { resolve, reject, outcome: null } });
      }),
    complete: (token) => close({ kind: 'token', token }),
    cancel: () => close({ kind: 'cancelled' }),
    dismissed: settleClosed,
  };
});

export const requestStepUp = () => useStepUpStore.getState().request();
