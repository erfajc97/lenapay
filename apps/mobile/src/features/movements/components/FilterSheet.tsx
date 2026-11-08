import { useTranslation } from 'react-i18next';
import { Pressable, View } from 'react-native';

import { CURRENCIES, MOVEMENT_STATUSES, MOVEMENT_TYPES } from '@/shared/api/types';
import type { DateRange, MovementFilterDraft } from '@/shared/stores/useMovementFiltersStore';
import { AppText } from '@/shared/ui/AppText';
import { Button } from '@/shared/ui/Button';
import { Icon } from '@/shared/ui/Icon';
import { Divider } from '@/shared/ui/Rows';
import { Sheet } from '@/shared/ui/Sheet';
import { TextField } from '@/shared/ui/TextField';

import type { FilterKey, useMovementsScreen } from '../hooks/useMovementsScreen';

type Screen = ReturnType<typeof useMovementsScreen>;

const QUICK_RANGES: Exclude<DateRange, 'all' | 'custom'>[] = ['today', '7d', '30d', 'month'];

function Option({ label, selected, onPress }: { label: string; selected: boolean; onPress: () => void }) {
  return (
    <Pressable
      onPress={onPress}
      accessibilityRole="radio"
      accessibilityLabel={label}
      accessibilityState={{ selected, checked: selected }}
      className="min-h-12 flex-row items-center gap-3 rounded-chip px-1 active:bg-primary-soft"
    >
      <AppText variant={selected ? 'bodyStrong' : 'body'} className="flex-1">
        {label}
      </AppText>
      {selected ? <Icon name="checkmark" size={20} tone="primary" /> : null}
    </Pressable>
  );
}

/** "Todos" first, then each value; the current one is checked. */
function OptionList<T extends string>({ all, options, value, label, onPick }: { all: string; options: readonly T[]; value: T | null; label: (option: T) => string; onPick: (value: T | null) => void }) {
  return (
    <View accessibilityRole="radiogroup">
      <Option label={all} selected={value === null} onPress={() => onPick(null)} />
      {options.map((option) => (
        <View key={option}>
          <Divider />
          <Option label={label(option)} selected={value === option} onPress={() => onPick(option)} />
        </View>
      ))}
    </View>
  );
}

function DateOptions({ draft, screen }: { draft: MovementFilterDraft; screen: Screen }) {
  const { t } = useTranslation();
  return (
    <View className="gap-4">
      <OptionList<Exclude<DateRange, 'all' | 'custom'>>
        all={t('movements.filter.anyDate')}
        options={QUICK_RANGES}
        value={draft.range === 'all' || draft.range === 'custom' ? null : draft.range}
        label={(range) => t(`movements.range.${range}`)}
        onPick={(range) => screen.pick('range', range ?? 'all')}
      />
      <Divider />
      <Option label={t('movements.range.custom')} selected={draft.range === 'custom' || screen.custom.open} onPress={screen.custom.show} />
      {screen.custom.open ? (
        <View className="gap-3">
          <View className="flex-row gap-3">
            <View className="flex-1">
              <TextField label={t('movements.filter.from')} value={screen.custom.fromText} onChangeText={screen.custom.setFromText} placeholder={t('movements.filter.dayPlaceholder')} inputMode="numeric" autoComplete="off" maxLength={10} />
            </View>
            <View className="flex-1">
              <TextField label={t('movements.filter.to')} value={screen.custom.toText} onChangeText={screen.custom.setToText} placeholder={t('movements.filter.dayPlaceholder')} inputMode="numeric" autoComplete="off" maxLength={10} returnKeyType="done" onSubmitEditing={screen.custom.apply} />
            </View>
          </View>
          {screen.custom.error ? (
            <AppText variant="label" tone="danger" accessibilityLiveRegion="polite">
              {t(screen.custom.error)}
            </AppText>
          ) : null}
          <Button label={t('movements.filter.apply')} onPress={screen.custom.apply} />
        </View>
      ) : null}
    </View>
  );
}

/** The sheet of the pill that was tapped: one option list per filter, quick ranges for dates. */
export function FilterSheet({ screen }: { screen: Screen }) {
  const { t } = useTranslation();
  const { draft, openFilter, shownFilter } = screen;
  const titles: Record<FilterKey, string> = {
    type: t('movements.filter.type'),
    status: t('movements.filter.status'),
    currency: t('movements.filter.currency'),
    date: t('movements.filter.date'),
  };
  return (
    <Sheet visible={openFilter !== null} title={titles[shownFilter]} onClose={screen.closeFilter}>
      {shownFilter === 'type' ? (
        <OptionList all={t('movements.filter.all')} options={MOVEMENT_TYPES} value={draft.type} label={(type) => t(`movements.type.${type}`)} onPick={(type) => screen.pick('type', type)} />
      ) : null}
      {shownFilter === 'status' ? (
        <OptionList all={t('movements.filter.all')} options={MOVEMENT_STATUSES} value={draft.status} label={(status) => t(`status.movement.${status}`)} onPick={(status) => screen.pick('status', status)} />
      ) : null}
      {shownFilter === 'currency' ? (
        <OptionList all={t('movements.filter.all')} options={CURRENCIES} value={draft.currency} label={(currency) => t(`currency.${currency}`)} onPick={(currency) => screen.pick('currency', currency)} />
      ) : null}
      {shownFilter === 'date' ? <DateOptions draft={draft} screen={screen} /> : null}
    </Sheet>
  );
}
