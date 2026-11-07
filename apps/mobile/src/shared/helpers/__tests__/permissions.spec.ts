import { describe, expect, it } from 'vitest';

import type { Payout } from '@/shared/api/types';

import { approvalBlocker, canManageTeam, canSeeReconciliation, moneyActionBlocker } from '../permissions';

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
    createdBy: { userId: 'u-creator', fullName: 'Martina López' },
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
