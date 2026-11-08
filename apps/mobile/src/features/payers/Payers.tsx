import { FlashList } from '@shopify/flash-list';
import { useTranslation } from 'react-i18next';
import { Pressable, View } from 'react-native';

import { usePullToRefresh } from '@/shared/hooks/usePullToRefresh';
import { Icon } from '@/shared/ui/Icon';
import { LIST_CONTENT_STYLE } from '@/shared/ui/listStyle';
import { Screen } from '@/shared/ui/Screen';
import { EmptyState, ErrorState, SkeletonList } from '@/shared/ui/States';
import { TextField } from '@/shared/ui/TextField';

import { PayerRow } from './components/PayerRow';
import { usePayersScreen } from './hooks/usePayers';

/** Payers and their CVUs (R-CVU-1): search by name, document or alias. */
export function Payers() {
  const { t } = useTranslation();
  const screen = usePayersScreen();
  const pull = usePullToRefresh(screen.payers.refetch);
  const addButton = screen.canCreate ? (
    <Pressable onPress={screen.create} accessibilityRole="button" accessibilityLabel={t('payers.new')} hitSlop={8} className="h-11 w-11 items-center justify-center rounded-full bg-primary-bg">
      <Icon name="person-add-outline" size={22} tone="primary" />
    </Pressable>
  ) : null;

  return (
    <Screen title={t('payers.title')} inTabs scroll={false} headerRight={addButton}>
      <View className="px-4 pb-2">
        <TextField
          label={t('payers.search')}
          value={screen.search}
          onChangeText={screen.setSearch}
          placeholder={t('payers.searchPlaceholder')}
          autoCapitalize="none"
          autoCorrect={false}
          autoComplete="off"
          returnKeyType="search"
        />
      </View>
      {screen.payers.isLoading ? (
        <View className="px-4">
          <SkeletonList rows={4} />
        </View>
      ) : screen.payers.isError ? (
        <ErrorState error={screen.payers.error} onRetry={() => void screen.payers.refetch()} />
      ) : (
        <FlashList
          data={screen.payers.data ?? []}
          keyExtractor={(payer) => payer.id}
          renderItem={({ item }) => <PayerRow payer={item} onPress={screen.open} />}
          contentContainerStyle={LIST_CONTENT_STYLE}
          keyboardShouldPersistTaps="handled"
          onRefresh={pull.onRefresh}
          refreshing={pull.refreshing}
          ListEmptyComponent={
            screen.searching ? (
              <EmptyState icon="search-outline" title={t('payers.noResults')} />
            ) : (
              <EmptyState
                icon="people-outline"
                title={t('payers.emptyTitle')}
                body={t('payers.emptyBody')}
                actionLabel={screen.canCreate ? t('payers.new') : undefined}
                onAction={screen.canCreate ? screen.create : undefined}
              />
            )
          }
        />
      )}
    </Screen>
  );
}
