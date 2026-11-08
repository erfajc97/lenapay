import { useMutation, useQueryClient } from '@tanstack/react-query';

import { api } from '@/shared/api';
import type { Me, UpdateMeRequest, UpdatePreferencesRequest, UserPreferences } from '@/shared/api/types';
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

function withPreferences(queryClient: ReturnType<typeof useQueryClient>, preferences: UserPreferences): void {
  queryClient.setQueryData<Me>(ME_KEY, (me) => (me ? { ...me, preferences } : me));
}

/** Cuenta predeterminada (R-ID-5): default org + account, validated by the API as a whole. */
export function useSaveDefaultAccount(onDone: () => void) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: UpdatePreferencesRequest) => api.profile.updatePreferences(body),
    onSuccess: (preferences) => {
      withPreferences(queryClient, preferences);
      notifySuccess('defaultAccount.saved');
      onDone();
    },
    onError: notifyError,
  });
}

/**
 * A language or theme picked on this phone goes to the server so other devices follow (R-ID-5).
 * The change already applied locally, so success needs no toast; a failure says it did not sync.
 */
export function useSyncAppearance() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: Pick<UpdatePreferencesRequest, 'locale' | 'theme'>) => api.profile.updatePreferences(body),
    onSuccess: (preferences) => withPreferences(queryClient, preferences),
    onError: notifyError,
  });
}
