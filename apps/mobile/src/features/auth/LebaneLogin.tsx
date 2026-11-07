import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { isMockApi } from '@/shared/api';
import { AppText } from '@/shared/ui/AppText';
import { Button } from '@/shared/ui/Button';
import { Card } from '@/shared/ui/Card';
import { FormField } from '@/shared/ui/FormField';
import { Icon } from '@/shared/ui/Icon';
import { Screen } from '@/shared/ui/Screen';

import { DemoHint } from './components/DemoHint';
import { useLoginForm } from './hooks/useAuthForms';

/** "Entrar con tu cuenta Lebane" (R-ID-4): same login, identity provider `lebane`. */
export function LebaneLogin() {
  const { t } = useTranslation();
  const { form, submit, pending } = useLoginForm('lebane');
  return (
    <Screen title={t('lebane.title')} back footer={<Button label={t('lebane.submit')} icon="business" onPress={submit} loading={pending} />}>
      <Card className="flex-row items-center gap-3">
        <View className="h-12 w-12 items-center justify-center rounded-card bg-primary-bg">
          <Icon name="business" size={24} tone="primary" />
        </View>
        <View className="flex-1 gap-1">
          <AppText variant="bodyStrong">{t('lebane.cardTitle')}</AppText>
          <AppText variant="label" tone="secondary">
            {t('lebane.cardBody')}
          </AppText>
        </View>
      </Card>
      <DemoHint visible={isMockApi} title={t('demo.title')} body={t('demo.login')} />
      <FormField control={form.control} name="email" label={t('fields.lebaneUser')} keyboardType="email-address" autoCapitalize="none" autoComplete="email" textContentType="username" returnKeyType="next" />
      <FormField control={form.control} name="password" label={t('fields.password')} secureTextEntry autoComplete="current-password" textContentType="password" returnKeyType="go" onSubmitEditing={submit} />
    </Screen>
  );
}
