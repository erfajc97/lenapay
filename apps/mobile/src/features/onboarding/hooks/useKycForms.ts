import { zodResolver } from '@hookform/resolvers/zod';
import { useState } from 'react';
import { useForm } from 'react-hook-form';

import { useIsMockApi } from '@/shared/api';
import type { KycDocumentKind } from '@/shared/api/types';
import { notifyInfo } from '@/shared/feedback/mutationFeedback';
import { digitsOnly } from '@/shared/helpers/cuit';
import { useActiveOrg, useKyc } from '@/shared/queries/orgs';

import { canSubmitKyc, companyFormValues, kycChecklist, personFormValues, toAddress, toPersonData } from '../helpers/kycChecklist';
import { useSaveKyc, useSubmitKyc, useUploadKycDocument } from '../mutations/useKycMutations';
import { type CompanyValues, companySchema, type PersonValues, personSchema, type RepresentativeValues, representativeSchema } from '../types';

/** Shared KYC plumbing: the case, uploads, the selfie camera (simulated in mock) and submit. */
function useKycCase() {
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

export function useCompanyKyc() {
  const base = useKycCase();
  const data = base.kyc.data;
  const form = useForm<CompanyValues>({
    resolver: zodResolver(companySchema),
    values: companyFormValues(data, base.org?.legalName ?? ''),
    resetOptions: { keepDirtyValues: true },
  });
  const repForm = useForm<RepresentativeValues>({ resolver: zodResolver(representativeSchema), defaultValues: { fullName: '', dni: '', position: '' } });
  const [repSheet, setRepSheet] = useState(false);
  const representatives = data?.company?.representatives ?? [];

  return {
    ...base,
    form,
    representatives,
    saveCompany: () =>
      void form.handleSubmit((values) =>
        base.save.mutate({ company: { cuit: digitsOnly(values.cuit), legalName: values.legalName.trim(), legalAddress: toAddress(values), representatives } }),
      )(),
    saving: base.save.isPending,
    repForm,
    repSheet,
    openRepSheet: () => setRepSheet(true),
    closeRepSheet: () => setRepSheet(false),
    // PUT replaces the whole company: a representative is added to the saved data, so the
    // company data must be saved first (the submit gate of this form, M2).
    addRepresentative: () =>
      void repForm.handleSubmit((values) => {
        const company = data?.company;
        if (!company) {
          notifyInfo('kyc.saveCompanyFirst');
          return;
        }
        const representative = { fullName: values.fullName.trim(), dni: digitsOnly(values.dni), position: values.position.trim() };
        base.save.mutate(
          { company: { ...company, representatives: [...company.representatives.filter((rep) => rep.dni !== representative.dni), representative] } },
          {
            onSuccess: () => {
              repForm.reset();
              setRepSheet(false);
            },
          },
        );
      })(),
  };
}

export function usePersonKyc() {
  const base = useKycCase();
  const form = useForm<PersonValues>({
    resolver: zodResolver(personSchema),
    values: personFormValues(base.kyc.data, base.org?.legalName ?? ''),
    resetOptions: { keepDirtyValues: true },
  });
  return {
    ...base,
    form,
    savePerson: () => void form.handleSubmit((values) => base.save.mutate({ person: toPersonData(values) }))(),
    saving: base.save.isPending,
  };
}
