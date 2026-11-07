import { View } from 'react-native';

import { AppText } from '@/shared/ui/AppText';

/** Labelled progress with its value exposed to screen readers. */
export function ProgressBar({ label, done, total }: { label: string; done: number; total: number }) {
  const percent = total === 0 ? 100 : Math.floor((done * 100) / total);
  return (
    <View className="gap-1" accessibilityRole="progressbar" accessibilityLabel={label} accessibilityValue={{ min: 0, max: total, now: done }}>
      <View className="flex-row justify-between">
        <AppText variant="label">{label}</AppText>
        <AppText variant="label" tone="muted">{`${done}/${total}`}</AppText>
      </View>
      <View className="h-2 overflow-hidden rounded-full bg-border-subtle">
        <View className="h-2 rounded-full bg-primary" style={{ width: `${percent}%` }} />
      </View>
    </View>
  );
}
