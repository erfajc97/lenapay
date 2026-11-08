import { Pressable, View } from 'react-native';

import { AppText } from './AppText';
import { elevation, pressedScale } from './elevation';
import { Icon, type IconName } from './Icon';

/** A round icon button with its caption (copy, share, QR on CVU cards); 44 pt target (M7). */
export function IconAction({ icon, label, onPress }: { icon: IconName; label: string; onPress: () => void }) {
  return (
    <Pressable onPress={onPress} accessibilityRole="button" accessibilityLabel={label} style={pressedScale} className="min-h-11 flex-1 items-center justify-center gap-1.5 py-1">
      <View className={`h-11 w-11 items-center justify-center rounded-full border border-border-subtle bg-primary-soft ${elevation('sm')}`}>
        <Icon name={icon} size={20} tone="primary" />
      </View>
      <AppText variant="caption" tone="brand" className="text-center">
        {label}
      </AppText>
    </Pressable>
  );
}
