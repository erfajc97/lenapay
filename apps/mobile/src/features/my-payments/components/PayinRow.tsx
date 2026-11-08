import { memo } from 'react';
import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import type { MyPaymentPayin } from '@/shared/api/types';
import { formatDateTime } from '@/shared/i18n/dates';
import { formatMoney } from '@/shared/money/format';
import { AppText } from '@/shared/ui/AppText';
import { Chip } from '@/shared/ui/Chip';
import { IconBadge } from '@/shared/ui/Rows';

import { PAYIN_TONE } from '../helpers/myPaymentsRules';

/** One of my payments to the company: amount, when, into which of my CVUs, its status. */
export const PayinRow = memo(function PayinRow({ payin }: { payin: MyPaymentPayin }) {
  const { t } = useTranslation();
  return (
    <View className="min-h-16 flex-row items-center gap-3 py-2" accessible accessibilityLabel={`${formatMoney(payin.amount, payin.currency)}, ${formatDateTime(payin.createdAt)}, ${t(`status.payin.${payin.status}`)}`}>
      <IconBadge name="arrow-up" />
      <View className="flex-1 gap-0.5">
        <AppText variant="bodyStrong">{formatMoney(payin.amount, payin.currency)}</AppText>
        <AppText variant="caption" tone="muted">
          {formatDateTime(payin.createdAt)}
        </AppText>
        <AppText variant="monoSmall" tone="muted" numberOfLines={1}>
          {t('myPayments.toCvu', { last: payin.cvu.slice(-4) })}
        </AppText>
      </View>
      <Chip label={t(`status.payin.${payin.status}`)} tone={PAYIN_TONE[payin.status]} />
    </View>
  );
});
