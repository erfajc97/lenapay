import { zodResolver } from '@hookform/resolvers/zod';
import { useState } from 'react';
import { useForm } from 'react-hook-form';

import { notifyInfo } from '@/shared/feedback/mutationFeedback';
import { digitsOnly } from '@/shared/helpers/cuit';
import { companyFormValues, toAddress } from '@/shared/kyc/kycChecklist';
import { type CompanyValues, companySchema, type RepresentativeValues, representativeSchema } from '@/shared/kyc/schema';
import { useKycCase } from '@/shared/kyc/useKycCase';

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
