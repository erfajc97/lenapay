import { useMutation, useQueryClient } from '@tanstack/react-query';
import { router } from 'expo-router';

import { checkApiHealth } from '@/shared/api/live/health';
import type { ApiTarget } from '@/shared/config/env';
import { notifyError, notifySuccess } from '@/shared/feedback/mutationFeedback';
import { queryPersister } from '@/shared/queries/queryClient';
import { clearSession } from '@/shared/session/session';
import { useApiTargetStore } from '@/shared/stores/useApiTargetStore';
import { useSessionStore } from '@/shared/stores/useSessionStore';

/**
 * Switch the API (or go back to the build's default with `null`). Tokens, the device key and
 * cached data belong to the previous API, so all of it is dropped and the person signs in again.
 */
export function useApplyApiTarget() {
  const queryClient = useQueryClient();
  const setStatus = useSessionStore((state) => state.setStatus);
  return useMutation({
    mutationFn: async (target: ApiTarget | null) => {
      await clearSession();
      await queryPersister.removeClient();
      const store = useApiTargetStore.getState();
      if (target) store.setOverride(target);
      else store.clearOverride();
    },
    onSuccess: () => {
      queryClient.clear();
      setStatus('signedOut');
      notifySuccess('developer.applied');
      // Signed out, `/` is the welcome screen of the new API.
      router.replace('/');
    },
    onError: notifyError,
  });
}

export function useCheckApi() {
  return useMutation({
    mutationFn: (url: string) => checkApiHealth(url.trim().replace(/\/+$/, '')),
    onSuccess: () => notifySuccess('developer.reachable'),
    onError: notifyError,
  });
}
