import { router } from 'expo-router';

import type { Payout } from '@/shared/api/types';
import { useActiveOrg, useApprovalPolicy } from '@/shared/queries/orgs';
import { useApprovals } from '@/shared/queries/payouts';
import { useMe } from '@/shared/queries/session';

export function useApprovalsScreen() {
  const { org } = useActiveOrg();
  const me = useMe();
  const approvals = useApprovals(org?.id);
  const policy = useApprovalPolicy(org?.role === 'owner' ? org.id : undefined);
  return {
    approvals,
    userId: me.data?.id ?? null,
    // The owner's own payouts are theirs to approve while self-approval is effective (R-OUT-7).
    selfApprovalAllowed: org?.role === 'owner' && policy.data?.ownerSelfApprovalEffective === true,
    open: (payout: Payout) => router.push({ pathname: '/approvals/[id]', params: { id: payout.id } }),
  };
}
