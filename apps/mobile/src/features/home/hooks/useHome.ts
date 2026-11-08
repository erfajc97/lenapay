import { router } from 'expo-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import type { AccountCvu, Currency, Movement } from '@/shared/api/types';
import { notifyInfo } from '@/shared/feedback/mutationFeedback';
import { aliasError } from '@/shared/helpers/cbu';
import { verifiedPersonalOrgId } from '@/shared/helpers/orgs';
import type { MoneyBlocker } from '@/shared/helpers/permissions';
import { useCopy, useShareText } from '@/shared/mutations/useDeviceActions';
import { useAccounts, useBalances } from '@/shared/queries/accounts';
import { useErpLink } from '@/shared/queries/erp';
import { useMovements } from '@/shared/queries/movements';
import { useActiveOrg } from '@/shared/queries/orgs';
import { useApprovals } from '@/shared/queries/payouts';
import { useMe } from '@/shared/queries/session';
import { useMovementFiltersStore } from '@/shared/stores/useMovementFiltersStore';

import { balanceFor, canLinkErp, kycActionRoute, kycNotice, personalCvus, quickActions, showErpInvite } from '../helpers/homeRules';
import { useUpdateAccountCvuAlias } from '../mutations/useUpdateAccountCvuAlias';

const RECENT_COUNT = 5;

type CvuSheet = { kind: 'qr' | 'alias'; cvu: AccountCvu } | null;

/** The personal wallet's extras (R-CVU-4, R-MP-2): "Mi CVU" with its sheets and the "Mis pagos" entry. */
function usePersonalWallet(orgId: string | undefined) {
  const { t } = useTranslation();
  const me = useMe();
  const accounts = useAccounts(orgId);
  const [sheet, setSheet] = useState<CvuSheet>(null);
  const [alias, setAlias] = useState('');
  const closeSheet = () => setSheet(null);
  const copy = useCopy();
  const share = useShareText();
  const updateAlias = useUpdateAccountCvuAlias(orgId, closeSheet);
  const cvus = personalCvus(accounts.data);
  const accountId = accounts.data?.find((account) => account.kind === 'personal')?.id ?? null;
  const invalidAlias = aliasError(alias);
  return {
    cvus,
    showMyPayments: orgId !== undefined && verifiedPersonalOrgId(me.data?.memberships) === orgId,
    sheet,
    closeSheet,
    openQr: (cvu: AccountCvu) => setSheet({ kind: 'qr', cvu }),
    openAlias: (cvu: AccountCvu) => {
      setAlias(cvu.alias ?? '');
      setSheet({ kind: 'alias', cvu });
    },
    alias,
    setAlias,
    aliasError: invalidAlias,
    saveAlias: () => {
      if (sheet && accountId && !invalidAlias) updateAlias.mutate({ accountId, cvuId: sheet.cvu.id, alias });
    },
    savingAlias: updateAlias.isPending,
    copy: copy.mutate,
    share: (cvu: AccountCvu) => share.mutate(t('home.myCvu.shareMessage', { currency: t(`currency.${cvu.currency}`), cvu: cvu.cvu, alias: cvu.alias ?? '-' })),
  };
}

export function useHome() {
  const { query: orgsQuery, org } = useActiveOrg();
  const me = useMe();
  const balances = useBalances(org?.id);
  const approvals = useApprovals(org?.kycStatus === 'approved' ? org.id : undefined);
  const movements = useMovements(org?.kycStatus === 'approved' ? org.id : undefined, {});
  const erpLink = useErpLink(org && canLinkErp(org) ? org.id : undefined);
  const [currency, setCurrency] = useState<Currency>('ARS');
  const showOnly = useMovementFiltersStore((state) => state.showOnly);
  const personal = org?.kind === 'personal';
  const wallet = usePersonalWallet(personal ? org.id : undefined);

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
    kycCopy: personal ? 'home.kycPersonal' : 'home.kyc',
    openKyc: () => {
      if (org) router.push(kycActionRoute(org));
    },
    personal,
    wallet,
    // Cobrar on a personal wallet shows my own CVU (QR): there are no payers to pick.
    collect: () => {
      if (!personal) router.push('/payers');
      else if (wallet.cvus[0]) wallet.openQr(wallet.cvus[0]);
      else notifyInfo('blockers.kyc');
    },
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
