import { useMutation } from '@tanstack/react-query';
import { router } from 'expo-router';

import { api } from '@/shared/api';
import type { SignupRequest } from '@/shared/api/types';
import { notifyError } from '@/shared/feedback/mutationFeedback';
import { useSessionStore } from '@/shared/stores/useSessionStore';

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
