import { useMutation, useQueryClient } from '@tanstack/react-query';
import { router } from 'expo-router';

import { api } from '@/shared/api';
import { ApiError } from '@/shared/api/errors';
import type { LoginRequest, SignupRequest, VerifyEmailRequest } from '@/shared/api/types';
import { authenticateWithBiometrics, biometricsAvailable } from '@/shared/biometrics/biometrics';
import { notifyError } from '@/shared/feedback/mutationFeedback';
import { i18n } from '@/shared/i18n';
import { clearSession, readBiometricsEnabled, readRefreshToken, saveTokens } from '@/shared/session/session';
import { useSessionStore } from '@/shared/stores/useSessionStore';

export function useLogin() {
  const queryClient = useQueryClient();
  const setStatus = useSessionStore((state) => state.setStatus);
  return useMutation({
    mutationFn: (body: LoginRequest) => api.auth.login(body),
    onSuccess: async (tokens) => {
      await saveTokens(tokens);
      queryClient.clear();
      setStatus('active');
    },
    onError: notifyError,
  });
}

export function useSignup() {
  const setPendingEmail = useSessionStore((state) => state.setPendingEmail);
  return useMutation({
    mutationFn: (body: SignupRequest) => api.auth.signup(body),
    onSuccess: (response) => {
      setPendingEmail(response.email);
      router.push('/verify');
    },
    onError: notifyError,
  });
}

export function useVerifyEmail() {
  const queryClient = useQueryClient();
  const setStatus = useSessionStore((state) => state.setStatus);
  const setPendingEmail = useSessionStore((state) => state.setPendingEmail);
  return useMutation({
    mutationFn: (body: VerifyEmailRequest) => api.auth.verifyEmail(body),
    onSuccess: async (tokens) => {
      await saveTokens(tokens);
      queryClient.clear();
      setPendingEmail(null);
      setStatus('active');
    },
    onError: notifyError,
  });
}

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
    onSuccess: () => setStatus('active'),
    onError: async (error) => {
      if (error instanceof ApiError && error.code === 'auth.session_expired') {
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
