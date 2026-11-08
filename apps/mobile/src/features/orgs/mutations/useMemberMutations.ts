import { useMutation, useQueryClient } from '@tanstack/react-query';

import { api } from '@/shared/api';
import type { Member, Role } from '@/shared/api/types';
import { notifyError, notifySuccess } from '@/shared/feedback/mutationFeedback';
import { displayName } from '@/shared/helpers/people';
import { APPROVAL_POLICY_KEY, MEMBERS_KEY } from '@/shared/queries/orgs';

/** Change a member's role (admins and the owner; never above the caller, never the owner: R-ORG-2, R-ORG-3). */
export function useUpdateMember(orgId: string | undefined, onDone: () => void) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: { member: Member; role: Role }) => api.orgs.updateMember(orgId ?? '', input.member.userId, { role: input.role }),
    onSuccess: (member) => {
      // Who else can approve decides whether the owner's self-approval applies (R-OUT-7).
      for (const queryKey of [MEMBERS_KEY(orgId ?? ''), APPROVAL_POLICY_KEY(orgId ?? '')]) void queryClient.invalidateQueries({ queryKey });
      notifySuccess('team.roleChanged', { name: displayName(member) });
      onDone();
    },
    onError: notifyError,
  });
}

/** Remove a member (admins and the owner; only the owner removes an admin; nobody the owner: `org.last_admin`). */
export function useRemoveMember(orgId: string | undefined, onDone: () => void) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (member: Member) => api.orgs.removeMember(orgId ?? '', member.userId),
    onSuccess: (_result, member) => {
      for (const queryKey of [MEMBERS_KEY(orgId ?? ''), APPROVAL_POLICY_KEY(orgId ?? '')]) void queryClient.invalidateQueries({ queryKey });
      notifySuccess('team.removed', { name: displayName(member) });
      onDone();
    },
    onError: notifyError,
  });
}
