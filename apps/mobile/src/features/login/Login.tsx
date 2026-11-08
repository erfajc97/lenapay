import { router } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { useIsMockApi } from '@/shared/api';
import { AppText } from '@/shared/ui/AppText';
import { Button } from '@/shared/ui/Button';
import { Card } from '@/shared/ui/Card';
import { DemoHint } from '@/shared/ui/DemoHint';
import { FormField } from '@/shared/ui/FormField';
import { Icon } from '@/shared/ui/Icon';
import { LinkButton } from '@/shared/ui/Rows';
import { Screen } from '@/shared/ui/Screen';
import { Wordmark } from '@/shared/ui/Wordmark';

import { useLoginForm } from './hooks/useLoginForm';

/**
 * Sign-in. `provider="lebane"` is "Entrar con tu cuenta Lebane" (R-ID-4): the same login and
 * endpoint (identity provider `lebane`), presented with the Lebane card and user label.
 */
export function Login({ provider = 'email' }: { provider?: 'email' | 'lebane' }) {
  const { t } = useTranslation();
  const { form, submit, pending } = useLoginForm();
  const isMock = useIsMockApi();
  const lebane = provider === 'lebane';
  return (
    <Screen
      title={t(lebane ? 'lebane.title' : 'login.title')}
      back
      footer={lebane ? <Button label={t('lebane.submit')} icon="business" onPress={submit} loading={pending} /> : <Button label={t('login.submit')} onPress={submit} loading={pending} />}
    >
      {lebane ? (
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
      ) : (
        <View className="pb-2 pt-1">
          <Wordmark size="small" />
        </View>
      )}
      <DemoHint visible={isMock} title={t('demo.title')} body={t('demo.login')} />
      <FormField
        control={form.control}
        name="email"
        label={t(lebane ? 'fields.lebaneUser' : 'fields.email')}
        keyboardType="email-address"
        autoCapitalize="none"
        autoComplete="email"
        textContentType={lebane ? 'username' : 'emailAddress'}
        returnKeyType="next"
      />
      <FormField control={form.control} name="password" label={t('fields.password')} secureTextEntry autoComplete="current-password" textContentType="password" returnKeyType="go" onSubmitEditing={submit} />
      {lebane ? null : (
        <View className="items-start gap-1">
          <LinkButton label={t('login.lebane')} onPress={() => router.replace('/lebane')} />
          <LinkButton label={t('login.noAccount')} onPress={() => router.replace('/signup')} />
        </View>
      )}
    </Screen>
  );
}
