import { router } from 'expo-router';
import { useDeferredValue, useState } from 'react';

import type { Payer } from '@/shared/api/types';
import { moneyActionBlocker } from '@/shared/helpers/permissions';
import { useActiveOrg } from '@/shared/queries/orgs';
import { usePayers } from '@/shared/queries/payers';

export function usePayersScreen() {
  const { org } = useActiveOrg();
  const [search, setSearch] = useState('');
  const deferredSearch = useDeferredValue(search.trim());
  const payers = usePayers(org?.id, { search: deferredSearch || undefined });
  return {
    payers,
    search,
    setSearch,
    searching: deferredSearch !== '',
    canCreate: org ? moneyActionBlocker(org) === null : false,
    create: () => router.push('/payers/new'),
    open: (payer: Payer) => router.push({ pathname: '/payers/[id]', params: { id: payer.id } }),
  };
}
