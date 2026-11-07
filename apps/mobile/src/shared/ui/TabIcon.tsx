import Ionicons from '@expo/vector-icons/Ionicons';
import { type ColorValue, View } from 'react-native';

import { elevation, SURFACE_TONE } from './elevation';
import type { IconName } from './Icon';

/**
 * Tab bar icon: the colour comes from the navigator (already a theme token). The active tab
 * sits in a tinted pill; the label and the filled icon also mark it, not the colour alone.
 */
export function TabIcon({ name, color, focused }: { name: IconName; color: ColorValue; focused: boolean }) {
  const outline = `${String(name)}-outline` as IconName;
  return (
    <View className={`h-7 w-12 items-center justify-center rounded-full ${focused ? 'bg-primary-bg' : ''}`}>
      <Ionicons name={focused ? name : outline} size={20} color={typeof color === 'string' ? color : undefined} />
    </View>
  );
}

/** Floating tab bar surface: rounded top corners over the page colour, lifted by a shadow. */
export function TabBarBackground() {
  return (
    <View className="absolute inset-0 bg-bg-page">
      <View className={`absolute inset-0 rounded-t-sheet border-x border-t border-border-subtle ${SURFACE_TONE.bar} ${elevation('bar')}`} />
    </View>
  );
}
