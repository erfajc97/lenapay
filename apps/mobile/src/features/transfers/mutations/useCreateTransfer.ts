import { useMutation, useQueryClient } from '@tanstack/react-query';
import { router } from 'expo-router';

import { api } from '@/shared/api';
import type { InternalTransferRequest } from '@/shared/api/types';
import { confirmHaptic } from '@/shared/feedback/haptics';
import { notifyError, notifySuccess } from '@/shared/feedback/mutationFeedback';
import { formatMoney } from '@/shared/money/format';
import { ACCOUNTS_KEY, BALANCES_KEY } from '@/shared/queries/accounts';
import { MOVEMENTS_KEY } from '@/shared/queries/movements';
import { runMoneyAction } from '@/shared/stepup/moneyAction';

/** Internal transfer with step-up and Idempotency-Key (R-ACC-4, R-ID-3). */
export function useCreateTransfer(orgId: string | undefined) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: InternalTransferRequest) =>
      runMoneyAction({ scope: `transfer:${orgId ?? ''}`, payload: body, call: (options) => api.accounts.transfer(orgId ?? '', body, options) }),
    onSuccess: (transfer) => {
      for (const key of [ACCOUNTS_KEY, BALANCES_KEY, MOVEMENTS_KEY]) {
        void queryClient.invalidateQueries({ queryKey: key(orgId ?? '') });
      }
      confirmHaptic();
      notifySuccess('transfer.done', { amount: formatMoney(transfer.amount, transfer.currency) });
      router.back();
    },
    onError: notifyError,
  });
}
