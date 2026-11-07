import { memo } from 'react';
import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import type { Account } from '@/shared/api/types';
import { AppText } from '@/shared/ui/AppText';
import { PressableCard } from '@/shared/ui/Card';
import { Chip, CurrencyChip } from '@/shared/ui/Chip';
import { Icon } from '@/shared/ui/Icon';
import { Money } from '@/shared/ui/Money';
import { IconBadge } from '@/shared/ui/Rows';

/** Account with its available balance per currency (R-ACC-1). */
export const AccountCard = memo(function AccountCard({ account, onPress }: { account: Account; onPress: (account: Account) => void }) {
  const { t } = useTranslation();
  return (
    <PressableCard onPress={() => onPress(account)} accessibilityLabel={account.name} className={`mb-3 gap-3 ${account.archivedAt ? 'opacity-60' : ''}`}>
      <View className="flex-row items-center gap-3">
        <IconBadge name={account.kind === 'project' ? 'business-outline' : 'wallet-outline'} />
        <View className="flex-1 gap-1">
          <AppText variant="bodyStrong" numberOfLines={1}>
            {account.name}
          </AppText>
          <View className="flex-row gap-2">
            <Chip label={t(`accounts.kind.${account.kind}`)} tone={account.kind === 'project' ? 'brand' : 'neutral'} />
            {account.archivedAt ? <Chip label={t('accounts.archivedChip')} /> : null}
          </View>
        </View>
        <Icon name="chevron-forward" size={18} tone="text-muted" />
      </View>
      <View className="gap-2">
        {account.balances.map((balance) => (
          <View key={balance.currency} className="flex-row items-center justify-between">
            <CurrencyChip currency={balance.currency} />
            <Money minor={balance.available} currency={balance.currency} />
          </View>
        ))}
      </View>
      {account.kind === 'project' ? (
        <AppText variant="caption" tone="muted">
          {t('accounts.payersCount', { count: account.payersCount })}
        </AppText>
      ) : null}
    </PressableCard>
  );
});
