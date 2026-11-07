import { create } from 'zustand';

import { ApiError } from '@/shared/api/errors';

/**
 * The step-up sheet as a promise (R-ID-3): a money mutation awaits `requestStepUp()`, the
 * sheet resolves it with a fresh step-up token or rejects it with `stepup.cancelled`.
 */
interface PendingStepUp {
  resolve(token: string): void;
  reject(error: ApiError): void;
}

interface StepUpState {
  open: boolean;
  pending: PendingStepUp | null;
  request(): Promise<string>;
  complete(token: string): void;
  cancel(): void;
}

export const useStepUpStore = create<StepUpState>()((set, get) => ({
  open: false,
  pending: null,
  request: () =>
    new Promise<string>((resolve, reject) => {
      // A second request while the sheet is open replaces the first, which is cancelled.
      get().pending?.reject(new ApiError('stepup.cancelled', 0));
      set({ open: true, pending: { resolve, reject } });
    }),
  complete: (token) => {
    get().pending?.resolve(token);
    set({ open: false, pending: null });
  },
  cancel: () => {
    get().pending?.reject(new ApiError('stepup.cancelled', 0));
    set({ open: false, pending: null });
  },
}));

export const requestStepUp = () => useStepUpStore.getState().request();
