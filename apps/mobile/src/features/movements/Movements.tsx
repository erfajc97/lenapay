import { FlashList } from '@shopify/flash-list';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, View } from 'react-native';

import { LIST_CONTENT_STYLE } from '@/shared/ui/listStyle';
import { MovementRow } from '@/shared/ui/MovementRow';
import { Divider } from '@/shared/ui/Rows';
import { Screen } from '@/shared/ui/Screen';
import { EmptyState, ErrorState, SkeletonList } from '@/shared/ui/States';

import { MovementFilters } from './components/MovementFilters';
import { useMovementsScreen } from './hooks/useMovementsScreen';

/** All movements of the org, cursor-paginated (R-MOV-1, R-NF-1). */
export function Movements() {
  const { t } = useTranslation();
  const screen = useMovementsScreen();
  return (
    <Screen title={t('movements.title.screen')} inTabs scroll={false}>
      <MovementFilters draft={screen.draft} setFilter={screen.setFilter} activeCount={screen.activeCount} onReset={screen.reset} />
      {screen.movements.isLoading ? (
        <View className="px-4">
          <SkeletonList rows={6} />
        </View>
      ) : screen.movements.isError ? (
        <ErrorState error={screen.movements.error} onRetry={() => void screen.movements.refetch()} />
      ) : (
        <FlashList
          data={screen.items}
          keyExtractor={(movement) => movement.id}
          renderItem={({ item }) => <MovementRow movement={item} onPress={screen.open} />}
          ItemSeparatorComponent={Divider}
          contentContainerStyle={LIST_CONTENT_STYLE}
          onEndReached={screen.loadMore}
          onEndReachedThreshold={0.4}
          onRefresh={() => void screen.movements.refetch()}
          refreshing={screen.movements.isRefetching && !screen.movements.isFetchingNextPage}
          ListFooterComponent={screen.movements.isFetchingNextPage ? <ActivityIndicator colorClassName="accent-primary" /> : null}
          ListEmptyComponent={
            <EmptyState
              icon="receipt-outline"
              title={screen.activeCount > 0 ? t('movements.noResults') : t('movements.emptyTitle')}
              actionLabel={screen.activeCount > 0 ? t('movements.clearFilters', { count: screen.activeCount }) : undefined}
              onAction={screen.activeCount > 0 ? screen.reset : undefined}
            />
          }
        />
      )}
    </Screen>
  );
}
