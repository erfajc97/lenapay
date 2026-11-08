import { useLocalSearchParams } from 'expo-router';

import { useActiveOrg } from '@/shared/queries/orgs';
import { useReconciliationItems, useReconciliationRuns } from '@/shared/queries/reconciliation';

export function useReconciliationRun() {
  const params = useLocalSearchParams<{ id?: string }>();
  const runId = typeof params.id === 'string' ? params.id : undefined;
  const { org } = useActiveOrg();
  const runs = useReconciliationRuns(org?.id);
  const items = useReconciliationItems(org?.id, runId);
  return { runs, run: runs.data?.find((item) => item.id === runId) ?? null, items };
}
