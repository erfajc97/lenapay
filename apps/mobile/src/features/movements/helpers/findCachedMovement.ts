import type { InfiniteData } from '@tanstack/react-query';

import type { Movement, Page } from '@/shared/api/types';

/**
 * The movement comes from the lists already loaded (the contract has no single-movement
 * endpoint); its payout or payin is then read from its own endpoint for the full detail.
 */
export function findCachedMovement(pages: (InfiniteData<Page<Movement>> | undefined)[], id: string): Movement | null {
  for (const data of pages) {
    for (const page of data?.pages ?? []) {
      const found = page.items.find((item) => item.id === id);
      if (found) return found;
    }
  }
  return null;
}
