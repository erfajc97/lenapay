import { useMutation, useQueryClient } from '@tanstack/react-query';

import { api } from '@/shared/api';
import type { LoginRequest } from '@/shared/api/types';
import { notifyError } from '@/shared/feedback/mutationFeedback';
import { localFromServer } from '@/shared/helpers/preferences';
import { saveTokens } from '@/shared/session/session';
import { registerDeviceInBackground } from '@/shared/stepup/registerThisDevice';
import { usePreferencesStore } from '@/shared/stores/usePreferencesStore';
import { useSessionStore } from '@/shared/stores/useSessionStore';

export function useLogin() {
  const queryClient = useQueryClient();
  const setStatus = useSessionStore((state) => state.setStatus);
  const setPendingEmail = useSessionStore((state) => state.setPendingEmail);
  return useMutation({
    mutationFn: async (body: LoginRequest) => {
      await saveTokens(await api.auth.login(body));
      // The synced language and theme win on sign-in (R-ID-5); unreachable, this phone's stay.
      return api.profile.preferences().catch(() => null);
    },
    onSuccess: (preferences) => {
      const synced = localFromServer(preferences);
      const store = usePreferencesStore.getState();
      if (synced.language) store.setLanguagePreference(synced.language);
      if (synced.theme) store.setThemePreference(synced.theme);
      // A new sign-in opens in the person's default org, not the previous session's pick.
      store.resetSessionOrg();
      queryClient.clear();
      setPendingEmail(null);
      setStatus('active');
      registerDeviceInBackground();
    },
    onError: notifyError,
  });
}
