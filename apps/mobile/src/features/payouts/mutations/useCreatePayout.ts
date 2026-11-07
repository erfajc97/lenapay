import { useMutation, useQueryClient } from '@tanstack/react-query';

import { api } from '@/shared/api';
import type { CreatePayoutRequest, Payout } from '@/shared/api/types';
import { confirmHaptic } from '@/shared/feedback/haptics';
import { notifyError } from '@/shared/feedback/mutationFeedback';
import { ACCOUNTS_KEY, BALANCES_KEY } from '@/shared/queries/accounts';
import { MOVEMENTS_KEY } from '@/shared/queries/movements';
import { APPROVALS_KEY } from '@/shared/queries/payouts';
import { runMoneyAction } from '@/shared/stepup/moneyAction';

/**
 * Create the payout (R-OUT-1..4): step-up, persisted Idempotency-Key, then the hold shows up
 * in balances and the payout in approvals and movements. The result step is the feedback.
 */
export function useCreatePayout(orgId: string | undefined, onCreated: (payout: Payout) => void) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: CreatePayoutRequest) =>
      runMoneyAction({ scope: `payout:${orgId ?? ''}`, payload: body, call: (options) => api.payouts.create(orgId ?? '', body, options) }),
    onSuccess: (payout) => {
      for (const key of [BALANCES_KEY, ACCOUNTS_KEY, APPROVALS_KEY, MOVEMENTS_KEY]) {
        void queryClient.invalidateQueries({ queryKey: key(orgId ?? '') });
      }
      confirmHaptic();
      onCreated(payout);
    },
    onError: notifyError,
  });
}
