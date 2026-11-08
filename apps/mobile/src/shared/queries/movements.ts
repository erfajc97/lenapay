import { useInfiniteQuery, useQuery } from '@tanstack/react-query';

import { api } from '@/shared/api';
import type { Movement, MovementFilters } from '@/shared/api/types';

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

export const MOVEMENT_KEY = (orgId: string, movementId: string) => ['movement', orgId, movementId] as const;

/** One movement (GET /movements/{id}); the row already loaded in a list shows while it loads. */
export function useMovement(orgId: string | undefined, movementId: string | undefined, placeholder: Movement | null) {
  return useQuery({
    queryKey: MOVEMENT_KEY(orgId ?? '', movementId ?? ''),
    queryFn: () => api.movements.get(orgId ?? '', movementId ?? ''),
    enabled: Boolean(orgId && movementId),
    placeholderData: placeholder ?? undefined,
  });
}
