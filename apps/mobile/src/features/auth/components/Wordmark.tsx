import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { AppText } from '@/shared/ui/AppText';
import { Icon } from '@/shared/ui/Icon';

/** "Lenapay" in Plus Jakarta Sans 700, primary colour (brand.md logo rule). */
export function Wordmark({ size = 'large' }: { size?: 'large' | 'small' }) {
  const { t } = useTranslation();
  return (
    <View className="flex-row items-center gap-2" accessibilityRole="header" accessibilityLabel={t('brand.name')}>
      <View className={`items-center justify-center rounded-card bg-primary ${size === 'large' ? 'h-12 w-12' : 'h-9 w-9'}`}>
        <Icon name="flash" size={size === 'large' ? 26 : 20} tone="on-primary" />
      </View>
      <AppText variant={size === 'large' ? 'balance' : 'heading'} tone="brand">
        {t('brand.name')}
      </AppText>
    </View>
  );
}
