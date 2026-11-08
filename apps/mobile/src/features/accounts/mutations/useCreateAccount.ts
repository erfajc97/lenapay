import { useMutation, useQueryClient } from '@tanstack/react-query';

import { api } from '@/shared/api';
import { notifyError, notifySuccess } from '@/shared/feedback/mutationFeedback';
import { ACCOUNTS_KEY } from '@/shared/queries/accounts';

export function useCreateAccount(orgId: string | undefined, onDone: () => void) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => api.accounts.create(orgId ?? '', { name }),
    onSuccess: (account) => {
      void queryClient.invalidateQueries({ queryKey: ACCOUNTS_KEY(orgId ?? '') });
      notifySuccess('accounts.created', { name: account.name });
      onDone();
    },
    onError: notifyError,
  });
}
