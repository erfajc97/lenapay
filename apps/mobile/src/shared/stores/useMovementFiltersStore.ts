import { create } from 'zustand';

import type { Currency, MovementStatus, MovementType } from '@/shared/api/types';

/** Draft filters of the movements screen (client state, M17). */
export type DateRange = 'all' | '7d' | '30d' | '90d';

export interface MovementFilterDraft {
  type: MovementType | null;
  status: MovementStatus | null;
  currency: Currency | null;
  range: DateRange;
}

interface MovementFiltersState extends MovementFilterDraft {
  setFilter<K extends keyof MovementFilterDraft>(key: K, value: MovementFilterDraft[K]): void;
  reset(): void;
}

const EMPTY: MovementFilterDraft = { type: null, status: null, currency: null, range: 'all' };

export const useMovementFiltersStore = create<MovementFiltersState>()((set) => ({
  ...EMPTY,
  setFilter: (key, value) => set({ [key]: value } as Partial<MovementFilterDraft>),
  reset: () => set(EMPTY),
}));
