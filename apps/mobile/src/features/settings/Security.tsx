import { useTranslation } from 'react-i18next';
import { Switch, View } from 'react-native';

import { AppText } from '@/shared/ui/AppText';
import { Button } from '@/shared/ui/Button';
import { Card } from '@/shared/ui/Card';
import { Chip } from '@/shared/ui/Chip';
import { Divider, IconBadge, ListRow, SectionTitle } from '@/shared/ui/Rows';
import { Screen } from '@/shared/ui/Screen';

import { useSecurity } from './hooks/useSecurity';

/** Biometrics, MFA status and push notifications. */
export function Security() {
  const { t } = useTranslation();
  const security = useSecurity();
  return (
    <Screen title={t('security.title')} back>
      <SectionTitle title={t('security.unlock')} />
      <Card className="py-1">
        <ListRow
          title={t('security.biometrics')}
          subtitle={security.biometricsAvailable ? t('security.biometricsBody') : t('security.biometricsUnavailable')}
          leading={<IconBadge name="finger-print" />}
          trailing={
            <Switch
              value={security.biometricsEnabled && security.biometricsAvailable}
              onValueChange={security.setBiometrics}
              disabled={!security.biometricsAvailable || security.toggling}
              accessibilityLabel={t('security.biometrics')}
              trackColorOnClassName="accent-primary"
            />
          }
        />
        <Divider />
        <ListRow
          title={t('security.mfa')}
          subtitle={t('security.mfaBody')}
          leading={<IconBadge name="key-outline" />}
          trailing={<Chip label={security.me.data?.mfaEnabled ? t('security.mfaOn') : t('security.mfaOff')} tone={security.me.data?.mfaEnabled ? 'success' : 'neutral'} />}
        />
      </Card>
      <SectionTitle title={t('security.stepUpTitle')} />
      <AppText variant="label" tone="secondary">
        {t('security.stepUpBody')}
      </AppText>
      <SectionTitle title={t('security.notifications')} />
      <Card className="gap-3">
        <View className="flex-row gap-3">
          <IconBadge name="notifications-outline" />
          <AppText variant="label" tone="secondary" className="flex-1">
            {t('security.pushBody')}
          </AppText>
        </View>
        <Button label={t('security.pushEnable')} variant="secondary" onPress={security.enablePush} loading={security.enablingPush} compact />
      </Card>
    </Screen>
  );
}
