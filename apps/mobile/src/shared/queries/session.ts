import { useQuery } from '@tanstack/react-query';

import { api } from '@/shared/api';
import { useSessionStore } from '@/shared/stores/useSessionStore';

export const ME_KEY = ['me'] as const;

export function useMe() {
  const active = useSessionStore((state) => state.status === 'active');
  return useQuery({ queryKey: ME_KEY, queryFn: () => api.auth.me(), enabled: active });
}
