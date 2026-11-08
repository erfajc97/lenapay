import { router } from 'expo-router';
import { useState } from 'react';

import type { Account } from '@/shared/api/types';
import { moneyActionBlocker } from '@/shared/helpers/permissions';
import { useAccounts as useAccountsQuery } from '@/shared/queries/accounts';
import { useActiveOrg } from '@/shared/queries/orgs';

import { accountNameError, sortAccounts } from '../helpers/accountRules';
import { useCreateAccount } from '../mutations/useAccountMutations';

export function useAccountsScreen() {
  const { org } = useActiveOrg();
  const accounts = useAccountsQuery(org?.id);
  const [showArchived, setShowArchived] = useState(false);
  const [sheetOpen, setSheetOpen] = useState(false);
  const [name, setName] = useState('');
  const [touched, setTouched] = useState(false);
  const create = useCreateAccount(org?.id, () => {
    setSheetOpen(false);
    setName('');
    setTouched(false);
  });

  const all = accounts.data ? sortAccounts(accounts.data) : [];
  const visible = all.filter((account) => showArchived || !account.archived);
  const nameError = accountNameError(name);

  return {
    accounts,
    visible,
    archivedCount: all.filter((account) => account.archived).length,
    showArchived,
    toggleArchived: () => setShowArchived((value) => !value),
    canCreate: org ? moneyActionBlocker(org) === null : false,
    sheetOpen,
    openSheet: () => setSheetOpen(true),
    closeSheet: () => setSheetOpen(false),
    name,
    setName: (value: string) => {
      setName(value);
      setTouched(true);
    },
    nameError: touched ? nameError : null,
    submit: () => {
      setTouched(true);
      if (!nameError) create.mutate(name.trim());
    },
    creating: create.isPending,
    open: (account: Account) => router.push({ pathname: '/accounts/[id]', params: { id: account.id } }),
  };
}
