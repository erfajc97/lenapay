import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import type { Account, Currency } from '@/shared/api/types';
import { formatMoney } from '@/shared/money/format';
import { AppText } from '@/shared/ui/AppText';
import { Card } from '@/shared/ui/Card';
import { Icon } from '@/shared/ui/Icon';

/** Desde → Hacia and the source's balance after the move (only once the amount is valid). */
export function TransferSummary({ from, to, currency, balanceAfter }: { from: Account; to: Account; currency: Currency; balanceAfter: number | null }) {
  const { t } = useTranslation();
  return (
    <Card className="gap-3">
      <View className="flex-row items-center gap-2">
        <View className="flex-1 gap-0.5">
          <AppText variant="caption" tone="muted">
            {t('transfer.from')}
          </AppText>
          <AppText variant="bodyStrong" numberOfLines={2}>
            {from.name}
          </AppText>
        </View>
        <Icon name="arrow-forward" size={20} tone="text-muted" />
        <View className="flex-1 gap-0.5">
          <AppText variant="caption" tone="muted">
            {t('transfer.to')}
          </AppText>
          <AppText variant="bodyStrong" numberOfLines={2}>
            {to.name}
          </AppText>
        </View>
      </View>
      <View className="flex-row items-center justify-between gap-3">
        <AppText variant="label" tone="secondary" className="flex-1">
          {t('transfer.balanceAfter', { account: from.name })}
        </AppText>
        <AppText variant="bodyStrong" tone={balanceAfter !== null && balanceAfter < 0 ? 'danger' : 'primary'}>
          {balanceAfter === null ? '—' : formatMoney(balanceAfter, currency)}
        </AppText>
      </View>
    </Card>
  );
}
