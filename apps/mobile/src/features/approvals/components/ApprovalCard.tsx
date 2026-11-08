import { memo } from 'react';
import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import type { Payout } from '@/shared/api/types';
import { formatDateTime } from '@/shared/i18n/dates';
import { AppText } from '@/shared/ui/AppText';
import { PressableCard } from '@/shared/ui/Card';
import { Chip, CurrencyChip } from '@/shared/ui/Chip';
import { Money } from '@/shared/ui/Money';

/**
 * A payout waiting for a second person. "Creado por vos" marks the ones you cannot approve;
 * "Aprobación propia" the owner's own ones while self-approval applies (R-OUT-7).
 */
export const ApprovalCard = memo(function ApprovalCard({
  payout,
  mine,
  selfApproval,
  onPress,
}: {
  payout: Payout;
  mine: boolean;
  selfApproval: boolean;
  onPress: (payout: Payout) => void;
}) {
  const { t } = useTranslation();
  return (
    <PressableCard onPress={() => onPress(payout)} accessibilityLabel={t('approvals.cardA11y', { name: payout.destination.holderName })} className="mb-3 gap-2">
      <View className="flex-row items-center justify-between">
        <CurrencyChip currency={payout.currency} />
        {mine ? (
          <Chip label={t(selfApproval ? 'approvals.mineSelf' : 'approvals.mine')} tone={selfApproval ? 'warning' : 'neutral'} />
        ) : (
          <Chip label={t('status.payout.pending_approval')} tone="warning" />
        )}
      </View>
      <Money minor={payout.amount} currency={payout.currency} variant="heading" />
      <AppText variant="bodyStrong" numberOfLines={1}>
        {payout.destination.holderName}
      </AppText>
      <AppText variant="label" tone="secondary" numberOfLines={2}>
        {payout.concept}
      </AppText>
      <AppText variant="caption" tone="muted">
        {t('approvals.createdBy', { name: payout.createdBy.label ?? t('approvals.unknownPerson'), account: payout.accountName, date: formatDateTime(payout.createdAt) })}
      </AppText>
    </PressableCard>
  );
});
