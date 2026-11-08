import { useMutation, useQueryClient } from '@tanstack/react-query';

import { api } from '@/shared/api';
import type { UpdateMeRequest } from '@/shared/api/types';
import { notifyError, notifySuccess } from '@/shared/feedback/mutationFeedback';
import { ME_KEY } from '@/shared/queries/session';

/** Names show in every team list and on payouts: those lists refetch after a change. */
const NAME_KEYS = [['members'], ['approvals'], ['payout']] as const;

/** PATCH /me (R-ID-5): the full name; null clears it. */
export function useUpdateProfile(onDone: () => void) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: UpdateMeRequest) => api.profile.update(body),
    onSuccess: (me) => {
      queryClient.setQueryData(ME_KEY, me);
      for (const queryKey of NAME_KEYS) void queryClient.invalidateQueries({ queryKey });
      notifySuccess('profile.nameSaved');
      onDone();
    },
    onError: notifyError,
  });
}
