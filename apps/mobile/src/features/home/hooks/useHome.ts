import { router } from 'expo-router';
import { useState } from 'react';

import type { Currency, Movement } from '@/shared/api/types';
import { notifyInfo } from '@/shared/feedback/mutationFeedback';
import type { MoneyBlocker } from '@/shared/helpers/permissions';
import { useBalances } from '@/shared/queries/accounts';
import { useErpLink } from '@/shared/queries/erp';
import { useMovements } from '@/shared/queries/movements';
import { useActiveOrg } from '@/shared/queries/orgs';
import { useApprovals } from '@/shared/queries/payouts';
import { useMe } from '@/shared/queries/session';
import { useMovementFiltersStore } from '@/shared/stores/useMovementFiltersStore';

import { balanceFor, canLinkErp, kycNotice, quickActions, showErpInvite } from '../helpers/homeRules';

const RECENT_COUNT = 5;

export function useHome() {
  const { query: orgsQuery, org } = useActiveOrg();
  const me = useMe();
  const balances = useBalances(org?.id);
  const approvals = useApprovals(org?.kycStatus === 'approved' ? org.id : undefined);
  const movements = useMovements(org?.kycStatus === 'approved' ? org.id : undefined, {});
  const erpLink = useErpLink(org && canLinkErp(org) ? org.id : undefined);
  const [currency, setCurrency] = useState<Currency>('ARS');
  const showOnly = useMovementFiltersStore((state) => state.showOnly);

  return {
    orgsQuery,
    org,
    firstName: me.data?.fullName?.split(' ')[0] ?? null,
    erpInvite: org ? showErpInvite(org, erpLink.data?.status) : false,
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
    refresh: () => Promise.all([orgsQuery.refetch(), balances.refetch(), approvals.refetch(), movements.refetch()]),
    // The hero tiles open Movements filtered to what they add up: held = pending, in this currency.
    openHeld: () => {
      showOnly({ status: 'pending', currency });
      router.push('/movements');
    },
    openTotal: () => {
      showOnly({ currency });
      router.push('/movements');
    },
    explainBlocked: (blocker: NonNullable<MoneyBlocker>) => notifyInfo(`blockers.${blocker}`),
    openMovement: (movement: Movement) => router.push({ pathname: '/movements/[id]', params: { id: movement.id } }),
  };
}
