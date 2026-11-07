import { useQuery } from '@tanstack/react-query';

import { api } from '@/shared/api';

export const ERP_CANDIDATE_KEY = (orgId: string) => ['erp', orgId, 'candidate'] as const;
export const ERP_LINK_KEY = (orgId: string) => ['erp', orgId, 'link'] as const;

export function useErpCandidate(orgId: string | undefined) {
  return useQuery({ queryKey: ERP_CANDIDATE_KEY(orgId ?? ''), queryFn: () => api.erp.candidate(orgId ?? ''), enabled: Boolean(orgId) });
}

export function useErpLink(orgId: string | undefined) {
  return useQuery({
    queryKey: ERP_LINK_KEY(orgId ?? ''),
    queryFn: () => api.erp.get(orgId ?? ''),
    enabled: Boolean(orgId),
    // Polls only while the import runs; stops once linked or not linked (M3).
    refetchInterval: (query) => (query.state.data?.status === 'importing' ? 1_000 : false),
  });
}
