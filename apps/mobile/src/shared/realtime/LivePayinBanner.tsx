import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { Pressable, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

import { formatMoney } from '@/shared/money/format';
import { useLivePayinStore } from '@/shared/stores/useLivePayinStore';
import { AppText } from '@/shared/ui/AppText';
import { Icon } from '@/shared/ui/Icon';

const BANNER_MS = 6_000;

/** "Juan Pérez te transfirió $ 150.000,00 al CVU de Torre Palermo" (R-IN-3). */
export function LivePayinBanner() {
  const { t } = useTranslation();
  const insets = useSafeAreaInsets();
  const payin = useLivePayinStore((state) => state.payin);
  const dismiss = useLivePayinStore((state) => state.dismiss);

  useEffect(() => {
    if (!payin) return undefined;
    const timer = setTimeout(dismiss, BANNER_MS);
    return () => clearTimeout(timer);
  }, [payin, dismiss]);

  if (!payin) return null;
  const message = t('live.payin', { name: payin.originName ?? t('movements.unknownCounterparty'), amount: formatMoney(payin.amount, payin.currency), account: payin.accountName });
  return (
    <View pointerEvents="box-none" className="absolute inset-x-0 px-4" style={{ top: insets.top + 8 }}>
      <Pressable
        onPress={dismiss}
        accessibilityRole="alert"
        accessibilityLabel={message}
        accessibilityHint={t('common.dismiss')}
        className="flex-row items-center gap-3 rounded-card bg-primary px-4 py-3"
      >
        <View className="h-9 w-9 items-center justify-center rounded-full bg-primary-active">
          <Icon name="arrow-down" size={18} tone="on-primary" />
        </View>
        <View className="flex-1">
          <AppText variant="caption" tone="onPrimary" className="opacity-80">
            {t('live.title')}
          </AppText>
          <AppText variant="label" tone="onPrimary">
            {message}
          </AppText>
        </View>
      </Pressable>
    </View>
  );
}
