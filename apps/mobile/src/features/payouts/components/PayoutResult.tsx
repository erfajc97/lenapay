import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import type { Payout } from '@/shared/api/types';
import { formatMoney } from '@/shared/money/format';
import { AppText } from '@/shared/ui/AppText';
import { Icon } from '@/shared/ui/Icon';

import { payoutOutcome } from '../helpers/payoutRules';

/** "Enviado para aprobación" or "Transferencia enviada" (R-OUT-4). */
export function PayoutResult({ payout }: { payout: Payout }) {
  const { t } = useTranslation();
  const outcome = payoutOutcome(payout);
  const pending = outcome === 'pending_approval';
  return (
    <View className="items-center gap-4 pt-10" accessibilityLiveRegion="polite">
      <View className={`h-24 w-24 items-center justify-center rounded-full ${pending ? 'bg-warning-bg' : 'bg-success-bg'}`}>
        <Icon name={pending ? 'hourglass-outline' : 'checkmark-circle'} size={52} tone={pending ? 'warning' : 'success'} />
      </View>
      <AppText variant="title" className="text-center">
        {t(`payout.result.${outcome}.title`)}
      </AppText>
      <AppText variant="heading" className="text-center">
        {formatMoney(payout.amount, payout.currency)}
      </AppText>
      <AppText tone="secondary" className="text-center">
        {t(`payout.result.${outcome}.body`, { name: payout.destination.holderName })}
      </AppText>
    </View>
  );
}
