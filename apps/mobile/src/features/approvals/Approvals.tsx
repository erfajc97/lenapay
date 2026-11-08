import { FlashList } from '@shopify/flash-list';
import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { usePullToRefresh } from '@/shared/hooks/usePullToRefresh';
import { LIST_CONTENT_STYLE } from '@/shared/ui/listStyle';
import { Screen } from '@/shared/ui/Screen';
import { EmptyState, ErrorState, SkeletonList } from '@/shared/ui/States';

import { ApprovalCard } from './components/ApprovalCard';
import { useApprovalsScreen } from './hooks/useApprovals';

/** Approvals inbox (R-OUT-4). */
export function Approvals() {
  const { t } = useTranslation();
  const screen = useApprovalsScreen();
  const pull = usePullToRefresh(screen.approvals.refetch);
  return (
    <Screen title={t('approvals.title')} back scroll={false}>
      {screen.approvals.isLoading ? (
        <View className="px-4">
          <SkeletonList rows={3} />
        </View>
      ) : screen.approvals.isError ? (
        <ErrorState error={screen.approvals.error} onRetry={() => void screen.approvals.refetch()} />
      ) : (
        <FlashList
          data={screen.approvals.data ?? []}
          keyExtractor={(payout) => payout.id}
          renderItem={({ item }) => <ApprovalCard payout={item} mine={item.createdBy.userId === screen.userId} selfApproval={screen.selfApprovalAllowed} onPress={screen.open} />}
          contentContainerStyle={LIST_CONTENT_STYLE}
          onRefresh={pull.onRefresh}
          refreshing={pull.refreshing}
          ListEmptyComponent={<EmptyState icon="checkmark-done-circle-outline" title={t('approvals.emptyTitle')} body={t('approvals.emptyBody')} />}
        />
      )}
    </Screen>
  );
}
