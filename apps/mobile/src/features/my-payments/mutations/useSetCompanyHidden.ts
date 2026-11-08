import { useMutation, useQueryClient } from '@tanstack/react-query';

import { api } from '@/shared/api';
import { notifyError, notifySuccess } from '@/shared/feedback/mutationFeedback';
import { MY_PAYMENTS_KEY } from '@/shared/queries/myPayments';

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
