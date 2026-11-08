import { useMutation } from '@tanstack/react-query';

import { api } from '@/shared/api';
import { warningHaptic } from '@/shared/feedback/haptics';
import { readDeviceKey } from '@/shared/session/session';
import { useStepUpStore } from '@/shared/stores/useStepUpStore';

import { type StepUpInput, verifyStepUp } from '../stepUpFlow';

/**
 * Challenge + verify (R-ID-3). The sheet closes only on success (M2); the error is shown
 * inside the sheet itself, because a toast would render behind the modal.
 */
export function useVerifyStepUp() {
  const complete = useStepUpStore((state) => state.complete);
  return useMutation({
    mutationFn: (input: StepUpInput) => verifyStepUp(input, { auth: api.auth, readDeviceKey }),
    onSuccess: (token) => complete(token.stepUpToken),
    onError: () => warningHaptic(),
  });
}
