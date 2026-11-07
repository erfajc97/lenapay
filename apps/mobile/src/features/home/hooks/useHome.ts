import { router } from 'expo-router';
import { useState } from 'react';

import type { Currency, Movement } from '@/shared/api/types';
import { useBalances } from '@/shared/queries/accounts';
import { useMovements } from '@/shared/queries/movements';
import { useActiveOrg } from '@/shared/queries/orgs';
import { useApprovals } from '@/shared/queries/payouts';
import { useMe } from '@/shared/queries/session';

import { balanceFor, kycNotice, quickActions } from '../helpers/homeRules';

const RECENT_COUNT = 5;

export function useHome() {
  const { query: orgsQuery, org } = useActiveOrg();
  const me = useMe();
  const balances = useBalances(org?.id);
  const approvals = useApprovals(org?.kycStatus === 'approved' ? org.id : undefined);
  const movements = useMovements(org?.kycStatus === 'approved' ? org.id : undefined, {});
  const [currency, setCurrency] = useState<Currency>('ARS');

  return {
    orgsQuery,
    org,
    firstName: me.data?.fullName.split(' ')[0] ?? null,
    noOrgs: orgsQuery.isSuccess && org === null,
    currency,
    setCurrency,
    balances,
    balance: balanceFor(balances.data?.totals, currency),
    actions: org ? quickActions(org) : null,
    kycNotice: org ? kycNotice(org) : null,
    pendingApprovals: approvals.data?.length ?? 0,
    movements,
    recent: movements.data?.pages[0]?.items.slice(0, RECENT_COUNT) ?? [],
    refreshing: balances.isRefetching || movements.isRefetching,
    refresh: () => {
      void orgsQuery.refetch();
      void balances.refetch();
      void approvals.refetch();
      void movements.refetch();
    },
    openMovement: (movement: Movement) => router.push({ pathname: '/movements/[id]', params: { id: movement.id } }),
  };
}
