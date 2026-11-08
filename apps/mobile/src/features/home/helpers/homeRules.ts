import type { Balance, Currency, ErpLinkStatus, Org } from '@/shared/api/types';
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

/** Who may link the ERP (R-ERP-2): an admin of a verified company. */
export function canLinkErp(org: Pick<Org, 'kind' | 'kycStatus' | 'role'>): boolean {
  return org.kind === 'company' && org.kycStatus === 'approved' && org.role === 'admin';
}

/** The "¿Usás Lebane ERP?" card: only for who can link, and only once we know it is not linked. */
export function showErpInvite(org: Pick<Org, 'kind' | 'kycStatus' | 'role'>, linkStatus: ErpLinkStatus | undefined): boolean {
  return canLinkErp(org) && linkStatus === 'not_linked';
}

/** Held as a share (0..1) of the total posted balance: the bar under the "Retenido" tile. */
export function heldShare(balance: Balance | null): number {
  if (!balance || balance.posted <= 0) return 0;
  return Math.min(1, Math.max(0, balance.held / balance.posted));
}
