import { router, useLocalSearchParams } from 'expo-router';
import { useState } from 'react';

import type { Movement, Payer } from '@/shared/api/types';
import { accountNameError, archiveBlocker } from '@/shared/helpers/accounts';
import { moneyActionBlocker } from '@/shared/helpers/permissions';
import { useAccounts } from '@/shared/queries/accounts';
import { useMovements } from '@/shared/queries/movements';
import { useActiveOrg } from '@/shared/queries/orgs';
import { usePayers } from '@/shared/queries/payers';

import { useUpdateAccount } from '../mutations/useUpdateAccount';

const PREVIEW_COUNT = 8;

export function useAccountDetail() {
  const params = useLocalSearchParams<{ id?: string }>();
  const accountId = typeof params.id === 'string' ? params.id : undefined;
  const { org } = useActiveOrg();
  const accounts = useAccounts(org?.id);
  const account = accounts.data?.find((item) => item.id === accountId) ?? null;
  const payers = usePayers(org?.id, { account: accountId });
  const movements = useMovements(accountId ? org?.id : undefined, { account: accountId });
  const [sheet, setSheet] = useState<'rename' | 'archive' | null>(null);
  const [name, setName] = useState('');
  const update = useUpdateAccount(org?.id, accountId, () => setSheet(null));

  const canEdit = org ? moneyActionBlocker(org) === null : false;
  const nameError = accountNameError(name);

  return {
    accounts,
    account,
    notFound: accounts.isSuccess && account === null,
    canEdit,
    archive: account ? archiveBlocker(account) : 'main',
    payers,
    payerPreview: payers.data?.slice(0, PREVIEW_COUNT) ?? [],
    movements,
    movementPreview: movements.data?.pages[0]?.items.slice(0, PREVIEW_COUNT) ?? [],
    sheet,
    openRename: () => {
      setName(account?.name ?? '');
      setSheet('rename');
    },
    openArchive: () => setSheet('archive'),
    closeSheet: () => setSheet(null),
    name,
    setName,
    nameError: name === account?.name ? null : nameError,
    rename: () => {
      if (!nameError) update.mutate({ name: name.trim() });
    },
    confirmArchive: () => update.mutate({ archived: true }),
    saving: update.isPending,
    openPayer: (payer: Payer) => router.push({ pathname: '/payers/[id]', params: { id: payer.id } }),
    openMovement: (movement: Movement) => router.push({ pathname: '/movements/[id]', params: { id: movement.id } }),
  };
}
