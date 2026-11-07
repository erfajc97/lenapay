import { router, useLocalSearchParams } from 'expo-router';

import type { ReconciliationRun } from '@/shared/api/types';
import { canSeeReconciliation } from '@/shared/helpers/permissions';
import { useActiveOrg } from '@/shared/queries/orgs';
import { useReconciliationRuns } from '@/shared/queries/reconciliation';

export function useReconciliation() {
  const { org } = useActiveOrg();
  const allowed = org ? canSeeReconciliation(org.role) : false;
  const runs = useReconciliationRuns(allowed ? org?.id : undefined);
  return {
    allowed,
    runs,
    open: (run: ReconciliationRun) => router.push({ pathname: '/reconciliation/[id]', params: { id: run.id } }),
  };
}

export function useReconciliationRun() {
  const params = useLocalSearchParams<{ id?: string }>();
  const { org } = useActiveOrg();
  const runs = useReconciliationRuns(org?.id);
  return { runs, run: runs.data?.find((item) => item.id === params.id) ?? null };
}
