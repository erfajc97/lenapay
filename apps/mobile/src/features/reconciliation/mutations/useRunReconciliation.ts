import { useMutation, useQueryClient } from '@tanstack/react-query';

import { api } from '@/shared/api';
import { CURRENCIES } from '@/shared/api/types';
import { notifyError, notifySuccess } from '@/shared/feedback/mutationFeedback';
import { RECON_RUNS_KEY } from '@/shared/queries/reconciliation';

/** "Conciliar hoy" (admins): one on-demand run per currency for the business day (R-REC-1). */
export function useRunReconciliation(orgId: string | undefined) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (day: string) => Promise.all(CURRENCIES.map((currency) => api.reconciliation.run(orgId ?? '', { currency, day }))),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: RECON_RUNS_KEY(orgId ?? '') });
      notifySuccess('reconciliation.runQueued');
    },
    onError: notifyError,
  });
}
