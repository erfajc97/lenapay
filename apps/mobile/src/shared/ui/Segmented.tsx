import { Pressable, View } from 'react-native';

import { AppText } from './AppText';

/** Two-to-four option switch (currency, kind). Selection is exposed to screen readers (M7). */
export function Segmented<T extends string>({ options, value, onChange, label }: { options: { value: T; label: string }[]; value: T; onChange: (value: T) => void; label: string }) {
  return (
    <View className="flex-row rounded-button bg-border-subtle p-1" accessibilityRole="radiogroup" accessibilityLabel={label}>
      {options.map((option) => {
        const selected = option.value === value;
        return (
          <Pressable
            key={option.value}
            onPress={() => onChange(option.value)}
            accessibilityRole="radio"
            accessibilityLabel={option.label}
            accessibilityState={{ selected, checked: selected }}
            className={`min-h-10 flex-1 items-center justify-center rounded-button ${selected ? 'bg-bg-surface' : ''}`}
          >
            <AppText variant="label" tone={selected ? 'primary' : 'muted'}>
              {option.label}
            </AppText>
          </Pressable>
        );
      })}
    </View>
  );
}
