import { Pressable, View } from 'react-native';

import { AppText, type TextTone } from './AppText';
import { elevation } from './elevation';

/** `surface` on cards and pages; `hero` on the primary gradient (white text, translucent track). */
type SegmentedTone = 'surface' | 'hero';

const TRACK: Record<SegmentedTone, string> = { surface: 'bg-border-subtle', hero: 'bg-hero-tile' };
const THUMB: Record<SegmentedTone, string> = { surface: `bg-bg-surface ${elevation('sm')}`, hero: `bg-on-primary ${elevation('sm')}` };
const TEXT: Record<SegmentedTone, { selected: TextTone; idle: TextTone }> = {
  surface: { selected: 'primary', idle: 'muted' },
  hero: { selected: 'onCurrency', idle: 'onPrimary' },
};

/** Two-to-four option switch (currency, kind). Selection is exposed to screen readers (M7). */
export function Segmented<T extends string>({
  options,
  value,
  onChange,
  label,
  tone = 'surface',
}: {
  options: { value: T; label: string }[];
  value: T;
  onChange: (value: T) => void;
  label: string;
  tone?: SegmentedTone;
}) {
  return (
    <View className={`flex-row rounded-button p-1 ${TRACK[tone]}`} accessibilityRole="radiogroup" accessibilityLabel={label}>
      {options.map((option) => {
        const selected = option.value === value;
        return (
          <Pressable
            key={option.value}
            onPress={() => onChange(option.value)}
            accessibilityRole="radio"
            accessibilityLabel={option.label}
            accessibilityState={{ selected, checked: selected }}
            className={`min-h-10 flex-1 items-center justify-center rounded-button ${selected ? THUMB[tone] : ''}`}
          >
            <AppText variant="label" tone={selected ? TEXT[tone].selected : TEXT[tone].idle}>
              {option.label}
            </AppText>
          </Pressable>
        );
      })}
    </View>
  );
}
