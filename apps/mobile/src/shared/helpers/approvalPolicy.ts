import { type ApprovalMode, type ApprovalPolicy, CURRENCIES, type Currency, type MemberRole, type OrgKind } from '@/shared/api/types';

/**
 * The approval policy decision (R-OUT-7), the app's copy of the API's ApprovalRules (orgs package): the
 * mock backend decides with it and the payout review announces it before the step-up.
 */
type PolicyRules = Pick<ApprovalPolicy, 'mode' | 'thresholdMinor'>;

/** The API's policy until the owner changes it: every company payout needs approval. */
export const DEFAULT_APPROVAL_POLICY: ApprovalPolicy = {
  mode: 'always',
  thresholdMinor: { ARS: null, USD: null },
  allowOwnerSelfApproval: true,
  ownerSelfApprovalEffective: false,
  updatedBy: null,
  updatedAt: null,
};

/** The threshold in force for a currency: null when every payout of it needs approval. */
export function approvalThreshold(policy: PolicyRules, currency: Currency): number | null {
  if (policy.mode !== 'threshold') return null;
  return policy.thresholdMinor[currency];
}

/**
 * A company payout leaves at once only when its amount (fee not included) is STRICTLY below its
 * currency's threshold; a personal wallet never needs approval.
 */
export function needsApproval(policy: PolicyRules, payout: { orgKind: OrgKind; currency: Currency; amount: number }): boolean {
  if (payout.orgKind === 'personal') return false;
  const limit = approvalThreshold(policy, payout.currency);
  return limit === null || !(payout.amount < limit);
}

/** Self-approval is effective while allowed and nobody but the owner has a role ≥ approver. */
export function ownerMaySelfApprove(policy: Pick<ApprovalPolicy, 'allowOwnerSelfApproval'>, members: { role: MemberRole }[]): boolean {
  return policy.allowOwnerSelfApproval && !members.some((member) => member.role === 'approver' || member.role === 'admin');
}

/** What Settings and the read-only view say about the policy in force. */
export function policySummary(policy: Pick<ApprovalPolicy, 'mode' | 'thresholdMinor'>): { kind: ApprovalMode; thresholds: { currency: Currency; minor: number }[] } {
  if (policy.mode === 'always') return { kind: 'always', thresholds: [] };
  const thresholds = CURRENCIES.flatMap((currency) => {
    const minor = approvalThreshold(policy, currency);
    return minor === null ? [] : [{ currency, minor }];
  });
  return { kind: 'threshold', thresholds };
}
