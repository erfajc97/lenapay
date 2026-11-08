import { router } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { config } from '@/shared/config/env';
import { AppText } from '@/shared/ui/AppText';
import { Button } from '@/shared/ui/Button';
import { Card } from '@/shared/ui/Card';
import { DemoBadge } from '@/shared/ui/DemoBadge';
import { Icon } from '@/shared/ui/Icon';
import { LinkButton } from '@/shared/ui/Rows';
import { Screen } from '@/shared/ui/Screen';
import { Wordmark } from '@/shared/ui/Wordmark';

const HIGHLIGHTS = [
  { icon: 'qr-code-outline', key: 'welcome.highlightCvu' },
  { icon: 'shield-checkmark-outline', key: 'welcome.highlightApprovals' },
  { icon: 'flash-outline', key: 'welcome.highlightLive' },
] as const;

export function Welcome() {
  const { t } = useTranslation();
  return (
    <Screen
      footer={
        <View className="gap-3">
          <Button label={t('welcome.signup')} onPress={() => router.push('/signup')} />
          <Button label={t('welcome.login')} variant="secondary" onPress={() => router.push('/login')} />
          <Button label={t('welcome.lebane')} variant="ghost" icon="business-outline" onPress={() => router.push('/lebane')} />
        </View>
      }
    >
      <View className="gap-6 pt-10">
        <View className="flex-row items-start justify-between">
          <Wordmark byLebane />
          <DemoBadge />
        </View>
        <AppText variant="title">{t('welcome.tagline')}</AppText>
        <AppText tone="secondary">{t('welcome.body')}</AppText>
        <Card className="gap-4">
          {HIGHLIGHTS.map((item) => (
            <View key={item.key} className="flex-row items-center gap-3">
              <View className="h-10 w-10 items-center justify-center rounded-full bg-primary-bg">
                <Icon name={item.icon} size={20} tone="primary" />
              </View>
              <AppText className="flex-1">{t(item.key)}</AppText>
            </View>
          ))}
        </Card>
        {config.devTools ? (
          <View className="items-start">
            <LinkButton label={t('developer.title')} onPress={() => router.push('/developer')} />
          </View>
        ) : null}
      </View>
    </Screen>
  );
}
