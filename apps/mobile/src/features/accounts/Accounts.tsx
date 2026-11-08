import { FlashList } from '@shopify/flash-list';
import { useTranslation } from 'react-i18next';
import { Pressable, View } from 'react-native';

import { usePullToRefresh } from '@/shared/hooks/usePullToRefresh';
import { Button } from '@/shared/ui/Button';
import { Icon } from '@/shared/ui/Icon';
import { LinkButton } from '@/shared/ui/Rows';
import { Screen } from '@/shared/ui/Screen';
import { LIST_CONTENT_STYLE } from '@/shared/ui/listStyle';
import { Sheet } from '@/shared/ui/Sheet';
import { EmptyState, ErrorState, SkeletonList } from '@/shared/ui/States';
import { TextField } from '@/shared/ui/TextField';

import { AccountCard } from './components/AccountCard';
import { useAccountsScreen } from './hooks/useAccounts';

export function Accounts() {
  const { t } = useTranslation();
  const screen = useAccountsScreen();
  const pull = usePullToRefresh(screen.accounts.refetch);
  const addButton = screen.canCreate ? (
    <Pressable onPress={screen.openSheet} accessibilityRole="button" accessibilityLabel={t('accounts.new')} hitSlop={8} className="h-11 w-11 items-center justify-center rounded-full bg-primary-bg">
      <Icon name="add" size={24} tone="primary" />
    </Pressable>
  ) : null;

  return (
    <Screen title={t('accounts.title')} inTabs scroll={false} headerRight={addButton}>
      {screen.accounts.isLoading ? (
        <View className="px-4">
          <SkeletonList rows={3} />
        </View>
      ) : screen.accounts.isError ? (
        <ErrorState error={screen.accounts.error} onRetry={() => void screen.accounts.refetch()} />
      ) : (
        <FlashList
          data={screen.visible}
          keyExtractor={(account) => account.id}
          renderItem={({ item }) => <AccountCard account={item} onPress={screen.open} />}
          contentContainerStyle={LIST_CONTENT_STYLE}
          onRefresh={pull.onRefresh}
          refreshing={pull.refreshing}
          ListEmptyComponent={<EmptyState icon="wallet-outline" title={t('accounts.emptyTitle')} body={t('accounts.emptyBody')} />}
          ListFooterComponent={
            screen.archivedCount > 0 ? (
              <View className="items-center">
                <LinkButton label={screen.showArchived ? t('accounts.hideArchived') : t('accounts.showArchived', { count: screen.archivedCount })} onPress={screen.toggleArchived} />
              </View>
            ) : null
          }
        />
      )}
      <Sheet visible={screen.sheetOpen} title={t('accounts.new')} onClose={screen.closeSheet}>
        <TextField
          label={t('fields.accountName')}
          value={screen.name}
          onChangeText={screen.setName}
          error={screen.nameError ? t(screen.nameError) : null}
          hint={t('accounts.nameHint')}
          autoCapitalize="words"
          autoComplete="off"
          returnKeyType="done"
          onSubmitEditing={screen.submit}
          autoFocus
        />
        <Button label={t('accounts.create')} onPress={screen.submit} loading={screen.creating} />
      </Sheet>
    </Screen>
  );
}
