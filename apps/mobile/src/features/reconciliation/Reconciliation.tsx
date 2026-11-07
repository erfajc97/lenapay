import { FlashList } from '@shopify/flash-list';
import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { LIST_CONTENT_STYLE } from '@/shared/ui/listStyle';
import { Notice } from '@/shared/ui/Rows';
import { Screen } from '@/shared/ui/Screen';
import { EmptyState, ErrorState, SkeletonList } from '@/shared/ui/States';

import { RunCard } from './components/RunCard';
import { useReconciliation } from './hooks/useReconciliation';

/** Reconciliation runs of the master account per currency and day (R-REC-1, R-REC-2). */
export function Reconciliation() {
  const { t } = useTranslation();
  const screen = useReconciliation();

  if (!screen.allowed) {
    return (
      <Screen title={t('reconciliation.title')} back>
        <Notice tone="warning" icon="lock-closed-outline" title={t('blockers.role')} />
      </Screen>
    );
  }

  return (
    <Screen title={t('reconciliation.title')} back scroll={false}>
      {screen.runs.isLoading ? (
        <View className="px-4">
          <SkeletonList rows={4} />
        </View>
      ) : screen.runs.isError ? (
        <ErrorState error={screen.runs.error} onRetry={() => void screen.runs.refetch()} />
      ) : (
        <FlashList
          data={screen.runs.data ?? []}
          keyExtractor={(run) => run.id}
          renderItem={({ item }) => <RunCard run={item} onPress={screen.open} />}
          contentContainerStyle={LIST_CONTENT_STYLE}
          onRefresh={() => void screen.runs.refetch()}
          refreshing={screen.runs.isRefetching}
          ListEmptyComponent={<EmptyState icon="git-compare-outline" title={t('reconciliation.emptyTitle')} body={t('reconciliation.emptyBody')} />}
        />
      )}
    </Screen>
  );
}
