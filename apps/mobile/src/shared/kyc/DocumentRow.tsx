import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { AppText } from '@/shared/ui/AppText';
import { Button } from '@/shared/ui/Button';
import { Icon } from '@/shared/ui/Icon';

/** One required document: done (check) or the action that provides it. */
export function DocumentRow({ label, done, actionLabel, onAction, busy }: { label: string; done: boolean; actionLabel: string; onAction: () => void; busy: boolean }) {
  const { t } = useTranslation();
  return (
    <View className="min-h-14 flex-row items-center gap-3 py-1">
      <Icon name={done ? 'checkmark-circle' : 'ellipse-outline'} size={22} tone={done ? 'success' : 'text-muted'} />
      <AppText className="flex-1">{label}</AppText>
      {done ? (
        <AppText variant="label" tone="success">
          {t('kyc.loaded')}
        </AppText>
      ) : (
        <Button label={actionLabel} variant="secondary" onPress={onAction} disabled={busy} compact />
      )}
    </View>
  );
}
