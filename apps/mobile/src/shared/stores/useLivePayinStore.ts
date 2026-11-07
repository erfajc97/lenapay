import { create } from 'zustand';

import type { Payin } from '@/shared/api/types';

/** The live payin banner (R-IN-3): the last payin received through the event stream. */
interface LivePayinState {
  payin: Payin | null;
  show(payin: Payin): void;
  dismiss(): void;
}

export const useLivePayinStore = create<LivePayinState>()((set) => ({
  payin: null,
  show: (payin) => set({ payin }),
  dismiss: () => set({ payin: null }),
}));
