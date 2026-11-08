import { type QueryClient, useQuery } from '@tanstack/react-query';

import { api } from '@/shared/api';
import type { Me, UserPreferences } from '@/shared/api/types';
import { useSessionStore } from '@/shared/stores/useSessionStore';

export const ME_KEY = ['me'] as const;

export function useMe() {
  const active = useSessionStore((state) => state.status === 'active');
  return useQuery({ queryKey: ME_KEY, queryFn: () => api.auth.me(), enabled: active });
}

/** A preferences write answers with the stored preferences: the cached `/me` takes them as is. */
export function setMePreferences(queryClient: QueryClient, preferences: UserPreferences): void {
  queryClient.setQueryData<Me>(ME_KEY, (me) => (me ? { ...me, preferences } : me));
}
