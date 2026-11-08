import { router } from 'expo-router';

import type { ReconciliationRun } from '@/shared/api/types';
import { businessDate } from '@/shared/helpers/days';
import { canRunReconciliation, canSeeReconciliation } from '@/shared/helpers/permissions';
import { useActiveOrg } from '@/shared/queries/orgs';
import { useReconciliationRuns } from '@/shared/queries/reconciliation';

import { useRunReconciliation } from '../mutations/useRunReconciliation';

export function useReconciliation() {
  const { org } = useActiveOrg();
  const allowed = org ? canSeeReconciliation(org.role) : false;
  const runs = useReconciliationRuns(allowed ? org?.id : undefined);
  const runNow = useRunReconciliation(org?.id);
  return {
    allowed,
    runs,
    canRun: org ? canRunReconciliation(org.role) : false,
    runToday: () => runNow.mutate(businessDate(new Date())),
    running: runNow.isPending,
    open: (run: ReconciliationRun) => router.push({ pathname: '/reconciliation/[id]', params: { id: run.id } }),
  };
}
