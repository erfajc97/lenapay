import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import type { InternalTransfer } from '@/shared/api/types';
import { formatMoney } from '@/shared/money/format';
import { AppText } from '@/shared/ui/AppText';
import { Icon } from '@/shared/ui/Icon';

/** The last step: what moved, from where to where. */
export function TransferResult({ transfer, fromName, toName }: { transfer: InternalTransfer; fromName: string; toName: string }) {
  const { t } = useTranslation();
  return (
    <View className="items-center gap-4 pt-10" accessibilityLiveRegion="polite">
      <View className="h-24 w-24 items-center justify-center rounded-full bg-success-bg">
        <Icon name="checkmark-circle" size={52} tone="success" />
      </View>
      <AppText variant="title" className="text-center">
        {t('transfer.done', { amount: formatMoney(transfer.amount, transfer.currency) })}
      </AppText>
      <AppText tone="secondary" className="text-center">
        {t('transfer.resultBody', { from: fromName, to: toName })}
      </AppText>
      {transfer.concept ? (
        <AppText variant="label" tone="muted" className="text-center">
          {transfer.concept}
        </AppText>
      ) : null}
    </View>
  );
}
