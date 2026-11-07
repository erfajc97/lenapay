import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import type { Balance, Currency } from '@/shared/api/types';
import { AppText } from '@/shared/ui/AppText';
import { Card } from '@/shared/ui/Card';
import { BigBalance, Money } from '@/shared/ui/Money';
import { Segmented } from '@/shared/ui/Segmented';
import { Skeleton } from '@/shared/ui/States';

/** Available balance big, held and total below (R-ACC-2: available = posted − held). */
export function BalanceCard({ currency, onCurrency, balance, loading }: { currency: Currency; onCurrency: (currency: Currency) => void; balance: Balance | null; loading: boolean }) {
  const { t } = useTranslation();
  return (
    <Card className="gap-4">
      <Segmented
        label={t('home.currency')}
        value={currency}
        onChange={onCurrency}
        options={[
          { value: 'ARS', label: t('currency.ARS') },
          { value: 'USD', label: t('currency.USD') },
        ]}
      />
      <View className="gap-1">
        <AppText variant="label" tone="muted">
          {t('home.available')}
        </AppText>
        {loading || !balance ? <Skeleton className="h-10 w-48" /> : <BigBalance minor={balance.available} currency={currency} />}
      </View>
      <View className="flex-row gap-3">
        <View className="flex-1 gap-0.5 rounded-chip bg-warning-bg px-3 py-2">
          <AppText variant="caption" tone="secondary">
            {t('home.held')}
          </AppText>
          {balance ? <Money minor={balance.held} currency={currency} variant="label" /> : <Skeleton className="h-4 w-20" />}
        </View>
        <View className="flex-1 gap-0.5 rounded-chip bg-bg-page px-3 py-2">
          <AppText variant="caption" tone="secondary">
            {t('home.posted')}
          </AppText>
          {balance ? <Money minor={balance.posted} currency={currency} variant="label" /> : <Skeleton className="h-4 w-20" />}
        </View>
      </View>
    </Card>
  );
}
