import { type Member, type MemberRole, type Org, type Payout, type Role, ROLES } from '@/shared/api/types';

/**
 * Who may do what, decided in one place so every screen disables the same buttons the API
 * would reject (R-KYC-3, R-OUT-4, R-ORG-3, PRD §2 roles). The API stays the authority; this
 * only avoids offering an action that cannot succeed.
 */
export type MoneyBlocker = 'kyc' | 'role' | null;

/** viewer < operator < approver < admin < owner (contract `MemberRole`). */
const RANK: Record<MemberRole, number> = { viewer: 0, operator: 1, approver: 2, admin: 3, owner: 4 };

export function roleAtLeast(role: MemberRole, minimum: MemberRole): boolean {
  return RANK[role] >= RANK[minimum];
}

const CAN_OPERATE: readonly MemberRole[] = ['operator', 'admin', 'owner'];
const CAN_APPROVE: readonly MemberRole[] = ['approver', 'admin', 'owner'];

/** Creating payouts, transfers and CVUs: KYC approved (R-KYC-3) and an operating role. */
export function moneyActionBlocker(org: Pick<Org, 'kycStatus' | 'role'>): MoneyBlocker {
  if (org.kycStatus !== 'approved') return 'kyc';
  if (!CAN_OPERATE.includes(org.role)) return 'role';
  return null;
}

export function canManageTeam(role: MemberRole): boolean {
  return roleAtLeast(role, 'admin');
}

/** Roles a manager may grant (invites, role changes): never above their own, never `owner` (T4). */
export function grantableRoles(role: MemberRole): Role[] {
  return canManageTeam(role) ? ROLES.filter((candidate) => roleAtLeast(role, candidate)) : [];
}

/**
 * The member ladder (R-ORG-3): an admin or the owner edits other people's roles, never their own
 * and never an invite; only the owner acts on an admin; nobody acts on the owner (ownership
 * moves only by transfer).
 */
export function canEditMember(params: { role: MemberRole; myUserId: string | null; member: Pick<Member, 'userId' | 'status' | 'role'> }): boolean {
  const { role, myUserId, member } = params;
  if (!canManageTeam(role) || member.status !== 'active' || member.userId === myUserId) return false;
  if (member.role === 'owner') return false;
  return member.role !== 'admin' || role === 'owner';
}

/** Ownership transfer and the approval policy decide who moves money: the owner only (step-up). */
export function canTransferOwnership(role: MemberRole): boolean {
  return role === 'owner';
}

export function canEditApprovalPolicy(role: MemberRole): boolean {
  return role === 'owner';
}

/** Who can receive the ownership: any other active member (an invite has no membership yet). */
export function ownershipCandidates<M extends Pick<Member, 'userId' | 'status'>>(members: M[], myUserId: string | null): M[] {
  return members.filter((member) => member.status === 'active' && member.userId !== myUserId);
}

/** On-demand reconciliation runs are queued by admins and the owner (POST /reconciliation/runs). */
export function canRunReconciliation(role: MemberRole): boolean {
  return roleAtLeast(role, 'admin');
}

export function canSeeReconciliation(role: MemberRole): boolean {
  return roleAtLeast(role, 'approver');
}

type ApprovalBlocker = 'creator' | 'role' | 'status' | null;

/**
 * 4-eyes (R-OUT-4): the creator never approves their own payout, whatever their role — except
 * the owner while the policy's self-approval is effective (allowed and nobody else can approve,
 * R-OUT-7: `ApprovalPolicy.ownerSelfApprovalEffective`).
 */
export function approvalBlocker(params: {
  payout: Pick<Payout, 'status' | 'createdBy'>;
  userId: string;
  role: MemberRole;
  ownerMaySelfApprove?: boolean;
}): ApprovalBlocker {
  if (params.payout.status !== 'pending_approval') return 'status';
  const selfApprovalAllowed = params.role === 'owner' && params.ownerMaySelfApprove === true;
  if (params.payout.createdBy.userId === params.userId && !selfApprovalAllowed) return 'creator';
  if (!CAN_APPROVE.includes(params.role)) return 'role';
  return null;
}
