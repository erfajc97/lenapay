import { useQuery } from '@tanstack/react-query';

import { api } from '@/shared/api';

export const RECON_RUNS_KEY = (orgId: string) => ['reconciliation', orgId] as const;

export function useReconciliationRuns(orgId: string | undefined) {
  return useQuery({ queryKey: RECON_RUNS_KEY(orgId ?? ''), queryFn: () => api.reconciliation.runs(orgId ?? ''), enabled: Boolean(orgId) });
}
