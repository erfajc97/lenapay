import { zodResolver } from '@hookform/resolvers/zod';
import * as Crypto from 'expo-crypto';
import { useState } from 'react';
import { useForm } from 'react-hook-form';

import { digitsOnly } from '@/shared/helpers/cuit';
import { useActiveOrg, useKyc } from '@/shared/queries/orgs';

import { canSubmitKyc, kycChecklist } from '../helpers/kycChecklist';
import { useSaveKyc, useSubmitKyc, useUploadKycDocument } from '../mutations/useKycMutations';
import { type CompanyValues, companySchema, type PersonValues, personSchema, type RepresentativeValues, representativeSchema } from '../types';

/** Shared KYC plumbing: the case, uploads, the mock camera and submit. */
function useKycCase() {
  const { org } = useActiveOrg();
  const kyc = useKyc(org?.id);
  const upload = useUploadKycDocument(org?.id);
  const submit = useSubmitKyc(org?.id);
  const save = useSaveKyc(org?.id);
  const [camera, setCamera] = useState<{ representativeId?: string } | null>(null);
  return {
    org,
    kyc,
    save,
    checklist: kyc.data ? kycChecklist(kyc.data) : [],
    canSubmit: kyc.data ? canSubmitKyc(kyc.data) : false,
    submit: () => submit.mutate(),
    submitting: submit.isPending,
    upload: (type: 'bylaws' | 'afip_constancy' | 'dni_front', representativeId?: string) => upload.mutate({ type, representativeId }),
    uploading: upload.isPending,
    camera,
    openCamera: (representativeId?: string) => setCamera({ representativeId }),
    closeCamera: () => setCamera(null),
    capture: () => {
      upload.mutate({ type: 'selfie', representativeId: camera?.representativeId }, { onSuccess: () => setCamera(null) });
    },
  };
}

export function useCompanyKyc() {
  const base = useKycCase();
  const data = base.kyc.data;
  const form = useForm<CompanyValues>({
    resolver: zodResolver(companySchema),
    values: { cuit: data?.cuit ?? '', legalName: data?.legalName ?? base.org?.legalName ?? '', legalAddress: data?.legalAddress ?? '' },
    resetOptions: { keepDirtyValues: true },
  });
  const repForm = useForm<RepresentativeValues>({ resolver: zodResolver(representativeSchema), defaultValues: { fullName: '', dni: '' } });
  const [repSheet, setRepSheet] = useState(false);

  return {
    ...base,
    form,
    saveCompany: () => void form.handleSubmit((values) => base.save.mutate({ kind: 'company', ...values, cuit: digitsOnly(values.cuit) }))(),
    saving: base.save.isPending,
    repForm,
    repSheet,
    openRepSheet: () => setRepSheet(true),
    closeRepSheet: () => setRepSheet(false),
    addRepresentative: () =>
      void repForm.handleSubmit((values) => {
        const representatives = [...(data?.representatives ?? []), { id: Crypto.randomUUID(), fullName: values.fullName.trim(), dni: digitsOnly(values.dni) }];
        base.save.mutate(
          { kind: 'company', representatives },
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
  const data = base.kyc.data;
  const form = useForm<PersonValues>({
    resolver: zodResolver(personSchema),
    values: { fullName: data?.fullName ?? base.org?.legalName ?? '', dni: data?.dni ?? '' },
    resetOptions: { keepDirtyValues: true },
  });
  return {
    ...base,
    form,
    savePerson: () => void form.handleSubmit((values) => base.save.mutate({ kind: 'person', fullName: values.fullName.trim(), dni: digitsOnly(values.dni) }))(),
    saving: base.save.isPending,
  };
}
