import { useTranslation } from 'react-i18next';
import { ScrollView, View } from 'react-native';

import type { MovementFilterDraft } from '@/shared/stores/useMovementFiltersStore';
import { FilterChip } from '@/shared/ui/Chip';
import { LinkButton } from '@/shared/ui/Rows';

type SetFilter = <K extends keyof MovementFilterDraft>(key: K, value: MovementFilterDraft[K]) => void;

const TYPES = ['payin', 'payout', 'transfer'] as const;
const STATUSES = ['pending', 'settled', 'failed', 'cancelled'] as const;
const CURRENCIES = ['ARS', 'USD'] as const;
const RANGES = ['7d', '30d', '90d'] as const;

function Row({ children }: { children: React.ReactNode }) {
  return (
    <ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerClassName="gap-2 px-4">
      {children}
    </ScrollView>
  );
}

/** Type, status, currency and date filters as chip rows (R-MOV-1). Tapping a selected chip clears it. */
export function MovementFilters({ draft, setFilter, activeCount, onReset }: { draft: MovementFilterDraft; setFilter: SetFilter; activeCount: number; onReset: () => void }) {
  const { t } = useTranslation();
  return (
    <View className="gap-2 pb-2">
      <Row>
        {TYPES.map((type) => (
          <FilterChip key={type} label={t(`movements.type.${type}`)} selected={draft.type === type} onPress={() => setFilter('type', draft.type === type ? null : type)} />
        ))}
        {CURRENCIES.map((currency) => (
          <FilterChip key={currency} label={currency} selected={draft.currency === currency} onPress={() => setFilter('currency', draft.currency === currency ? null : currency)} />
        ))}
      </Row>
      <Row>
        {RANGES.map((range) => (
          <FilterChip key={range} label={t(`movements.range.${range}`)} selected={draft.range === range} onPress={() => setFilter('range', draft.range === range ? 'all' : range)} />
        ))}
        {STATUSES.map((status) => (
          <FilterChip key={status} label={t(`status.movement.${status}`)} selected={draft.status === status} onPress={() => setFilter('status', draft.status === status ? null : status)} />
        ))}
      </Row>
      {activeCount > 0 ? (
        <View className="px-4">
          <LinkButton label={t('movements.clearFilters', { count: activeCount })} onPress={onReset} />
        </View>
      ) : null}
    </View>
  );
}
