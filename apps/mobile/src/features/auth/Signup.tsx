import { router } from 'expo-router';
import { useTranslation } from 'react-i18next';

import { Button } from '@/shared/ui/Button';
import { FormField } from '@/shared/ui/FormField';
import { LinkButton } from '@/shared/ui/Rows';
import { Screen } from '@/shared/ui/Screen';

import { useSignupForm } from './hooks/useAuthForms';

export function Signup() {
  const { t } = useTranslation();
  const { form, submit, pending } = useSignupForm();
  return (
    <Screen title={t('signup.title')} back footer={<Button label={t('signup.submit')} onPress={submit} loading={pending} />}>
      <FormField control={form.control} name="fullName" label={t('fields.fullName')} autoComplete="name" textContentType="name" autoCapitalize="words" returnKeyType="next" />
      <FormField control={form.control} name="email" label={t('fields.email')} keyboardType="email-address" autoCapitalize="none" autoComplete="email" textContentType="emailAddress" returnKeyType="next" />
      <FormField
        control={form.control}
        name="password"
        label={t('fields.password')}
        hint={t('signup.passwordHint')}
        secureTextEntry
        autoComplete="new-password"
        textContentType="newPassword"
        returnKeyType="go"
        onSubmitEditing={submit}
      />
      <LinkButton label={t('signup.haveAccount')} onPress={() => router.replace('/login')} />
    </Screen>
  );
}
