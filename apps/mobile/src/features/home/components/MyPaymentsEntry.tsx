import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { AppText } from '@/shared/ui/AppText';
import { PressableCard } from '@/shared/ui/Card';
import { Icon } from '@/shared/ui/Icon';
import { IconBadge } from '@/shared/ui/Rows';

/** Entry to "Mis pagos" on the personal wallet's Home (R-MP-2). */
export function MyPaymentsEntry({ onOpen }: { onOpen: () => void }) {
  const { t } = useTranslation();
  return (
    <PressableCard onPress={onOpen} accessibilityLabel={t('home.myPayments.title')} className="flex-row items-center gap-3">
      <IconBadge name="business-outline" />
      <View className="flex-1 gap-0.5">
        <AppText variant="bodyStrong">{t('home.myPayments.title')}</AppText>
        <AppText variant="caption" tone="muted">
          {t('home.myPayments.body')}
        </AppText>
      </View>
      <Icon name="chevron-forward" size={18} tone="text-muted" />
    </PressableCard>
  );
}
