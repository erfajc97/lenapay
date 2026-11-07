import { useMutation, useQueryClient } from '@tanstack/react-query';
import { router } from 'expo-router';

import { api } from '@/shared/api';
import { confirmHaptic } from '@/shared/feedback/haptics';
import { notifyError, notifySuccess } from '@/shared/feedback/mutationFeedback';
import { ACCOUNTS_KEY, BALANCES_KEY } from '@/shared/queries/accounts';
import { MOVEMENTS_KEY } from '@/shared/queries/movements';
import { APPROVALS_KEY, PAYOUT_KEY } from '@/shared/queries/payouts';
import { runMoneyAction } from '@/shared/stepup/moneyAction';

type Decision = { kind: 'approve' } | { kind: 'reject'; reason: string };

/** Approve or reject with step-up (R-OUT-4); both release or move held money. */
export function useDecidePayout(orgId: string | undefined, payoutId: string | undefined, onDone: () => void) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (decision: Decision) =>
      runMoneyAction({
        scope: `decision:${payoutId ?? ''}`,
        payload: decision,
        call: (options) =>
          decision.kind === 'approve'
            ? api.payouts.approve(orgId ?? '', payoutId ?? '', options)
            : api.payouts.reject(orgId ?? '', payoutId ?? '', { reason: decision.reason }, options),
      }),
    onSuccess: (payout) => {
      for (const key of [APPROVALS_KEY, BALANCES_KEY, ACCOUNTS_KEY, MOVEMENTS_KEY]) {
        void queryClient.invalidateQueries({ queryKey: key(orgId ?? '') });
      }
      queryClient.setQueryData(PAYOUT_KEY(orgId ?? '', payout.id), payout);
      confirmHaptic();
      notifySuccess(payout.status === 'rejected' ? 'approvals.rejected' : 'approvals.approved', { name: payout.destination.holderName });
      onDone();
      router.back();
    },
    onError: notifyError,
  });
}
