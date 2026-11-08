import { describe, expect, it } from 'vitest';

import type { Payout } from '@/shared/api/types';

import {
  approvalBlocker,
  canEditApprovalPolicy,
  canEditMember,
  canManageTeam,
  canRunReconciliation,
  canSeeReconciliation,
  canTransferOwnership,
  grantableRoles,
  moneyActionBlocker,
  ownershipCandidates,
  roleAtLeast,
} from '../permissions';

// Why: offering "Aprobar" to the person who created the payout breaks 4-eyes in the UI, and
// offering "Pagar" before KYC approval sends people into a flow the API rejects (R-KYC-3).
describe('moneyActionBlocker', () => {
  it('blocks every role until KYC is approved', () => {
    for (const kycStatus of ['draft', 'submitted', 'rejected'] as const) {
      expect(moneyActionBlocker({ kycStatus, role: 'admin' })).toBe('kyc');
    }
  });

  it('lets operators, admins and the owner move money, not viewers or approvers', () => {
    expect(moneyActionBlocker({ kycStatus: 'approved', role: 'operator' })).toBeNull();
    expect(moneyActionBlocker({ kycStatus: 'approved', role: 'admin' })).toBeNull();
    expect(moneyActionBlocker({ kycStatus: 'approved', role: 'owner' })).toBeNull();
    expect(moneyActionBlocker({ kycStatus: 'approved', role: 'viewer' })).toBe('role');
    expect(moneyActionBlocker({ kycStatus: 'approved', role: 'approver' })).toBe('role');
  });
});

describe('approvalBlocker', () => {
  const payout = (over: Partial<Pick<Payout, 'status' | 'createdBy'>> = {}) => ({
    status: 'pending_approval' as const,
    createdBy: { userId: 'u-creator', label: 'Martina López' },
    ...over,
  });

  it('lets another approver, admin or the owner approve', () => {
    expect(approvalBlocker({ payout: payout(), userId: 'u-other', role: 'approver' })).toBeNull();
    expect(approvalBlocker({ payout: payout(), userId: 'u-other', role: 'admin' })).toBeNull();
    expect(approvalBlocker({ payout: payout(), userId: 'u-other', role: 'owner' })).toBeNull();
  });

  it('never lets the creator approve, even as admin', () => {
    expect(approvalBlocker({ payout: payout(), userId: 'u-creator', role: 'admin' })).toBe('creator');
    // An admin never self-approves, whatever the policy says (R-OUT-7).
    expect(approvalBlocker({ payout: payout(), userId: 'u-creator', role: 'admin', ownerMaySelfApprove: true })).toBe('creator');
  });

  // Why: a lone owner with self-approval allowed would otherwise be stuck with payouts nobody can
  // approve; with another approver around, 4-eyes holds for the owner too.
  it('lets the owner approve their own payout only while self-approval is effective', () => {
    expect(approvalBlocker({ payout: payout(), userId: 'u-creator', role: 'owner', ownerMaySelfApprove: true })).toBeNull();
    expect(approvalBlocker({ payout: payout(), userId: 'u-creator', role: 'owner', ownerMaySelfApprove: false })).toBe('creator');
    expect(approvalBlocker({ payout: payout(), userId: 'u-creator', role: 'owner' })).toBe('creator');
  });

  it('blocks operators and viewers', () => {
    expect(approvalBlocker({ payout: payout(), userId: 'u-other', role: 'operator' })).toBe('role');
    expect(approvalBlocker({ payout: payout(), userId: 'u-other', role: 'viewer' })).toBe('role');
  });

  it('blocks payouts that are no longer pending', () => {
    expect(approvalBlocker({ payout: payout({ status: 'approved' }), userId: 'u-other', role: 'admin' })).toBe('status');
  });
});

describe('role gates', () => {
  it('ranks owner above admin above approver above operator above viewer', () => {
    expect(roleAtLeast('owner', 'admin')).toBe(true);
    expect(roleAtLeast('admin', 'admin')).toBe(true);
    expect(roleAtLeast('admin', 'owner')).toBe(false);
    expect(roleAtLeast('approver', 'operator')).toBe(true);
    expect(roleAtLeast('viewer', 'operator')).toBe(false);
  });

  it('limits team management to admins and the owner, reconciliation to approvers and up', () => {
    expect(canManageTeam('owner')).toBe(true);
    expect(canManageTeam('admin')).toBe(true);
    expect(canManageTeam('approver')).toBe(false);
    expect(canSeeReconciliation('owner')).toBe(true);
    expect(canSeeReconciliation('approver')).toBe(true);
    expect(canSeeReconciliation('operator')).toBe(false);
  });

  // Why: the owner decides who moves money; offering these to an admin only shows a 403.
  it('keeps ownership transfer and the approval policy to the owner', () => {
    expect(canTransferOwnership('owner')).toBe(true);
    expect(canEditApprovalPolicy('owner')).toBe(true);
    for (const role of ['viewer', 'operator', 'approver', 'admin'] as const) {
      expect(canTransferOwnership(role)).toBe(false);
      expect(canEditApprovalPolicy(role)).toBe(false);
    }
  });
});

// Why: granting above your own role is escalation (T4); `owner` is never grantable.
describe('grantableRoles', () => {
  it('lets admins and the owner grant up to admin, and nobody else grant anything', () => {
    expect(grantableRoles('owner')).toEqual(['viewer', 'operator', 'approver', 'admin']);
    expect(grantableRoles('admin')).toEqual(['viewer', 'operator', 'approver', 'admin']);
    expect(grantableRoles('approver')).toEqual([]);
    expect(grantableRoles('viewer')).toEqual([]);
  });
});

describe('ownershipCandidates', () => {
  it('offers every other active member, never yourself or a pending invite', () => {
    const members = [
      { userId: 'u-me', status: 'active' as const, role: 'owner' as const },
      { userId: 'u-a', status: 'active' as const, role: 'viewer' as const },
      { userId: 'inv', status: 'invited' as const, role: 'admin' as const },
    ];
    expect(ownershipCandidates(members, 'u-me').map((member) => member.userId)).toEqual(['u-a']);
    expect(ownershipCandidates([], 'u-me')).toEqual([]);
  });
});

// Why: an admin who demotes or removes themselves can leave the org with nobody able to
// approve or manage it; a pending invite has no membership the API could change.
describe('canEditMember', () => {
  const member = { userId: 'u-other', status: 'active' as const, role: 'operator' as const };

  it('lets an admin or the owner edit another active member', () => {
    expect(canEditMember({ role: 'admin', myUserId: 'u-me', member })).toBe(true);
    expect(canEditMember({ role: 'owner', myUserId: 'u-me', member })).toBe(true);
  });

  // Why: the member ladder (R-ORG-3): an admin changing another admin is role.escalation_forbidden
  // and nobody changes the owner (org.last_admin); the buttons must not promise what fails.
  it('lets only the owner act on an admin, and nobody on the owner', () => {
    expect(canEditMember({ role: 'owner', myUserId: 'u-me', member: { ...member, role: 'admin' } })).toBe(true);
    expect(canEditMember({ role: 'admin', myUserId: 'u-me', member: { ...member, role: 'admin' } })).toBe(false);
    expect(canEditMember({ role: 'admin', myUserId: 'u-me', member: { ...member, role: 'owner' } })).toBe(false);
    expect(canEditMember({ role: 'owner', myUserId: 'u-me', member: { ...member, role: 'owner' } })).toBe(false);
  });

  it('never edits yourself, an invite, or as a non-admin', () => {
    expect(canEditMember({ role: 'admin', myUserId: 'u-other', member })).toBe(false);
    expect(canEditMember({ role: 'admin', myUserId: 'u-me', member: { ...member, status: 'invited' } })).toBe(false);
    expect(canEditMember({ role: 'approver', myUserId: 'u-me', member })).toBe(false);
  });
});

// Why: the API answers 403 to anyone else; offering the button would only show an error.
describe('canRunReconciliation', () => {
  it('is for admins and the owner only', () => {
    expect(canRunReconciliation('admin')).toBe(true);
    expect(canRunReconciliation('owner')).toBe(true);
    for (const role of ['viewer', 'operator', 'approver'] as const) expect(canRunReconciliation(role)).toBe(false);
  });
});
