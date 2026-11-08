import { useTranslation } from 'react-i18next';
import { Pressable, View } from 'react-native';

import type { MoneyBlocker } from '@/shared/helpers/permissions';
import { AppText } from '@/shared/ui/AppText';
import { elevation, pressedScale } from '@/shared/ui/elevation';
import { Icon, type IconName } from '@/shared/ui/Icon';

import type { QuickActionsState } from '../helpers/homeRules';

function Action({ icon, label, blocker, onPress, onBlocked }: { icon: IconName; label: string; blocker: MoneyBlocker; onPress: () => void; onBlocked: (blocker: NonNullable<MoneyBlocker>) => void }) {
  const { t } = useTranslation();
  const blocked = blocker !== null;
  // A blocked action stays tappable: the tap explains why instead of doing nothing.
  return (
    <Pressable
      onPress={() => (blocker === null ? onPress() : onBlocked(blocker))}
      accessibilityRole="button"
      accessibilityLabel={label}
      accessibilityHint={blocked ? t(`blockers.${blocker}`) : undefined}
      style={pressedScale}
      className={`flex-1 items-center gap-2 ${blocked ? 'opacity-50' : ''}`}
    >
      <View className={`h-16 w-16 items-center justify-center rounded-full border border-hero-from bg-primary ${blocked ? '' : elevation('glow')}`}>
        <Icon name={icon} size={26} tone="on-primary" />
      </View>
      <AppText variant="label">{label}</AppText>
    </Pressable>
  );
}

/** Cobrar, Pagar, Mover as raised round buttons with a primary glow. A blocked action is dimmed and explains itself on tap. */
export function QuickActions({
  state,
  onCollect,
  onPay,
  onMove,
  onBlocked,
}: {
  state: QuickActionsState;
  onCollect: () => void;
  onPay: () => void;
  onMove: () => void;
  onBlocked: (blocker: NonNullable<MoneyBlocker>) => void;
}) {
  const { t } = useTranslation();
  return (
    <View className="flex-row">
      <Action icon="qr-code-outline" label={t('home.collect')} blocker={state.collect} onPress={onCollect} onBlocked={onBlocked} />
      <Action icon="arrow-up" label={t('home.pay')} blocker={state.pay} onPress={onPay} onBlocked={onBlocked} />
      <Action icon="swap-horizontal" label={t('home.move')} blocker={state.move} onPress={onMove} onBlocked={onBlocked} />
    </View>
  );
}
