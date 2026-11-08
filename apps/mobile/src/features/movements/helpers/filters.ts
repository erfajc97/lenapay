import type { MovementFilters } from '@/shared/api/types';
import { businessDate } from '@/shared/helpers/days';
import type { MovementFilterDraft } from '@/shared/stores/useMovementFiltersStore';

const QUICK_RANGE_DAYS = { today: 1, '7d': 7, '30d': 30 } as const;

function rangeStart(draft: MovementFilterDraft, now: Date): string | null {
  if (draft.range === 'all') return null;
  if (draft.range === 'custom') return draft.customFrom;
  if (draft.range === 'month') return businessDate(new Date(now.getFullYear(), now.getMonth(), 1));
  // Calendar arithmetic (not milliseconds) so a DST change never drops a day.
  return businessDate(new Date(now.getFullYear(), now.getMonth(), now.getDate() - (QUICK_RANGE_DAYS[draft.range] - 1)));
}

/** Screen filters → API query (R-MOV-1). Both ends are inclusive business dates. */
export function toMovementFilters(draft: MovementFilterDraft, now: Date): MovementFilters {
  const filters: MovementFilters = {};
  if (draft.type) filters.type = draft.type;
  if (draft.status) filters.status = draft.status;
  if (draft.currency) filters.currency = draft.currency;
  const from = rangeStart(draft, now);
  if (from) filters.from = from;
  if (draft.range === 'custom' && draft.customTo) filters.to = draft.customTo;
  return filters;
}

export function activeFilterCount(draft: MovementFilterDraft): number {
  return [draft.type, draft.status, draft.currency].filter(Boolean).length + (draft.range === 'all' ? 0 : 1);
}

export function customRangeError(from: string | null, to: string | null): 'movements.filter.rangeInvalid' | 'movements.filter.rangeOrder' | null {
  if (!from || !to) return 'movements.filter.rangeInvalid';
  return from > to ? 'movements.filter.rangeOrder' : null;
}
