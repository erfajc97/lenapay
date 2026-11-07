import { useInfiniteQuery } from '@tanstack/react-query';

import { api } from '@/shared/api';
import type { MovementFilters } from '@/shared/api/types';

export const MOVEMENTS_KEY = (orgId: string) => ['movements', orgId] as const;

export function useMovements(orgId: string | undefined, filters: MovementFilters) {
  return useInfiniteQuery({
    queryKey: [...MOVEMENTS_KEY(orgId ?? ''), filters],
    queryFn: ({ pageParam }) => api.movements.list(orgId ?? '', filters, pageParam),
    initialPageParam: null as string | null,
    getNextPageParam: (lastPage) => lastPage.nextCursor,
    enabled: Boolean(orgId),
  });
}
