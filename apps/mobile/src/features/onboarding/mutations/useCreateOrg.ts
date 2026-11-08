import { useMutation, useQueryClient } from '@tanstack/react-query';
import { router } from 'expo-router';

import { api } from '@/shared/api';
import type { CreateOrgRequest } from '@/shared/api/types';
import { notifyError } from '@/shared/feedback/mutationFeedback';
import { ORGS_KEY } from '@/shared/queries/orgs';
import { usePreferencesStore } from '@/shared/stores/usePreferencesStore';

export function useCreateOrg() {
  const queryClient = useQueryClient();
  const setActiveOrgId = usePreferencesStore((state) => state.setActiveOrgId);
  return useMutation({
    mutationFn: (body: CreateOrgRequest) => api.orgs.create(body),
    onSuccess: (org) => {
      setActiveOrgId(org.id);
      void queryClient.invalidateQueries({ queryKey: ORGS_KEY });
      router.replace(org.kind === 'company' ? '/onboarding/company' : '/onboarding/person');
    },
    onError: notifyError,
  });
}
