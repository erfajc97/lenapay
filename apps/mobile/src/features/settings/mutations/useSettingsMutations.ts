import { useMutation, useQueryClient } from '@tanstack/react-query';

import { api } from '@/shared/api';
import { notifyError, notifySuccess } from '@/shared/feedback/mutationFeedback';
import { enablePush, openNotificationSettings } from '@/shared/notifications/push';
import { queryPersister } from '@/shared/queries/queryClient';
import { clearSession, setBiometricsEnabled } from '@/shared/session/session';
import { useSessionStore } from '@/shared/stores/useSessionStore';

import { BIOMETRICS_SETTING_KEY } from '../types';

/** Logout (M4): server session, every secure key, the in-memory token and cached data. */
export function useLogout() {
  const queryClient = useQueryClient();
  const setStatus = useSessionStore((state) => state.setStatus);
  return useMutation({
    mutationFn: async () => {
      // The local session is wiped even when the server is unreachable.
      await api.auth.logout().catch(() => undefined);
      await clearSession();
      await queryPersister.removeClient();
    },
    onSuccess: () => {
      queryClient.clear();
      setStatus('signedOut');
    },
    onError: notifyError,
  });
}

export function useToggleBiometrics() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: setBiometricsEnabled,
    onSuccess: (_, enabled) => {
      queryClient.setQueryData(BIOMETRICS_SETTING_KEY, enabled);
      notifySuccess(enabled ? 'security.biometricsOn' : 'security.biometricsOff');
    },
    onError: notifyError,
  });
}

/** Push permission is asked here, after the in-app explanation (ME13). */
export function useEnablePush() {
  return useMutation({
    mutationFn: enablePush,
    onSuccess: (result) => {
      if (result === 'denied') openNotificationSettings();
      notifySuccess(`security.push.${result}`);
    },
    onError: notifyError,
  });
}
