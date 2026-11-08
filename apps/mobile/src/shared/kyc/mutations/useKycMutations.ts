import { useMutation, useQueryClient } from '@tanstack/react-query';
import { router } from 'expo-router';

import { api } from '@/shared/api';
import type { KycDocumentKind, KycUpdateRequest } from '@/shared/api/types';
import { confirmHaptic } from '@/shared/feedback/haptics';
import { notifyError, notifySuccess } from '@/shared/feedback/mutationFeedback';
import { KYC_KEY, ORGS_KEY } from '@/shared/queries/orgs';

import { pickKycFile } from '../kycFiles';

export function useSaveKyc(orgId: string | undefined) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: KycUpdateRequest) => api.kyc.update(orgId ?? '', body),
    onSuccess: (kyc) => {
      queryClient.setQueryData(KYC_KEY(orgId ?? ''), kyc);
      notifySuccess('kyc.saved');
    },
    onError: notifyError,
  });
}

/** Pick (or fake, in mock) the file and upload it; a cancelled picker is not an error. */
export function useUploadKycDocument(orgId: string | undefined) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (input: { kind: KycDocumentKind; representativeDni?: string }) => {
      const file = await pickKycFile(input.kind);
      if (!file) return null;
      return api.kyc.uploadDocument(orgId ?? '', { kind: input.kind, representativeDni: input.representativeDni, ...file });
    },
    onSuccess: (document) => {
      if (!document) return;
      void queryClient.invalidateQueries({ queryKey: KYC_KEY(orgId ?? '') });
      notifySuccess('kyc.uploaded', { name: document.fileName });
    },
    onError: notifyError,
  });
}

export function useSubmitKyc(orgId: string | undefined) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => api.kyc.submit(orgId ?? ''),
    onSuccess: (kyc) => {
      queryClient.setQueryData(KYC_KEY(orgId ?? ''), kyc);
      void queryClient.invalidateQueries({ queryKey: ORGS_KEY });
      confirmHaptic();
      router.replace('/onboarding/status');
    },
    onError: notifyError,
  });
}
