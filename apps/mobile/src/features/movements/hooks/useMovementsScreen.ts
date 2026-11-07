import { router } from 'expo-router';
import { useMemo } from 'react';
import { useShallow } from 'zustand/react/shallow';

import type { Movement } from '@/shared/api/types';
import { useMovements } from '@/shared/queries/movements';
import { useActiveOrg } from '@/shared/queries/orgs';
import { useMovementFiltersStore } from '@/shared/stores/useMovementFiltersStore';

import { activeFilterCount, toMovementFilters } from '../helpers/filters';

export function useMovementsScreen() {
  const { org } = useActiveOrg();
  const draft = useMovementFiltersStore(useShallow((state) => ({ type: state.type, status: state.status, currency: state.currency, range: state.range })));
  const setFilter = useMovementFiltersStore((state) => state.setFilter);
  const reset = useMovementFiltersStore((state) => state.reset);
  // Filters are rebuilt only when the draft changes, so the query key stays stable.
  const filters = useMemo(() => toMovementFilters(draft, new Date()), [draft]);
  const movements = useMovements(org?.id, filters);

  return {
    draft,
    setFilter,
    reset,
    activeCount: activeFilterCount(draft),
    movements,
    items: movements.data?.pages.flatMap((page) => page.items) ?? [],
    loadMore: () => {
      if (movements.hasNextPage && !movements.isFetchingNextPage) void movements.fetchNextPage();
    },
    open: (movement: Movement) => router.push({ pathname: '/movements/[id]', params: { id: movement.id } }),
  };
}
