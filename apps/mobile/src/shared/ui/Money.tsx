import { View } from 'react-native';

import type { Currency } from '@/shared/api/types';
import { formatMoney, splitMoney } from '@/shared/money/format';

import { AppText, type TextTone, type TextVariant } from './AppText';

/** An amount from minor units, formatted es-AR (brand.md). */
export function Money({
  minor,
  currency,
  variant = 'bodyStrong',
  tone = 'primary',
  signed,
}: {
  minor: number;
  currency: Currency;
  variant?: TextVariant;
  tone?: TextTone;
  signed?: boolean;
}) {
  return (
    <AppText variant={variant} tone={tone}>
      {formatMoney(minor, currency, { signed })}
    </AppText>
  );
}

/** The big balance: units at 32 pt bold, cents smaller, read as one value by screen readers. */
export function BigBalance({ minor, currency, tone = 'primary' }: { minor: number; currency: Currency; tone?: TextTone }) {
  const parts = splitMoney(minor, currency);
  return (
    <View className="flex-row items-baseline" accessible accessibilityLabel={formatMoney(minor, currency)}>
      <AppText variant="subheading" tone={tone} className="mr-1">
        {parts.symbol}
      </AppText>
      <AppText variant="balance" tone={tone}>
        {parts.units}
      </AppText>
      <AppText variant="subheading" tone={tone}>
        {parts.cents}
      </AppText>
    </View>
  );
}
