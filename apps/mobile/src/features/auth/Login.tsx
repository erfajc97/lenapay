import { router } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { isMockApi } from '@/shared/api';
import { Button } from '@/shared/ui/Button';
import { FormField } from '@/shared/ui/FormField';
import { LinkButton } from '@/shared/ui/Rows';
import { Screen } from '@/shared/ui/Screen';

import { DemoHint } from './components/DemoHint';
import { useLoginForm } from './hooks/useAuthForms';

export function Login() {
  const { t } = useTranslation();
  const { form, submit, pending } = useLoginForm();
  return (
    <Screen title={t('login.title')} back footer={<Button label={t('login.submit')} onPress={submit} loading={pending} />}>
      <DemoHint visible={isMockApi} title={t('demo.title')} body={t('demo.login')} />
      <FormField control={form.control} name="email" label={t('fields.email')} keyboardType="email-address" autoCapitalize="none" autoComplete="email" textContentType="emailAddress" returnKeyType="next" />
      <FormField control={form.control} name="password" label={t('fields.password')} secureTextEntry autoComplete="current-password" textContentType="password" returnKeyType="go" onSubmitEditing={submit} />
      <View className="items-start gap-1">
        <LinkButton label={t('login.lebane')} onPress={() => router.replace('/lebane')} />
        <LinkButton label={t('login.noAccount')} onPress={() => router.replace('/signup')} />
      </View>
    </Screen>
  );
}
