import { zodResolver } from '@hookform/resolvers/zod';
import { useForm, useWatch } from 'react-hook-form';

import type { Currency } from '@/shared/api/types';
import { digitsOnly } from '@/shared/helpers/cuit';
import { useAccounts } from '@/shared/queries/accounts';
import { useActiveOrg } from '@/shared/queries/orgs';

import { useCreatePayer } from '../mutations/useCreatePayer';
import { payerSchema, type PayerValues } from '../types';

export function usePayerForm() {
  const { org } = useActiveOrg();
  const accounts = useAccounts(org?.id);
  const projects = (accounts.data ?? []).filter((account) => account.kind === 'project' && !account.archived);
  const create = useCreatePayer(org?.id);
  const form = useForm<PayerValues>({
    resolver: zodResolver(payerSchema),
    defaultValues: { accountId: '', name: '', docType: 'cuit', docNumber: '', email: '', currencies: ['ARS'] },
  });
  const currencies = useWatch({ control: form.control, name: 'currencies' });

  return {
    form,
    accounts,
    projects,
    toggleCurrency: (currency: Currency) => {
      const next = currencies.includes(currency) ? currencies.filter((item) => item !== currency) : [...currencies, currency];
      form.setValue('currencies', next, { shouldValidate: true });
    },
    submit: () =>
      void form.handleSubmit((values) =>
        create.mutate({
          body: { accountId: values.accountId, name: values.name.trim(), docType: values.docType, docNumber: digitsOnly(values.docNumber), email: values.email || undefined },
          currencies: values.currencies,
        }),
      )(),
    pending: create.isPending,
  };
}
