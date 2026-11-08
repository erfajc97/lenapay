import { useState } from 'react';

/**
 * A pull the person made, as a small state machine: spinner on, refetch, spinner off when it
 * settles (also on error); a second pull while one runs is ignored.
 */
export function createPull(setRefreshing: (refreshing: boolean) => void) {
  let active = false;
  return async (refetch: () => Promise<unknown>): Promise<void> => {
    if (active) return;
    active = true;
    setRefreshing(true);
    try {
      await refetch();
    } catch {
      // The query already holds the error and the screen shows it; the spinner just stops.
    } finally {
      active = false;
      setRefreshing(false);
    }
  };
}

/**
 * Pull-to-refresh state for a RefreshControl. `refreshing` reflects ONLY the person's pull,
 * never a query flag: on iOS a control that starts refreshing by itself (mount, focus,
 * background refetch) insets the list and keeps the gap until the person scrolls.
 */
export function usePullToRefresh(refetch: () => Promise<unknown>) {
  const [refreshing, setRefreshing] = useState(false);
  const [pull] = useState(() => createPull(setRefreshing));
  // `pull` never rejects (it catches), so nothing floats here.
  return { refreshing, onRefresh: () => void pull(refetch) };
}
