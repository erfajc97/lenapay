import { useMutation, useQueryClient } from '@tanstack/react-query';

import { api } from '@/shared/api';
import type { UpdateApprovalPolicyRequest } from '@/shared/api/types';
import { notifyError, notifySuccess } from '@/shared/feedback/mutationFeedback';
import { APPROVAL_POLICY_KEY } from '@/shared/queries/orgs';
import { runStepUpAction } from '@/shared/stepup/stepUpAction';

/** Replace the approval policy (owner + step-up, R-OUT-7); the answer is the policy in force. */
export function useUpdateApprovalPolicy(orgId: string | undefined, onDone: () => void) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: UpdateApprovalPolicyRequest) => runStepUpAction({ call: (options) => api.orgs.updateApprovalPolicy(orgId ?? '', body, options) }),
    onSuccess: (policy) => {
      queryClient.setQueryData(APPROVAL_POLICY_KEY(orgId ?? ''), policy);
      void queryClient.invalidateQueries({ queryKey: APPROVAL_POLICY_KEY(orgId ?? '') });
      notifySuccess('policy.saved');
      onDone();
    },
    onError: notifyError,
  });
}
