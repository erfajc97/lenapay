import { create } from 'zustand';

/** The one toast on screen. Only mutations call `show` (M2). */
type ToastKind = 'success' | 'error' | 'info';

interface Toast {
  id: number;
  kind: ToastKind;
  message: string;
}

interface ToastState {
  toast: Toast | null;
  show(kind: ToastKind, message: string): void;
  dismiss(id: number): void;
}

let nextToastId = 0;

export const useToastStore = create<ToastState>()((set) => ({
  toast: null,
  show: (kind, message) => {
    nextToastId += 1;
    set({ toast: { id: nextToastId, kind, message } });
  },
  dismiss: (id) => set((state) => (state.toast?.id === id ? { toast: null } : state)),
}));

/** Outside React (mutation callbacks) the toast is raised through the store's actions. */
export const showToast = (kind: ToastKind, message: string) => useToastStore.getState().show(kind, message);
