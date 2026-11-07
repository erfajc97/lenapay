import { onlineManager } from '@tanstack/react-query';
import { useSyncExternalStore } from 'react';

/** Connectivity as TanStack Query sees it (wired to NetInfo at the root). */
export function useIsOnline(): boolean {
  return useSyncExternalStore(
    (onChange) => onlineManager.subscribe(onChange),
    () => onlineManager.isOnline(),
    () => true,
  );
}
