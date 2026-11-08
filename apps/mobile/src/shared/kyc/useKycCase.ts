import { useState } from 'react';

import { useIsMockApi } from '@/shared/api';
import type { KycDocumentKind } from '@/shared/api/types';
import { useActiveOrg, useKyc } from '@/shared/queries/orgs';

import { canSubmitKyc, kycChecklist } from './kycChecklist';
import { useSaveKyc, useSubmitKyc, useUploadKycDocument } from './mutations/useKycMutations';

/** Shared KYC plumbing: the case, uploads, the selfie camera (simulated in mock) and submit. */
export function useKycCase() {
  const { org } = useActiveOrg();
  const kyc = useKyc(org?.id);
  const upload = useUploadKycDocument(org?.id);
  const submit = useSubmitKyc(org?.id);
  const save = useSaveKyc(org?.id);
  const isMock = useIsMockApi();
  // `null` = closed; `{ representativeDni }` = the simulated camera is open for that person.
  const [camera, setCamera] = useState<{ representativeDni?: string } | null>(null);
  return {
    org,
    kyc,
    save,
    checklist: kyc.data ? kycChecklist(kyc.data) : [],
    canSubmit: kyc.data ? canSubmitKyc(kyc.data) : false,
    submit: () => submit.mutate(),
    submitting: submit.isPending,
    upload: (kind: KycDocumentKind, representativeDni?: string) => upload.mutate({ kind, representativeDni }),
    uploading: upload.isPending,
    camera,
    // Live mode picks a real photo; the mock shows the simulated camera instead.
    takeSelfie: (representativeDni?: string) => (isMock ? setCamera({ representativeDni }) : upload.mutate({ kind: 'selfie', representativeDni })),
    closeCamera: () => setCamera(null),
    capture: () => {
      upload.mutate({ kind: 'selfie', representativeDni: camera?.representativeDni }, { onSuccess: () => setCamera(null) });
    },
  };
}
