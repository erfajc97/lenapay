import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { AppText } from '@/shared/ui/AppText';
import { Button } from '@/shared/ui/Button';
import { elevation } from '@/shared/ui/elevation';
import { Icon } from '@/shared/ui/Icon';
import { Screen } from '@/shared/ui/Screen';
import { Wordmark } from '@/shared/ui/Wordmark';

import { useForgetSession, useUnlock } from './mutations/useAuthMutations';

/** Biometric unlock of a stored session (M1 lane: biometric unlock). */
export function Unlock() {
  const { t } = useTranslation();
  const unlock = useUnlock();
  const forget = useForgetSession();
  return (
    <Screen
      footer={
        <View className="gap-3">
          <Button label={t('unlock.submit')} icon="finger-print" onPress={() => unlock.mutate()} loading={unlock.isPending} />
          <Button label={t('unlock.otherAccount')} variant="ghost" onPress={() => forget.mutate()} disabled={unlock.isPending} />
        </View>
      }
    >
      <View className="items-center gap-6 pt-16">
        <Wordmark />
        <View className={`h-24 w-24 items-center justify-center rounded-full border border-border-subtle bg-bg-surface ${elevation('lg')}`}>
          <Icon name="finger-print" size={48} tone="primary" />
        </View>
        <AppText variant="title" className="text-center">
          {t('unlock.title')}
        </AppText>
        <AppText tone="secondary" className="text-center">
          {t('unlock.body')}
        </AppText>
      </View>
    </Screen>
  );
}
