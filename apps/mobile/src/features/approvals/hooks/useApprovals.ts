import { router, useLocalSearchParams } from 'expo-router';
import { useRef, useState } from 'react';

import type { Payout } from '@/shared/api/types';
import { approvalBlocker } from '@/shared/helpers/permissions';
import { useActiveOrg } from '@/shared/queries/orgs';
import { useApprovals, usePayout } from '@/shared/queries/payouts';
import { useMe } from '@/shared/queries/session';

import { useDecidePayout } from '../mutations/useDecidePayout';

export function useApprovalsScreen() {
  const { org } = useActiveOrg();
  const me = useMe();
  const approvals = useApprovals(org?.id);
  return {
    approvals,
    userId: me.data?.id ?? null,
    open: (payout: Payout) => router.push({ pathname: '/approvals/[id]', params: { id: payout.id } }),
  };
}

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

  const blocker = payout.data && me.data && org ? approvalBlocker({ payout: payout.data, userId: me.data.id, role: org.role }) : 'status';
  const reasonError = rejectReasonError(reason);

  return {
    payout,
    blocker,
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
