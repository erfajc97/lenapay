import type { InfiniteData } from '@tanstack/react-query';

import type { Movement, Page } from '@/shared/api/types';

/**
 * The movement as a list already loaded it: the detail paints with it while
 * GET /movements/{id} loads (and when the list is all the cache has, e.g. offline).
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
