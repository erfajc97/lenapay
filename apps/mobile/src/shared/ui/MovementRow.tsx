import { memo } from 'react';
import { useTranslation } from 'react-i18next';
import { Pressable, View } from 'react-native';

import type { Movement } from '@/shared/api/types';
import { MOVEMENT_STATUS_TONE } from '@/shared/helpers/status';
import { formatDateTime } from '@/shared/i18n/dates';
import { formatMoney } from '@/shared/money/format';

import { AppText } from './AppText';
import { Chip } from './Chip';
import { IconBadge } from './Rows';

const ICONS = { payin: 'arrow-down', payout: 'arrow-up', transfer: 'swap-horizontal' } as const;

/** One movement row (memoised for FlashList, ME8). Credits in green with a + sign. */
export const MovementRow = memo(function MovementRow({ movement, onPress }: { movement: Movement; onPress: (movement: Movement) => void }) {
  const { t } = useTranslation();
  const incoming = movement.direction === 'in';
  const amount = formatMoney(incoming ? movement.amount : -movement.amount, movement.currency, { signed: incoming });
  const title = t(`movements.title.${movement.type}.${movement.direction}`, { name: movement.counterparty });
  return (
    <Pressable
      onPress={() => onPress(movement)}
      accessibilityRole="button"
      accessibilityLabel={`${title}, ${amount}, ${t(`status.movement.${movement.status}`)}`}
      className="min-h-16 flex-row items-center gap-3 py-2 active:opacity-70"
    >
      <IconBadge name={ICONS[movement.type]} tone={incoming ? 'success' : 'text-primary'} soft={incoming ? 'bg-success-bg' : 'bg-border-subtle'} />
      <View className="flex-1 gap-0.5">
        <AppText variant="bodyStrong" numberOfLines={1}>
          {movement.counterparty}
        </AppText>
        <AppText variant="caption" tone="muted" numberOfLines={1}>
          {`${t(`movements.kind.${movement.type}`)} · ${movement.accountName}`}
        </AppText>
        <AppText variant="caption" tone="muted" numberOfLines={1}>
          {formatDateTime(movement.createdAt)}
        </AppText>
      </View>
      <View className="items-end gap-1">
        <AppText variant="bodyStrong" tone={incoming ? 'success' : 'primary'}>
          {amount}
        </AppText>
        {movement.status !== 'settled' ? <Chip label={t(`status.movement.${movement.status}`)} tone={MOVEMENT_STATUS_TONE[movement.status]} /> : null}
      </View>
    </Pressable>
  );
});
