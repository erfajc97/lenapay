import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { AppText } from '@/shared/ui/AppText';
import { Button } from '@/shared/ui/Button';
import { Icon } from '@/shared/ui/Icon';
import { LinkButton } from '@/shared/ui/Rows';
import { Sheet } from '@/shared/ui/Sheet';
import { TextField } from '@/shared/ui/TextField';

import { useStepUpSheet } from './useStepUpSheet';

/** Mounted once at the root; opens whenever a money mutation awaits `requestStepUp()`. */
export function StepUpSheet() {
  const { t } = useTranslation();
  const sheet = useStepUpSheet();
  return (
    <Sheet visible={sheet.open} title={t('stepUp.title')} onClose={sheet.close}>
      <View className="items-center gap-2 py-2">
        <View className="h-16 w-16 items-center justify-center rounded-full bg-primary-bg">
          <Icon name={sheet.mode === 'biometric' ? 'finger-print' : 'lock-closed-outline'} size={32} tone="primary" />
        </View>
        <AppText tone="secondary" className="text-center">
          {sheet.mode === 'biometric' ? t('stepUp.bodyBiometric') : t('stepUp.bodyPassword')}
        </AppText>
      </View>
      {sheet.mode === 'password' ? (
        <TextField
          label={t('stepUp.password')}
          value={sheet.password}
          onChangeText={sheet.setPassword}
          secureTextEntry
          autoComplete="current-password"
          textContentType="password"
          returnKeyType="done"
          onSubmitEditing={sheet.confirmWithPassword}
          autoFocus
        />
      ) : null}
      {sheet.errorText ? (
        <AppText variant="label" tone="danger" className="text-center" accessibilityLiveRegion="polite">
          {sheet.errorText}
        </AppText>
      ) : null}
      {sheet.mode === 'biometric' ? (
        <View className="gap-2">
          <Button label={t('stepUp.confirmBiometric')} icon="finger-print" onPress={sheet.confirmWithBiometrics} loading={sheet.pending} />
          <View className="items-center">
            <LinkButton label={t('stepUp.usePassword')} onPress={sheet.usePassword} />
          </View>
        </View>
      ) : (
        <Button label={t('stepUp.confirm')} onPress={sheet.confirmWithPassword} loading={sheet.pending} disabled={!sheet.canSubmitPassword} />
      )}
    </Sheet>
  );
}
