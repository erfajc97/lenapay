import { Pressable, View } from 'react-native';

import type { Currency } from '@/shared/api/types';

import { AppText, type TextTone } from './AppText';

export type ChipTone = 'neutral' | 'brand' | 'success' | 'warning' | 'danger';

/** Soft tinted backgrounds: the tone reads from the tint and the text, never colour alone (M7). */
const BG: Record<ChipTone, string> = {
  neutral: 'bg-bg-page border border-border-subtle',
  brand: 'bg-primary-bg',
  success: 'bg-success-bg',
  warning: 'bg-warning-bg',
  danger: 'bg-danger-bg',
};
const FG: Record<ChipTone, TextTone> = { neutral: 'secondary', brand: 'brand', success: 'success', warning: 'primary', danger: 'danger' };

/** Status always goes in a chip with text, never colour alone (M7). */
export function Chip({ label, tone = 'neutral' }: { label: string; tone?: ChipTone }) {
  return (
    <View className={`self-start rounded-full px-2.5 py-0.5 ${BG[tone]}`}>
      <AppText variant="caption" tone={FG[tone]} className="font-sans-semibold">
        {label}
      </AppText>
    </View>
  );
}

export function CurrencyChip({ currency }: { currency: Currency }) {
  return (
    <View className={`self-start rounded-full px-2.5 py-0.5 ${currency === 'ARS' ? 'bg-currency-ars' : 'bg-currency-usd'}`}>
      <AppText variant="caption" tone="onCurrency" className="font-sans-semibold">
        {currency}
      </AppText>
    </View>
  );
}

/** A selectable filter chip; the selection is exposed to screen readers, not only coloured. */
export function FilterChip({ label, selected, onPress }: { label: string; selected: boolean; onPress: () => void }) {
  return (
    <Pressable
      onPress={onPress}
      accessibilityRole="button"
      accessibilityLabel={label}
      accessibilityState={{ selected }}
      hitSlop={6}
      className={`min-h-9 justify-center rounded-full border px-3.5 ${selected ? 'border-primary bg-primary-bg' : 'border-border-default bg-bg-surface'}`}
    >
      <AppText variant="label" tone={selected ? 'brand' : 'secondary'}>
        {label}
      </AppText>
    </Pressable>
  );
}
