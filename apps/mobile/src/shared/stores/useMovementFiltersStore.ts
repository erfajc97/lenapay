import { create } from 'zustand';

import type { Currency, MovementStatus, MovementType } from '@/shared/api/types';

/** Draft filters of the movements screen (client state, M17). */
export type DateRange = 'all' | 'today' | '7d' | '30d' | 'month' | 'custom';

export interface MovementFilterDraft {
  type: MovementType | null;
  status: MovementStatus | null;
  currency: Currency | null;
  range: DateRange;
  /** Business dates (YYYY-MM-DD), set only with `range: 'custom'`. */
  customFrom: string | null;
  customTo: string | null;
}

interface MovementFiltersState extends MovementFilterDraft {
  setFilter<K extends 'type' | 'status' | 'currency' | 'range'>(key: K, value: MovementFilterDraft[K]): void;
  setCustomRange(from: string, to: string): void;
  /** Opens the list with exactly these filters (the home balance tiles). */
  showOnly(filters: Partial<Pick<MovementFilterDraft, 'type' | 'status' | 'currency'>>): void;
  reset(): void;
}

export const EMPTY_FILTERS: MovementFilterDraft = { type: null, status: null, currency: null, range: 'all', customFrom: null, customTo: null };

export const useMovementFiltersStore = create<MovementFiltersState>()((set) => ({
  ...EMPTY_FILTERS,
  setFilter: (key, value) => set(key === 'range' ? { range: value as DateRange, customFrom: null, customTo: null } : ({ [key]: value } as Partial<MovementFilterDraft>)),
  setCustomRange: (customFrom, customTo) => set({ range: 'custom', customFrom, customTo }),
  showOnly: (filters) => set({ ...EMPTY_FILTERS, ...filters }),
  reset: () => set(EMPTY_FILTERS),
}));
