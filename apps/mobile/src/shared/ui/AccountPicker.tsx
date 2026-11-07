import { Pressable, View } from 'react-native';

import type { Account, Currency } from '@/shared/api/types';
import { formatMoney } from '@/shared/money/format';

import { AppText } from './AppText';
import { Icon } from './Icon';

/** Pick one of the org's open accounts, showing its available balance in the chosen currency. */
export function AccountPicker({
  label,
  accounts,
  selectedId,
  currency,
  onSelect,
  disabledId,
}: {
  label: string;
  accounts: Account[];
  selectedId: string | null;
  currency: Currency;
  onSelect: (accountId: string) => void;
  disabledId?: string | null;
}) {
  return (
    <View className="gap-2" accessibilityRole="radiogroup" accessibilityLabel={label}>
      <AppText variant="label" tone="secondary">
        {label}
      </AppText>
      {accounts.map((account) => {
        const selected = account.id === selectedId;
        const disabled = account.id === disabledId;
        const available = account.balances.find((balance) => balance.currency === currency)?.available ?? 0;
        return (
          <Pressable
            key={account.id}
            onPress={() => onSelect(account.id)}
            disabled={disabled}
            accessibilityRole="radio"
            accessibilityLabel={`${account.name}, ${formatMoney(available, currency)}`}
            accessibilityState={{ selected, checked: selected, disabled }}
            className={`min-h-14 flex-row items-center gap-3 rounded-card border px-3 py-2 ${selected ? 'border-primary bg-primary-soft' : 'border-border-subtle bg-bg-surface'} ${disabled ? 'opacity-40' : ''}`}
          >
            <Icon name={selected ? 'radio-button-on' : 'radio-button-off'} size={20} tone={selected ? 'primary' : 'text-muted'} />
            <AppText variant="bodyStrong" className="flex-1" numberOfLines={1}>
              {account.name}
            </AppText>
            <AppText variant="label" tone="secondary">
              {formatMoney(available, currency)}
            </AppText>
          </Pressable>
        );
      })}
    </View>
  );
}
