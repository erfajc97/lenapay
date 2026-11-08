import { useMutation, useQueryClient } from '@tanstack/react-query';

import { api } from '@/shared/api';
import type { UpdatePreferencesRequest } from '@/shared/api/types';
import { notifyError } from '@/shared/feedback/mutationFeedback';
import { setMePreferences } from '@/shared/queries/session';

/**
 * A language or theme picked on this phone goes to the server so other devices follow (R-ID-5).
 * The change already applied locally, so success needs no toast; a failure says it did not sync.
 */
export function useSyncAppearance() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: Pick<UpdatePreferencesRequest, 'locale' | 'theme'>) => api.profile.updatePreferences(body),
    onSuccess: (preferences) => setMePreferences(queryClient, preferences),
    onError: notifyError,
  });
}
