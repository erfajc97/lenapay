import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { AppText } from './AppText';

/** "Paso 2 de 3 · Hacia" with one bar per step; the step is in the text, never colour alone (M7). */
export function StepProgress({ current, labels }: { current: number; labels: string[] }) {
  const { t } = useTranslation();
  const label = labels[current - 1] ?? '';
  return (
    <View className="gap-2" accessible accessibilityRole="progressbar" accessibilityLabel={t('common.stepOf', { current, total: labels.length, label })}>
      <View className="w-full flex-row gap-1.5">
        {labels.map((name, index) => (
          <View key={name} className={`h-1.5 min-w-4 flex-1 rounded-full ${index < current ? 'bg-primary' : 'bg-border-subtle'}`} />
        ))}
      </View>
      <AppText variant="caption" tone="secondary">
        {t('common.stepOf', { current, total: labels.length, label })}
      </AppText>
    </View>
  );
}
