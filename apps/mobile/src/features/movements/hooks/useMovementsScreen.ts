import { router } from 'expo-router';
import { useMemo, useState } from 'react';
import { useShallow } from 'zustand/react/shallow';

import type { Movement } from '@/shared/api/types';
import { useMovements } from '@/shared/queries/movements';
import { useActiveOrg } from '@/shared/queries/orgs';
import { type MovementFilterDraft, useMovementFiltersStore } from '@/shared/stores/useMovementFiltersStore';
import { formatDayInput, parseDayInput } from '@/shared/helpers/days';

import { activeFilterCount, customRangeError, toMovementFilters } from '../helpers/filters';

export type FilterKey = 'type' | 'status' | 'currency' | 'date';

export function useMovementsScreen() {
  const { org } = useActiveOrg();
  const draft = useMovementFiltersStore(
    useShallow((state) => ({ type: state.type, status: state.status, currency: state.currency, range: state.range, customFrom: state.customFrom, customTo: state.customTo })),
  );
  const setFilter = useMovementFiltersStore((state) => state.setFilter);
  const setCustomRange = useMovementFiltersStore((state) => state.setCustomRange);
  const reset = useMovementFiltersStore((state) => state.reset);
  const [openFilter, setOpenFilter] = useState<FilterKey | null>(null);
  // The sheet keeps its content while it slides out, so it remembers the last filter shown.
  const [shownFilter, setShownFilter] = useState<FilterKey>('type');
  const [customOpen, setCustomOpen] = useState(false);
  const [fromText, setFromText] = useState('');
  const [toText, setToText] = useState('');
  const [customSubmitted, setCustomSubmitted] = useState(false);
  // Filters are rebuilt only when the draft changes, so the query key stays stable.
  const filters = useMemo(() => toMovementFilters(draft, new Date()), [draft]);
  const movements = useMovements(org?.id, filters);
  const rangeError = customRangeError(parseDayInput(fromText), parseDayInput(toText));

  function closeFilter() {
    setOpenFilter(null);
    setCustomOpen(false);
    setCustomSubmitted(false);
  }

  return {
    draft,
    activeCount: activeFilterCount(draft),
    reset,
    openFilter,
    shownFilter,
    open: (key: FilterKey) => {
      setOpenFilter(key);
      setShownFilter(key);
      setCustomOpen(key === 'date' && draft.range === 'custom');
      setFromText(draft.customFrom ? formatDayInput(draft.customFrom) : '');
      setToText(draft.customTo ? formatDayInput(draft.customTo) : '');
    },
    closeFilter,
    /** Picking an option applies it at once and closes the sheet. */
    pick: <K extends 'type' | 'status' | 'currency' | 'range'>(key: K, value: MovementFilterDraft[K]) => {
      setFilter(key, value);
      closeFilter();
    },
    custom: {
      open: customOpen,
      show: () => setCustomOpen(true),
      fromText,
      setFromText,
      toText,
      setToText,
      error: customSubmitted ? rangeError : null,
      apply: () => {
        setCustomSubmitted(true);
        const from = parseDayInput(fromText);
        const to = parseDayInput(toText);
        if (rangeError || !from || !to) return;
        setCustomRange(from, to);
        closeFilter();
      },
    },
    movements,
    items: movements.data?.pages.flatMap((page) => page.items) ?? [],
    loadMore: () => {
      if (movements.hasNextPage && !movements.isFetchingNextPage) void movements.fetchNextPage();
    },
    openMovement: (movement: Movement) => router.push({ pathname: '/movements/[id]', params: { id: movement.id } }),
  };
}
