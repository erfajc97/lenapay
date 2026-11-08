import { useMutation, useQueryClient } from '@tanstack/react-query';

import { api } from '@/shared/api';
import type { Member, Role } from '@/shared/api/types';
import { notifyError, notifySuccess } from '@/shared/feedback/mutationFeedback';
import { MEMBERS_KEY } from '@/shared/queries/orgs';

/** Change a member's role (admins; never above the caller, never the last admin: R-ORG-2). */
export function useUpdateMember(orgId: string | undefined, onDone: () => void) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: { member: Member; role: Role }) => api.orgs.updateMember(orgId ?? '', input.member.userId, { role: input.role }),
    onSuccess: (member) => {
      void queryClient.invalidateQueries({ queryKey: MEMBERS_KEY(orgId ?? '') });
      notifySuccess('team.roleChanged', { email: member.email });
      onDone();
    },
    onError: notifyError,
  });
}

/** Remove a member from the organization (admins; the last admin stays: `org.last_admin`). */
export function useRemoveMember(orgId: string | undefined, onDone: () => void) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (member: Member) => api.orgs.removeMember(orgId ?? '', member.userId),
    onSuccess: (_result, member) => {
      void queryClient.invalidateQueries({ queryKey: MEMBERS_KEY(orgId ?? '') });
      notifySuccess('team.removed', { email: member.email });
      onDone();
    },
    onError: notifyError,
  });
}
