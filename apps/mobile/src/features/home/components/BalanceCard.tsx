import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Pressable, View } from 'react-native';

import type { Balance, Currency } from '@/shared/api/types';
import { formatMoney } from '@/shared/money/format';
import { AppText } from '@/shared/ui/AppText';
import { HeroSurface } from '@/shared/ui/HeroSurface';
import { Icon, type IconName } from '@/shared/ui/Icon';
import { BigBalance, Money } from '@/shared/ui/Money';
import { Segmented } from '@/shared/ui/Segmented';
import { Skeleton } from '@/shared/ui/States';

import { heldShare } from '../helpers/homeRules';

/** A row of the glass panel: icon, label (and an optional meter) and the amount; opens movements. */
function GlassRow({
  icon,
  label,
  minor,
  currency,
  hint,
  onPress,
  children,
}: {
  icon: IconName;
  label: string;
  minor: number | null;
  currency: Currency;
  hint: string;
  onPress: () => void;
  children?: ReactNode;
}) {
  return (
    <Pressable
      onPress={onPress}
      accessibilityRole="button"
      accessibilityLabel={minor === null ? label : `${label}, ${formatMoney(minor, currency)}`}
      accessibilityHint={hint}
      className="min-h-14 flex-row items-center gap-3 px-3 py-2.5 active:bg-hero-tile"
    >
      <View className="h-9 w-9 items-center justify-center rounded-full border border-hero-glass-border bg-hero-glass">
        <Icon name={icon} size={16} tone="on-primary" />
      </View>
      <View className="flex-1 gap-1.5">
        <AppText variant="label" tone="onPrimary" numberOfLines={1}>
          {label}
        </AppText>
        {children}
      </View>
      {minor === null ? <Skeleton className="h-5 w-24" /> : <Money minor={minor} currency={currency} variant="bodyStrong" tone="onPrimary" />}
      <Icon name="chevron-forward" size={16} tone="on-primary" />
    </Pressable>
  );
}

/**
 * Available balance big; held and total below in a glass panel (R-ACC-2: available = posted −
 * held). The held row shows how much of the total is not spendable yet.
 */
export function BalanceCard({
  currency,
  onCurrency,
  balance,
  loading,
  onOpenHeld,
  onOpenTotal,
}: {
  currency: Currency;
  onCurrency: (currency: Currency) => void;
  balance: Balance | null;
  loading: boolean;
  onOpenHeld: () => void;
  onOpenTotal: () => void;
}) {
  const { t } = useTranslation();
  const share = heldShare(balance);
  const percent = Math.round(share * 100);
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
      <View className="overflow-hidden rounded-card border border-hero-glass-border bg-hero-glass">
        <GlassRow icon="lock-closed" label={t('home.held')} minor={balance?.held ?? null} currency={currency} hint={t('home.heldHint')} onPress={onOpenHeld}>
          <View className="flex-row items-center gap-2">
            <View
              className="h-1 flex-1 overflow-hidden rounded-full bg-hero-track"
              accessible
              accessibilityRole="progressbar"
              accessibilityLabel={t('home.heldShare', { percent })}
              accessibilityValue={{ min: 0, max: 100, now: percent }}
            >
              <View className="h-1 rounded-full bg-on-primary" style={{ width: `${share * 100}%` }} />
            </View>
            <AppText variant="caption" tone="onPrimary">
              {t('home.heldShare', { percent })}
            </AppText>
          </View>
        </GlassRow>
        <View className="mx-3 h-px bg-hero-glass-border" />
        <GlassRow icon="layers" label={t('home.posted')} minor={balance?.posted ?? null} currency={currency} hint={t('home.postedHint')} onPress={onOpenTotal} />
      </View>
    </HeroSurface>
  );
}
