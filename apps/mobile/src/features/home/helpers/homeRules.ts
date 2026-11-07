import type { Balance, Currency, Org } from '@/shared/api/types';
import { type MoneyBlocker, moneyActionBlocker } from '@/shared/helpers/permissions';

/** The org-wide balance of one currency, or null while it is not known (never a fake 0, M6). */
export function balanceFor(totals: Balance[] | undefined, currency: Currency): Balance | null {
  return totals?.find((item) => item.currency === currency) ?? null;
}

export interface QuickActionsState {
  collect: MoneyBlocker;
  pay: MoneyBlocker;
  move: MoneyBlocker;
}

/** Cobrar is browsing payers and their CVUs (always open); Pagar and Mover move money. */
export function quickActions(org: Pick<Org, 'kycStatus' | 'role'>): QuickActionsState {
  const blocker = moneyActionBlocker(org);
  return { collect: org.kycStatus === 'approved' ? null : 'kyc', pay: blocker, move: blocker };
}

type KycNotice = 'draft' | 'submitted' | 'rejected' | null;

export function kycNotice(org: Pick<Org, 'kycStatus'>): KycNotice {
  return org.kycStatus === 'approved' ? null : org.kycStatus;
}
