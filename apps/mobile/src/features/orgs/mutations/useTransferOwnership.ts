import { useMutation, useQueryClient } from '@tanstack/react-query';

import { api } from '@/shared/api';
import type { Member } from '@/shared/api/types';
import { notifyError, notifySuccess } from '@/shared/feedback/mutationFeedback';
import { displayName } from '@/shared/helpers/people';
import { APPROVAL_POLICY_KEY, MEMBERS_KEY, ORGS_KEY } from '@/shared/queries/orgs';
import { ME_KEY } from '@/shared/queries/session';
import { runStepUpAction } from '@/shared/stepup/stepUpAction';

/**
 * Hand the organization to another member (owner + step-up, R-ORG-3). Both roles change, so the
 * members, the org list (my role), my memberships and the policy's self-approval all refetch.
 */
export function useTransferOwnership(orgId: string | undefined, onDone: () => void) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (member: Member) => runStepUpAction({ call: (options) => api.orgs.transferOwnership(orgId ?? '', { userId: member.userId }, options) }),
    onSuccess: (transfer) => {
      for (const queryKey of [MEMBERS_KEY(orgId ?? ''), ORGS_KEY, ME_KEY, APPROVAL_POLICY_KEY(orgId ?? '')]) {
        void queryClient.invalidateQueries({ queryKey });
      }
      notifySuccess('team.transferred', { name: displayName(transfer.owner) });
      onDone();
    },
    onError: notifyError,
  });
}
