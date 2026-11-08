import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { useDemoEmailCode } from '@/shared/api';
import { AppText } from '@/shared/ui/AppText';
import { Button } from '@/shared/ui/Button';
import { DemoHint } from '@/shared/ui/DemoHint';
import { FormField } from '@/shared/ui/FormField';
import { LinkButton } from '@/shared/ui/Rows';
import { Screen } from '@/shared/ui/Screen';

import { useVerifyForm } from './hooks/useVerifyForm';

/** 6-digit email code (R-ID-1). */
export function VerifyEmail() {
  const { t } = useTranslation();
  const { form, submit, pending, email, resend } = useVerifyForm();
  const demoEmailCode = useDemoEmailCode();
  return (
    <Screen title={t('verify.title')} back footer={<Button label={t('verify.submit')} onPress={submit} loading={pending} />}>
      <AppText tone="secondary">{t('verify.body', { email: email ?? '' })}</AppText>
      <DemoHint visible={demoEmailCode !== null} title={t('demo.title')} body={t('demo.code', { code: demoEmailCode ?? '' })} />
      <FormField
        control={form.control}
        name="code"
        label={t('fields.code')}
        keyboardType="number-pad"
        autoComplete="one-time-code"
        textContentType="oneTimeCode"
        maxLength={6}
        mono
        returnKeyType="go"
        onSubmitEditing={submit}
        autoFocus
      />
      <View className="items-start">
        <LinkButton label={t('verify.resend')} onPress={resend} />
      </View>
    </Screen>
  );
}
