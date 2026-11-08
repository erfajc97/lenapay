import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';

import { personFormValues, toPersonData } from '@/shared/kyc/kycChecklist';
import { type PersonValues, personSchema } from '@/shared/kyc/schema';
import { useKycCase } from '@/shared/kyc/useKycCase';

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
