import { useMutation, useQueryClient } from '@tanstack/react-query';

import { api } from '@/shared/api';
import type { InviteRequest } from '@/shared/api/types';
import { notifyError, notifySuccess } from '@/shared/feedback/mutationFeedback';
import { MEMBERS_KEY } from '@/shared/queries/orgs';

/** Invite by email with a role; invites expire in 7 days on the API (R-ORG-2). */
export function useInvite(orgId: string | undefined, onDone: () => void) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: InviteRequest) => api.orgs.invite(orgId ?? '', body),
    onSuccess: (invite) => {
      void queryClient.invalidateQueries({ queryKey: MEMBERS_KEY(orgId ?? '') });
      notifySuccess('team.invited', { email: invite.email });
      onDone();
    },
    onError: notifyError,
  });
}
