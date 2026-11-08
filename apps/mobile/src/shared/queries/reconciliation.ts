import { useQuery } from '@tanstack/react-query';

import { api } from '@/shared/api';

export const RECON_RUNS_KEY = (orgId: string) => ['reconciliation', orgId] as const;

export function useReconciliationRuns(orgId: string | undefined) {
  return useQuery({ queryKey: RECON_RUNS_KEY(orgId ?? ''), queryFn: () => api.reconciliation.runs(orgId ?? ''), enabled: Boolean(orgId) });
}

export const RECON_ITEMS_KEY = (orgId: string, runId: string) => ['reconciliation', orgId, 'items', runId] as const;

export function useReconciliationItems(orgId: string | undefined, runId: string | undefined) {
  return useQuery({
    queryKey: RECON_ITEMS_KEY(orgId ?? '', runId ?? ''),
    queryFn: () => api.reconciliation.items(orgId ?? '', runId ?? ''),
    enabled: Boolean(orgId && runId),
  });
}
