import { useMutation, useQueryClient } from '@tanstack/react-query';

import { api } from '@/shared/api';
import { notifyError } from '@/shared/feedback/mutationFeedback';
import { queryPersister } from '@/shared/queries/queryClient';
import { clearSession } from '@/shared/session/session';
import { useSessionStore } from '@/shared/stores/useSessionStore';

/** Logout (M4): server session, every secure key, the in-memory token and cached data. */
export function useLogout() {
  const queryClient = useQueryClient();
  const setStatus = useSessionStore((state) => state.setStatus);
  return useMutation({
    mutationFn: async () => {
      // The local session is wiped even when the server is unreachable.
      await api.auth.logout().catch(() => undefined);
      await clearSession();
      await queryPersister.removeClient();
    },
    onSuccess: () => {
      queryClient.clear();
      setStatus('signedOut');
    },
    onError: notifyError,
  });
}
