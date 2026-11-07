import type { MovementFilters } from '@/shared/api/types';
import type { DateRange, MovementFilterDraft } from '@/shared/stores/useMovementFiltersStore';

const RANGE_DAYS: Record<Exclude<DateRange, 'all'>, number> = { '7d': 7, '30d': 30, '90d': 90 };
const DAY_MS = 86_400_000;

/** Screen filters → API query (R-MOV-1). Ranges start at local midnight N days ago. */
export function toMovementFilters(draft: MovementFilterDraft, now: Date): MovementFilters {
  const filters: MovementFilters = {};
  if (draft.type) filters.type = draft.type;
  if (draft.status) filters.status = draft.status;
  if (draft.currency) filters.currency = draft.currency;
  if (draft.range !== 'all') {
    const start = new Date(now.getTime() - (RANGE_DAYS[draft.range] - 1) * DAY_MS);
    start.setHours(0, 0, 0, 0);
    filters.from = start.toISOString();
  }
  return filters;
}

export function activeFilterCount(draft: MovementFilterDraft): number {
  return [draft.type, draft.status, draft.currency].filter(Boolean).length + (draft.range === 'all' ? 0 : 1);
}
