import { useMutation, useQueryClient } from '@tanstack/react-query';

import { api } from '@/shared/api';
import type { CreatePayoutRequest, LocalUpload, Payout } from '@/shared/api/types';
import { confirmHaptic } from '@/shared/feedback/haptics';
import { notifyError, notifyInfo } from '@/shared/feedback/mutationFeedback';
import { captureException } from '@/shared/reporting/reporting';
import { ACCOUNTS_KEY, BALANCES_KEY } from '@/shared/queries/accounts';
import { MOVEMENTS_KEY } from '@/shared/queries/movements';
import { APPROVALS_KEY } from '@/shared/queries/payouts';
import { runMoneyAction } from '@/shared/stepup/moneyAction';

interface CreatePayoutInput {
  body: CreatePayoutRequest;
  /** The invoice picked in the form, uploaded once the payout exists (multipart). */
  attachment: LocalUpload | null;
}

/**
 * Create the payout (R-OUT-1..4): step-up, persisted Idempotency-Key, then the hold shows up
 * in balances and the payout in approvals and movements. The result step is the feedback.
 * The attachment goes after: a failed upload never undoes (or repeats) the payout.
 */
export function useCreatePayout(orgId: string | undefined, onCreated: (payout: Payout) => void) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ body, attachment }: CreatePayoutInput) => {
      const payout = await runMoneyAction({ scope: `payout:${orgId ?? ''}`, payload: body, call: (options) => api.payouts.create(orgId ?? '', body, options) });
      if (!attachment) return { payout, attachmentFailed: false };
      try {
        const uploaded = await api.payouts.uploadAttachment(orgId ?? '', payout.id, attachment);
        return { payout: { ...payout, attachments: [...payout.attachments, uploaded] }, attachmentFailed: false };
      } catch (error) {
        captureException(error, { area: 'payout_attachment' });
        return { payout, attachmentFailed: true };
      }
    },
    onSuccess: ({ payout, attachmentFailed }) => {
      for (const key of [BALANCES_KEY, ACCOUNTS_KEY, APPROVALS_KEY, MOVEMENTS_KEY]) {
        void queryClient.invalidateQueries({ queryKey: key(orgId ?? '') });
      }
      confirmHaptic();
      if (attachmentFailed) notifyInfo('payout.attachmentFailed');
      onCreated(payout);
    },
    onError: notifyError,
  });
}
