import { useMutation } from '@tanstack/react-query';
import { router } from 'expo-router';

import { api } from '@/shared/api';
import type { ResendCodeRequest, VerifyEmailRequest } from '@/shared/api/types';
import { notifyError, notifySuccess } from '@/shared/feedback/mutationFeedback';

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
