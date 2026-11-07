import Ionicons from '@expo/vector-icons/Ionicons';
import type { ColorValue } from 'react-native';

import type { IconName } from './Icon';

/** Tab bar icon: the colour comes from the navigator (already a theme token). */
export function TabIcon({ name, color, focused }: { name: IconName; color: ColorValue; focused: boolean }) {
  const outline = `${String(name)}-outline` as IconName;
  return <Ionicons name={focused ? name : outline} size={24} color={typeof color === 'string' ? color : undefined} />;
}
