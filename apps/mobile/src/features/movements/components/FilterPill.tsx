import { Pressable } from 'react-native';

import { AppText } from '@/shared/ui/AppText';
import { elevation, pressedScale, SURFACE_TONE } from '@/shared/ui/elevation';
import { Icon } from '@/shared/ui/Icon';

/**
 * A filter pill: the filter's name while empty, its value (tinted) once set; the chevron says
 * a sheet opens. The state is in the label for screen readers, not only in the colour (M7).
 */
export function FilterPill({ name, value, onPress }: { name: string; value: string | null; onPress: () => void }) {
  const active = value !== null;
  return (
    <Pressable
      onPress={onPress}
      accessibilityRole="button"
      accessibilityLabel={active ? `${name}: ${value}` : name}
      accessibilityState={{ selected: active }}
      style={pressedScale}
      className={`min-h-9 flex-row items-center gap-1.5 rounded-full border pl-3.5 pr-2.5 ${
        active ? 'border-primary bg-primary-bg' : `border-border-default ${SURFACE_TONE.md} ${elevation('sm', { pressable: true })}`
      }`}
    >
      <AppText variant="label" tone={active ? 'brand' : 'secondary'} numberOfLines={1}>
        {value ?? name}
      </AppText>
      <Icon name="chevron-down" size={14} tone={active ? 'primary' : 'text-muted'} />
    </Pressable>
  );
}
