import { useMutation, useQueryClient } from '@tanstack/react-query';

import { api } from '@/shared/api';
import { ApiError } from '@/shared/api/errors';
import { authenticateWithBiometrics, biometricsAvailable } from '@/shared/biometrics/biometrics';
import { notifyError } from '@/shared/feedback/mutationFeedback';
import { i18n } from '@/shared/i18n';
import { clearSession, readBiometricsEnabled, readRefreshToken, saveTokens } from '@/shared/session/session';
import { registerDeviceInBackground } from '@/shared/stepup/registerThisDevice';
import { useSessionStore } from '@/shared/stores/useSessionStore';

/** Unlock: biometrics (when enabled and available), then rotate the stored session. */
export function useUnlock() {
  const setStatus = useSessionStore((state) => state.setStatus);
  return useMutation({
    mutationFn: async () => {
      if ((await readBiometricsEnabled()) && (await biometricsAvailable())) {
        const ok = await authenticateWithBiometrics({ prompt: i18n.t('unlock.prompt'), cancelLabel: i18n.t('common.cancel') });
        if (!ok) throw new ApiError('stepup.failed', 401);
      }
      const refreshToken = await readRefreshToken();
      if (!refreshToken) throw new ApiError('auth.session_expired', 401);
      await saveTokens(await api.auth.refresh(refreshToken));
    },
    onSuccess: () => {
      setStatus('active');
      registerDeviceInBackground();
    },
    onError: async (error) => {
      if (error instanceof ApiError && (error.code === 'auth.session_expired' || error.code === 'auth.refresh_invalid')) {
        await clearSession();
        setStatus('signedOut');
      }
      notifyError(error);
    },
  });
}

/** "Usar otra cuenta" from the lock screen: forget the stored session. */
export function useForgetSession() {
  const queryClient = useQueryClient();
  const setStatus = useSessionStore((state) => state.setStatus);
  return useMutation({
    mutationFn: clearSession,
    onSuccess: () => {
      queryClient.clear();
      setStatus('signedOut');
    },
    onError: notifyError,
  });
}
