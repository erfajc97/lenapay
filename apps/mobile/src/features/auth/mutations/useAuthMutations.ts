import { useMutation, useQueryClient } from '@tanstack/react-query';
import { router } from 'expo-router';

import { api } from '@/shared/api';
import { ApiError } from '@/shared/api/errors';
import type { LoginRequest, ResendCodeRequest, SignupRequest, VerifyEmailRequest } from '@/shared/api/types';
import { authenticateWithBiometrics, biometricsAvailable } from '@/shared/biometrics/biometrics';
import { notifyError, notifySuccess } from '@/shared/feedback/mutationFeedback';
import { i18n } from '@/shared/i18n';
import { captureException } from '@/shared/reporting/reporting';
import { clearSession, readBiometricsEnabled, readRefreshToken, saveTokens } from '@/shared/session/session';
import { registerThisDevice } from '@/shared/stepup/registerThisDevice';
import { useSessionStore } from '@/shared/stores/useSessionStore';

/** The device key is an upgrade of step-up: registering it never delays or fails the sign-in. */
function registerDeviceInBackground(): void {
  registerThisDevice().catch((error: unknown) => captureException(error, { area: 'device_registration' }));
}

export function useLogin() {
  const queryClient = useQueryClient();
  const setStatus = useSessionStore((state) => state.setStatus);
  const setPendingEmail = useSessionStore((state) => state.setPendingEmail);
  return useMutation({
    mutationFn: (body: LoginRequest) => api.auth.login(body),
    onSuccess: async (tokens) => {
      await saveTokens(tokens);
      queryClient.clear();
      setPendingEmail(null);
      setStatus('active');
      registerDeviceInBackground();
    },
    onError: notifyError,
  });
}

export function useSignup() {
  const setPendingEmail = useSessionStore((state) => state.setPendingEmail);
  return useMutation({
    mutationFn: (body: SignupRequest) => api.auth.signup(body),
    onSuccess: (_response, body) => {
      setPendingEmail(body.email.trim().toLowerCase());
      router.push('/verify');
    },
    onError: notifyError,
  });
}

/** The contract verifies the email without a session (204): the person then signs in. */
export function useVerifyEmail() {
  return useMutation({
    mutationFn: (body: VerifyEmailRequest) => api.auth.verifyEmail(body),
    onSuccess: () => {
      notifySuccess('verify.verified');
      router.replace('/login');
    },
    onError: notifyError,
  });
}

/** "Reenviar código": the API answers the same whether or not the email exists (no enumeration). */
export function useResendCode() {
  return useMutation({
    mutationFn: (body: ResendCodeRequest) => api.auth.resendCode(body),
    onSuccess: () => notifySuccess('verify.resent'),
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
