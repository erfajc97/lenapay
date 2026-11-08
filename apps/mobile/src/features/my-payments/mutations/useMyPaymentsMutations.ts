import { useMutation, useQueryClient } from '@tanstack/react-query';

import { api } from '@/shared/api';
import type { PayCompanyRequest, Payout } from '@/shared/api/types';
import { confirmHaptic } from '@/shared/feedback/haptics';
import { notifyError, notifySuccess } from '@/shared/feedback/mutationFeedback';
import { ACCOUNTS_KEY, BALANCES_KEY } from '@/shared/queries/accounts';
import { MOVEMENTS_KEY } from '@/shared/queries/movements';
import { MY_PAYMENTS_KEY } from '@/shared/queries/myPayments';
import { runMoneyAction } from '@/shared/stepup/moneyAction';

/**
 * Pay a company from the personal wallet (R-MP-4): step-up, persisted Idempotency-Key, no
 * approval. The money leaves the personal org, so its balances and movements refetch with the
 * company's totals and history. The result view inside the sheet is the feedback.
 */
export function usePayCompany(params: { linkId: string | undefined; personalOrgId: string | null; onPaid: (payout: Payout) => void }) {
  const queryClient = useQueryClient();
  const { linkId, personalOrgId, onPaid } = params;
  return useMutation({
    mutationFn: (body: PayCompanyRequest) =>
      runMoneyAction({ scope: `mypay:${linkId ?? ''}`, payload: body, call: (options) => api.myPayments.pay(linkId ?? '', body, options) }),
    onSuccess: (payout) => {
      void queryClient.invalidateQueries({ queryKey: MY_PAYMENTS_KEY });
      if (personalOrgId) {
        for (const key of [BALANCES_KEY, ACCOUNTS_KEY, MOVEMENTS_KEY]) void queryClient.invalidateQueries({ queryKey: key(personalOrgId) });
      }
      confirmHaptic();
      onPaid(payout);
    },
    onError: notifyError,
  });
}

/** Hide a company from "Mis pagos" or show it again; the list refetches with the new state. */
export function useSetCompanyHidden() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: { linkId: string; hidden: boolean }) => api.myPayments.setHidden(input.linkId, input.hidden),
    onSuccess: (company) => {
      void queryClient.invalidateQueries({ queryKey: MY_PAYMENTS_KEY });
      notifySuccess(company.hidden ? 'myPayments.hiddenDone' : 'myPayments.shownDone', { name: company.companyName });
    },
    onError: notifyError,
  });
}
