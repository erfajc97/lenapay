import { describe, expect, it } from 'vitest';

import type { ApprovalPolicy } from '@/shared/api/types';

import { approvalThreshold, DEFAULT_APPROVAL_POLICY, needsApproval, ownerMaySelfApprove, policySummary } from '../approvalPolicy';

// Why: the app tells the person, before step-up, whether a payout leaves at once or waits for a
// second person; a wrong boundary promises "sale sin aprobación" for money that then sits pending
// (or the reverse). The API decides strictly below the threshold (R-OUT-7, orgs.ApprovalRules).
const threshold = (over: Partial<ApprovalPolicy> = {}): Pick<ApprovalPolicy, 'mode' | 'thresholdMinor'> => ({
  mode: 'threshold',
  thresholdMinor: { ARS: 500_000_00, USD: null },
  ...over,
});

describe('needsApproval', () => {
  it('approves strictly below the threshold: threshold − 1 leaves, the threshold waits', () => {
    expect(needsApproval(threshold(), { orgKind: 'company', currency: 'ARS', amount: 500_000_00 - 1 })).toBe(false);
    expect(needsApproval(threshold(), { orgKind: 'company', currency: 'ARS', amount: 500_000_00 })).toBe(true);
  });

  it('always needs approval for a currency without a threshold and in always mode', () => {
    expect(needsApproval(threshold(), { orgKind: 'company', currency: 'USD', amount: 1 })).toBe(true);
    expect(needsApproval(threshold({ mode: 'always' }), { orgKind: 'company', currency: 'ARS', amount: 1 })).toBe(true);
    expect(needsApproval(DEFAULT_APPROVAL_POLICY, { orgKind: 'company', currency: 'ARS', amount: 1 })).toBe(true);
  });

  it('never asks a personal wallet for approval, whatever the policy', () => {
    expect(needsApproval(threshold({ mode: 'always' }), { orgKind: 'personal', currency: 'ARS', amount: 900_000_000_00 })).toBe(false);
  });
});

describe('approvalThreshold', () => {
  it('is the currency threshold in threshold mode, and null otherwise', () => {
    expect(approvalThreshold(threshold(), 'ARS')).toBe(500_000_00);
    expect(approvalThreshold(threshold(), 'USD')).toBeNull();
    // Thresholds are kept but ignored while the mode is `always`.
    expect(approvalThreshold(threshold({ mode: 'always' }), 'ARS')).toBeNull();
  });
});

// Why: self-approval with another approver around would break 4-eyes (R-OUT-4).
describe('ownerMaySelfApprove', () => {
  const allowed = { allowOwnerSelfApproval: true };

  it('holds only while allowed and nobody but the owner can approve', () => {
    expect(ownerMaySelfApprove(allowed, [{ role: 'owner' }, { role: 'operator' }, { role: 'viewer' }])).toBe(true);
    expect(ownerMaySelfApprove(allowed, [{ role: 'owner' }])).toBe(true);
    expect(ownerMaySelfApprove(allowed, [{ role: 'owner' }, { role: 'approver' }])).toBe(false);
    expect(ownerMaySelfApprove(allowed, [{ role: 'owner' }, { role: 'admin' }])).toBe(false);
    expect(ownerMaySelfApprove({ allowOwnerSelfApproval: false }, [{ role: 'owner' }])).toBe(false);
  });
});

describe('policySummary', () => {
  it('reads always, or the thresholds in force in currency order', () => {
    expect(policySummary({ ...threshold(), mode: 'always' })).toEqual({ kind: 'always', thresholds: [] });
    expect(policySummary(threshold())).toEqual({ kind: 'threshold', thresholds: [{ currency: 'ARS', minor: 500_000_00 }] });
    expect(policySummary({ ...threshold(), thresholdMinor: { ARS: 1, USD: 2 } }).thresholds.map((item) => item.currency)).toEqual(['ARS', 'USD']);
  });
});
