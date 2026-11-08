import { useMutation, useQueryClient } from '@tanstack/react-query';

import { api } from '@/shared/api';
import type { UpdateAccountRequest } from '@/shared/api/types';
import { notifyError, notifySuccess } from '@/shared/feedback/mutationFeedback';
import { ACCOUNTS_KEY, BALANCES_KEY } from '@/shared/queries/accounts';

export function useUpdateAccount(orgId: string | undefined, accountId: string | undefined, onDone: () => void) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: UpdateAccountRequest) => api.accounts.update(orgId ?? '', accountId ?? '', body),
    onSuccess: (account, body) => {
      void queryClient.invalidateQueries({ queryKey: ACCOUNTS_KEY(orgId ?? '') });
      void queryClient.invalidateQueries({ queryKey: BALANCES_KEY(orgId ?? '') });
      notifySuccess(body.archived ? 'accounts.archived' : 'accounts.renamed', { name: account.name });
      onDone();
    },
    onError: notifyError,
  });
}
