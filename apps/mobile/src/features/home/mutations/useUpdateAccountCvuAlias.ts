import { useMutation, useQueryClient } from '@tanstack/react-query';

import { api } from '@/shared/api';
import { notifyError, notifySuccess } from '@/shared/feedback/mutationFeedback';
import { ACCOUNTS_KEY } from '@/shared/queries/accounts';

/** New alias for one of the wallet's own CVUs (R-CVU-4, PATCH …/accounts/{id}/cvus/{cvuId}). */
export function useUpdateAccountCvuAlias(orgId: string | undefined, onDone: () => void) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: { accountId: string; cvuId: string; alias: string }) =>
      api.accounts.updateCvu(orgId ?? '', input.accountId, input.cvuId, { alias: input.alias.trim().toLowerCase() }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ACCOUNTS_KEY(orgId ?? '') });
      notifySuccess('payers.aliasUpdated');
      onDone();
    },
    onError: notifyError,
  });
}
