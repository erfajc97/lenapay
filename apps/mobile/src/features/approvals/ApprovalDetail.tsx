import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { PAYOUT_STATUS_TONE } from '@/shared/helpers/status';
import { formatDateTime } from '@/shared/i18n/dates';
import { AppText } from '@/shared/ui/AppText';
import { Button } from '@/shared/ui/Button';
import { Card } from '@/shared/ui/Card';
import { Chip } from '@/shared/ui/Chip';
import { Money } from '@/shared/ui/Money';
import { Field, Notice } from '@/shared/ui/Rows';
import { Screen } from '@/shared/ui/Screen';
import { Sheet } from '@/shared/ui/Sheet';
import { ErrorState, SkeletonList } from '@/shared/ui/States';
import { TextField } from '@/shared/ui/TextField';

import { useApprovalDetail } from './hooks/useApprovals';

/** Approve or reject with step-up; the creator sees the buttons disabled with the reason. */
export function ApprovalDetail() {
  const { t } = useTranslation();
  const detail = useApprovalDetail();
  const payout = detail.payout.data;

  if (detail.payout.isError) {
    return (
      <Screen back>
        <ErrorState error={detail.payout.error} onRetry={() => void detail.payout.refetch()} />
      </Screen>
    );
  }
  if (!payout) {
    return (
      <Screen back>
        <SkeletonList rows={3} />
      </Screen>
    );
  }

  const canDecide = detail.blocker === null;
  return (
    <Screen
      title={t('approvals.detailTitle')}
      back
      footer={
        payout.status === 'pending_approval' ? (
          <View className="gap-3">
            {detail.blocker === 'creator' || detail.blocker === 'role' ? (
              <AppText variant="label" tone="muted" className="text-center">
                {t(`approvals.blocked.${detail.blocker}`)}
              </AppText>
            ) : null}
            <View className="flex-row gap-3">
              <View className="flex-1">
                <Button label={t('approvals.reject')} variant="danger" onPress={detail.openReject} disabled={!canDecide || detail.deciding} />
              </View>
              <View className="flex-1">
                <Button label={t('approvals.approve')} icon="finger-print" onPress={detail.approve} loading={detail.deciding} disabled={!canDecide} />
              </View>
            </View>
          </View>
        ) : undefined
      }
    >
      <Card className="gap-2">
        <Chip label={t(`status.payout.${payout.status}`)} tone={PAYOUT_STATUS_TONE[payout.status]} />
        <Money minor={payout.amount} currency={payout.currency} variant="title" />
        <View className="flex-row items-center gap-1">
          <AppText variant="caption" tone="muted">
            {t('payout.fee')}
          </AppText>
          <Money minor={payout.fee} currency={payout.currency} variant="caption" tone="muted" />
        </View>
      </Card>
      <Card className="gap-1">
        <Field label={t('approvals.to')} value={payout.destination.holderName} />
        <Field label={t('fields.cbu')} value={payout.destination.cbu} mono />
        <Field label={t('fields.bank')} value={payout.destination.bankName} />
        <Field label={t('payout.fromAccount')} value={payout.accountName} />
        <Field label={t('fields.concept')} value={payout.concept} />
        {payout.attachmentName ? <Field label={t('fields.attachment')} value={payout.attachmentName} /> : null}
        <Field label={t('approvals.createdByLabel')} value={`${payout.createdBy.fullName} · ${formatDateTime(payout.createdAt)}`} />
      </Card>
      {payout.rejectionReason ? <Notice tone="danger" icon="close-circle-outline" title={t('approvals.rejectionReason')} body={payout.rejectionReason} /> : null}

      <Sheet visible={detail.rejecting} title={t('approvals.rejectTitle')} onClose={detail.closeReject} onDismissed={detail.rejectSheetDismissed}>
        <TextField
          label={t('fields.reason')}
          value={detail.reason}
          onChangeText={detail.setReason}
          error={detail.reasonError ? t(detail.reasonError) : null}
          placeholder={t('approvals.reasonPlaceholder')}
          multiline
          autoComplete="off"
          autoFocus
        />
        <Button label={t('approvals.confirmReject')} variant="danger" onPress={detail.reject} loading={detail.deciding} disabled={!detail.canReject} />
      </Sheet>
    </Screen>
  );
}
