import { describe, expect, it } from 'vitest';

import type { Payout } from '@/shared/api/types';

import { approvalBlocker, canEditMember, canManageTeam, canRunReconciliation, canSeeReconciliation, moneyActionBlocker } from '../permissions';

// Why: offering "Aprobar" to the person who created the payout breaks 4-eyes in the UI, and
// offering "Pagar" before KYC approval sends people into a flow the API rejects (R-KYC-3).
describe('moneyActionBlocker', () => {
  it('blocks every role until KYC is approved', () => {
    for (const kycStatus of ['draft', 'submitted', 'rejected'] as const) {
      expect(moneyActionBlocker({ kycStatus, role: 'admin' })).toBe('kyc');
    }
  });

  it('lets operators and admins move money, not viewers or approvers', () => {
    expect(moneyActionBlocker({ kycStatus: 'approved', role: 'operator' })).toBeNull();
    expect(moneyActionBlocker({ kycStatus: 'approved', role: 'admin' })).toBeNull();
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

  it('lets another approver or admin approve', () => {
    expect(approvalBlocker({ payout: payout(), userId: 'u-other', role: 'approver' })).toBeNull();
    expect(approvalBlocker({ payout: payout(), userId: 'u-other', role: 'admin' })).toBeNull();
  });

  it('never lets the creator approve, even as admin', () => {
    expect(approvalBlocker({ payout: payout(), userId: 'u-creator', role: 'admin' })).toBe('creator');
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
  it('limits team management to admins and reconciliation to admins and approvers', () => {
    expect(canManageTeam('admin')).toBe(true);
    expect(canManageTeam('approver')).toBe(false);
    expect(canSeeReconciliation('approver')).toBe(true);
    expect(canSeeReconciliation('operator')).toBe(false);
  });
});

// Why: an admin who demotes or removes themselves can leave the org with nobody able to
// approve or manage it; a pending invite has no membership the API could change.
describe('canEditMember', () => {
  const member = { userId: 'u-other', status: 'active' as const };

  it('lets an admin edit another active member', () => {
    expect(canEditMember({ role: 'admin', myUserId: 'u-me', member })).toBe(true);
  });

  it('never edits yourself, an invite, or as a non-admin', () => {
    expect(canEditMember({ role: 'admin', myUserId: 'u-other', member })).toBe(false);
    expect(canEditMember({ role: 'admin', myUserId: 'u-me', member: { ...member, status: 'invited' } })).toBe(false);
    expect(canEditMember({ role: 'approver', myUserId: 'u-me', member })).toBe(false);
  });
});

// Why: the API answers 403 to anyone else; offering the button would only show an error.
describe('canRunReconciliation', () => {
  it('is for admins only', () => {
    expect(canRunReconciliation('admin')).toBe(true);
    for (const role of ['viewer', 'operator', 'approver'] as const) expect(canRunReconciliation(role)).toBe(false);
  });
});
