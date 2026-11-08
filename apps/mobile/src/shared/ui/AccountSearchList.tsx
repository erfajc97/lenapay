import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import type { Account, Currency } from '@/shared/api/types';

import { AccountPicker } from './AccountPicker';
import { AppText } from './AppText';
import { LinkButton } from './Rows';
import { TextField } from './TextField';

/**
 * A searchable account list for wizard steps: a name search, the accounts the caller already
 * filtered (a few by default) and "Ver todas" when more are hidden.
 */
export function AccountSearchList({
  label,
  query,
  onQueryChange,
  accounts,
  hiddenCount,
  onShowAll,
  selectedId,
  currency,
  onSelect,
}: {
  label: string;
  query: string;
  onQueryChange: (text: string) => void;
  accounts: Account[];
  hiddenCount: number;
  onShowAll: () => void;
  selectedId: string | null;
  currency: Currency;
  onSelect: (accountId: string) => void;
}) {
  const { t } = useTranslation();
  return (
    <View className="gap-3">
      <TextField
        label={t('accounts.searchLabel')}
        value={query}
        onChangeText={onQueryChange}
        placeholder={t('accounts.searchPlaceholder')}
        autoCapitalize="none"
        autoCorrect={false}
        autoComplete="off"
        returnKeyType="search"
      />
      {accounts.length > 0 ? (
        <AccountPicker label={label} accounts={accounts} selectedId={selectedId} currency={currency} onSelect={onSelect} />
      ) : (
        <AppText tone="secondary">{query.trim() ? t('accounts.noMatches', { query: query.trim() }) : t('accounts.noOtherAccounts')}</AppText>
      )}
      {hiddenCount > 0 ? (
        <View className="items-center">
          <LinkButton label={t('accounts.showAll', { count: hiddenCount })} onPress={onShowAll} />
        </View>
      ) : null}
    </View>
  );
}
