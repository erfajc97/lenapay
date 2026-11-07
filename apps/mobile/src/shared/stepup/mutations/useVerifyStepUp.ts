import { useMutation } from '@tanstack/react-query';

import { api } from '@/shared/api';
import { warningHaptic } from '@/shared/feedback/haptics';
import { useStepUpStore } from '@/shared/stores/useStepUpStore';

interface StepUpInput {
  method: 'device_key' | 'password';
  password?: string;
}

/**
 * Challenge + verify (R-ID-3). The sheet closes only on success (M2); the error is shown
 * inside the sheet itself, because a toast would render behind the modal.
 */
export function useVerifyStepUp() {
  const complete = useStepUpStore((state) => state.complete);
  return useMutation({
    mutationFn: async (input: StepUpInput) => {
      const challenge = await api.auth.stepUpChallenge();
      return api.auth.stepUpVerify({ challengeId: challenge.challengeId, method: input.method, password: input.password });
    },
    onSuccess: (token) => complete(token.stepUpToken),
    onError: () => warningHaptic(),
  });
}
