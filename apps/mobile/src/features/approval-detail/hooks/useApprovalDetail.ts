import { router, useLocalSearchParams } from 'expo-router';
import { useRef, useState } from 'react';

import { approvalBlocker } from '@/shared/helpers/permissions';
import { useActiveOrg, useApprovalPolicy } from '@/shared/queries/orgs';
import { usePayout } from '@/shared/queries/payouts';
import { useMe } from '@/shared/queries/session';

import { useDecidePayout } from '../mutations/useDecidePayout';

/** Rejection reason: required, so the creator knows what to fix. */
function rejectReasonError(reason: string): 'validation.reason' | null {
  return reason.trim().length >= 5 ? null : 'validation.reason';
}

export function useApprovalDetail() {
  const params = useLocalSearchParams<{ id?: string }>();
  const payoutId = typeof params.id === 'string' ? params.id : undefined;
  const { org } = useActiveOrg();
  const me = useMe();
  const payout = usePayout(org?.id, payoutId);
  const policy = useApprovalPolicy(org?.role === 'owner' ? org.id : undefined);
  const [rejecting, setRejecting] = useState(false);
  const [reason, setReason] = useState('');
  // Leaving while the reason sheet still slides out freezes iOS: go back on its dismissal.
  const leaveAfterSheet = useRef(false);
  const decide = useDecidePayout(org?.id, payoutId, (kind) => {
    if (kind === 'approve') {
      router.back();
      return;
    }
    leaveAfterSheet.current = true;
    setRejecting(false);
  });

  const ownerMaySelfApprove = policy.data?.ownerSelfApprovalEffective === true;
  const blocker = payout.data && me.data && org ? approvalBlocker({ payout: payout.data, userId: me.data.id, role: org.role, ownerMaySelfApprove }) : 'status';
  const reasonError = rejectReasonError(reason);

  return {
    payout,
    blocker,
    // Allowed AND my own: the owner approving their payout with no other approver (R-OUT-7).
    selfApproval: blocker === null && payout.data?.createdBy.userId === me.data?.id,
    approve: () => decide.mutate({ kind: 'approve' }),
    deciding: decide.isPending,
    rejecting,
    openReject: () => setRejecting(true),
    closeReject: () => setRejecting(false),
    rejectSheetDismissed: () => {
      if (!leaveAfterSheet.current) return;
      leaveAfterSheet.current = false;
      router.back();
    },
    reason,
    setReason,
    reasonError: reason === '' ? null : reasonError,
    canReject: reasonError === null,
    reject: () => {
      if (!reasonError) decide.mutate({ kind: 'reject', reason: reason.trim() });
    },
  };
}
