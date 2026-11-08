import { useTranslation } from 'react-i18next';
import { Pressable, ScrollView, View } from 'react-native';

import type { MovementFilterDraft } from '@/shared/stores/useMovementFiltersStore';
import { AppText } from '@/shared/ui/AppText';
import { Icon } from '@/shared/ui/Icon';

import { formatDayInput } from '@/shared/helpers/days';
import type { FilterKey } from '../hooks/useMovementsScreen';
import { FilterPill } from './FilterPill';

/** What each pill shows: its name while empty, the chosen value once set. */
function useFilterLabels(draft: MovementFilterDraft): Record<FilterKey, string | null> {
  const { t } = useTranslation();
  const customRange = draft.customFrom && draft.customTo ? `${formatDayInput(draft.customFrom).slice(0, 5)} – ${formatDayInput(draft.customTo).slice(0, 5)}` : null;
  return {
    type: draft.type ? t(`movements.type.${draft.type}`) : null,
    status: draft.status ? t(`status.movement.${draft.status}`) : null,
    currency: draft.currency ? t(`currency.${draft.currency}`) : null,
    date: draft.range === 'all' ? null : draft.range === 'custom' ? customRange : t(`movements.range.${draft.range}`),
  };
}

const KEYS: FilterKey[] = ['type', 'status', 'currency', 'date'];

/**
 * Sticky filter bar under the header (R-MOV-1): one pill per filter (Tipo, Estado, Moneda,
 * Fecha) that opens its sheet, and "Limpiar" while any filter is active.
 */
export function MovementFilters({ draft, activeCount, onOpen, onReset }: { draft: MovementFilterDraft; activeCount: number; onOpen: (key: FilterKey) => void; onReset: () => void }) {
  const { t } = useTranslation();
  const values = useFilterLabels(draft);
  return (
    <View className="flex-row items-center border-b border-border-subtle bg-bg-page pb-3">
      <ScrollView horizontal showsHorizontalScrollIndicator={false} className="flex-1" contentContainerClassName="items-center gap-2 px-4">
        {KEYS.map((key) => (
          <FilterPill key={key} name={t(`movements.filter.${key}`)} value={values[key]} onPress={() => onOpen(key)} />
        ))}
      </ScrollView>
      {/* Outside the scroll row, so "Limpiar" is always visible however many pills are set. */}
      {activeCount > 0 ? (
        <Pressable
          onPress={onReset}
          accessibilityRole="button"
          accessibilityLabel={t('movements.clearFilters', { count: activeCount })}
          hitSlop={8}
          className="min-h-9 flex-row items-center gap-1 border-l border-border-subtle pl-3 pr-4"
        >
          <Icon name="close-circle" size={18} tone="primary" />
          <AppText variant="label" tone="brand">
            {t('movements.filter.clear')}
          </AppText>
        </Pressable>
      ) : null}
    </View>
  );
}
