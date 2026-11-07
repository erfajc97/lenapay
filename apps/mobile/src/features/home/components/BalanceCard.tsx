import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import type { Balance, Currency } from '@/shared/api/types';
import { AppText } from '@/shared/ui/AppText';
import { HeroSurface } from '@/shared/ui/HeroSurface';
import { BigBalance, Money } from '@/shared/ui/Money';
import { Segmented } from '@/shared/ui/Segmented';
import { Skeleton } from '@/shared/ui/States';

/** Available balance big, held and total below (R-ACC-2: available = posted − held). */
export function BalanceCard({ currency, onCurrency, balance, loading }: { currency: Currency; onCurrency: (currency: Currency) => void; balance: Balance | null; loading: boolean }) {
  const { t } = useTranslation();
  return (
    <HeroSurface>
      <Segmented
        label={t('home.currency')}
        value={currency}
        onChange={onCurrency}
        tone="hero"
        options={[
          { value: 'ARS', label: t('currency.ARS') },
          { value: 'USD', label: t('currency.USD') },
        ]}
      />
      <View className="gap-1">
        <AppText variant="label" tone="onPrimary">
          {t('home.available')}
        </AppText>
        {loading || !balance ? <Skeleton className="h-10 w-48" /> : <BigBalance minor={balance.available} currency={currency} tone="onPrimary" />}
      </View>
      <View className="flex-row gap-3">
        <View className="flex-1 gap-0.5 rounded-chip bg-hero-tile px-3 py-2">
          <AppText variant="caption" tone="onPrimary">
            {t('home.held')}
          </AppText>
          {balance ? <Money minor={balance.held} currency={currency} variant="label" tone="onPrimary" /> : <Skeleton className="h-4 w-20" />}
        </View>
        <View className="flex-1 gap-0.5 rounded-chip bg-hero-tile px-3 py-2">
          <AppText variant="caption" tone="onPrimary">
            {t('home.posted')}
          </AppText>
          {balance ? <Money minor={balance.posted} currency={currency} variant="label" tone="onPrimary" /> : <Skeleton className="h-4 w-20" />}
        </View>
      </View>
    </HeroSurface>
  );
}
