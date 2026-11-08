import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import type { Rail } from '@/shared/api/types';
import { RAIL_BADGE } from '@/shared/helpers/rail';

import { AppText } from './AppText';
import { Icon } from './Icon';

/**
 * How the money moves (R-MP-5): "Lenapay instantánea" (on-us) or "Banco". Icon and text,
 * never colour alone (M7). `long` is the payout-flow banner ("… · sin costo").
 */
export function RailBadge({ rail, long }: { rail: Rail; long?: boolean }) {
  const { t } = useTranslation();
  const badge = RAIL_BADGE[rail];
  if (long && rail === 'internal') {
    return (
      <View className="flex-row items-center gap-3 rounded-card bg-primary-bg p-3" accessibilityRole="text" accessibilityLiveRegion="polite">
        <Icon name={badge.icon} size={20} tone="primary" />
        <AppText variant="label" tone="brand" className="flex-1">
          {t('rail.internalLong')}
        </AppText>
      </View>
    );
  }
  return (
    <View className={`flex-row items-center gap-1 self-start rounded-full px-2 py-0.5 ${badge.background}`}>
      <Icon name={badge.icon} size={12} tone={badge.tone} />
      <AppText variant="caption" tone={rail === 'internal' ? 'brand' : 'secondary'} className="font-sans-semibold">
        {t(`rail.${rail}`)}
      </AppText>
    </View>
  );
}
