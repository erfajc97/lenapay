import { Pressable, View } from 'react-native';

import { AppText } from '@/shared/ui/AppText';
import { Icon, type IconName } from '@/shared/ui/Icon';

/** Big selectable card for "Empresa" / "Persona". */
export function KindCard({ icon, title, body, selected, disabled, onPress }: { icon: IconName; title: string; body: string; selected: boolean; disabled?: boolean; onPress: () => void }) {
  return (
    <Pressable
      onPress={onPress}
      disabled={disabled}
      accessibilityRole="radio"
      accessibilityLabel={title}
      accessibilityHint={body}
      accessibilityState={{ selected, checked: selected, disabled: Boolean(disabled) }}
      className={`flex-row items-center gap-3 rounded-card border p-4 ${selected ? 'border-primary bg-primary-soft' : 'border-border-subtle bg-bg-surface'} ${disabled ? 'opacity-50' : ''}`}
    >
      <View className="h-12 w-12 items-center justify-center rounded-full bg-primary-bg">
        <Icon name={icon} size={24} tone="primary" />
      </View>
      <View className="flex-1 gap-1">
        <AppText variant="bodyStrong">{title}</AppText>
        <AppText variant="label" tone="secondary">
          {body}
        </AppText>
      </View>
      <Icon name={selected ? 'radio-button-on' : 'radio-button-off'} size={22} tone={selected ? 'primary' : 'text-muted'} />
    </Pressable>
  );
}
