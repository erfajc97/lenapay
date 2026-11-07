import { keepPreviousData, useQuery } from '@tanstack/react-query';

import { api } from '@/shared/api';
import type { PayerQuery } from '@/shared/api/client';

export const PAYERS_KEY = (orgId: string) => ['payers', orgId] as const;
export const PAYER_KEY = (orgId: string, payerId: string) => ['payers', orgId, 'one', payerId] as const;

export function usePayers(orgId: string | undefined, query: PayerQuery) {
  return useQuery({
    queryKey: [...PAYERS_KEY(orgId ?? ''), 'list', query],
    queryFn: () => api.payers.list(orgId ?? '', query),
    enabled: Boolean(orgId),
    placeholderData: keepPreviousData,
  });
}

export function usePayer(orgId: string | undefined, payerId: string | undefined) {
  return useQuery({
    queryKey: PAYER_KEY(orgId ?? '', payerId ?? ''),
    queryFn: () => api.payers.get(orgId ?? '', payerId ?? ''),
    enabled: Boolean(orgId && payerId),
  });
}
