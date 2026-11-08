import { useMutation, useQueryClient } from '@tanstack/react-query';

import { api } from '@/shared/api';
import type { UpdatePreferencesRequest } from '@/shared/api/types';
import { notifyError, notifySuccess } from '@/shared/feedback/mutationFeedback';
import { setMePreferences } from '@/shared/queries/session';

/** Cuenta predeterminada (R-ID-5): default org + account, validated by the API as a whole. */
export function useSaveDefaultAccount(onDone: () => void) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: UpdatePreferencesRequest) => api.profile.updatePreferences(body),
    onSuccess: (preferences) => {
      setMePreferences(queryClient, preferences);
      notifySuccess('defaultAccount.saved');
      onDone();
    },
    onError: notifyError,
  });
}
