import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { captureException } from '@/shared/reporting/reporting';

import { AppText } from './AppText';
import { Button } from './Button';
import { Icon } from './Icon';

/** Root error boundary screen (ME1, M12): reports through the adapter and offers a retry. */
export function RootError({ error, retry }: { error: Error; retry: () => Promise<void> }) {
  const { t } = useTranslation();
  captureException(error, { name: error.name });
  return (
    <View className="flex-1 items-center justify-center gap-4 bg-bg-page px-8">
      <View className="h-16 w-16 items-center justify-center rounded-full bg-danger-bg">
        <Icon name="warning-outline" size={30} tone="danger" />
      </View>
      <AppText variant="heading" className="text-center">
        {t('common.crashTitle')}
      </AppText>
      <AppText tone="secondary" className="text-center">
        {t('common.crashBody')}
      </AppText>
      <Button label={t('common.retry')} onPress={() => void retry()} icon="refresh" />
    </View>
  );
}
