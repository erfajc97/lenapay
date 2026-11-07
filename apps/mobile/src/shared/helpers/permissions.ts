import type { Org, Payout, Role } from '@/shared/api/types';

/**
 * Who may do what, decided in one place so every screen disables the same buttons the API
 * would reject (R-KYC-3, R-OUT-4, PRD §2 roles). The API stays the authority; this only
 * avoids offering an action that cannot succeed.
 */
export type MoneyBlocker = 'kyc' | 'role' | null;

const CAN_OPERATE: readonly Role[] = ['operator', 'admin'];
const CAN_APPROVE: readonly Role[] = ['approver', 'admin'];

/** Creating payouts, transfers and CVUs: KYC approved (R-KYC-3) and an operating role. */
export function moneyActionBlocker(org: Pick<Org, 'kycStatus' | 'role'>): MoneyBlocker {
  if (org.kycStatus !== 'approved') return 'kyc';
  if (!CAN_OPERATE.includes(org.role)) return 'role';
  return null;
}

export function canManageTeam(role: Role): boolean {
  return role === 'admin';
}

export function canSeeReconciliation(role: Role): boolean {
  return role === 'admin' || role === 'approver';
}

type ApprovalBlocker = 'creator' | 'role' | 'status' | null;

/** 4-eyes (R-OUT-4): the creator never approves their own payout, whatever their role. */
export function approvalBlocker(params: { payout: Pick<Payout, 'status' | 'createdBy'>; userId: string; role: Role }): ApprovalBlocker {
  if (params.payout.status !== 'pending_approval') return 'status';
  if (params.payout.createdBy.userId === params.userId) return 'creator';
  if (!CAN_APPROVE.includes(params.role)) return 'role';
  return null;
}
