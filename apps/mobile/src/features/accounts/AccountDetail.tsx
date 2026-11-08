import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { AppText } from '@/shared/ui/AppText';
import { Button } from '@/shared/ui/Button';
import { Card } from '@/shared/ui/Card';
import { Chip, CurrencyChip } from '@/shared/ui/Chip';
import { BigBalance, Money } from '@/shared/ui/Money';
import { MovementRow } from '@/shared/ui/MovementRow';
import { Avatar, Divider, ListRow, Notice, SectionTitle } from '@/shared/ui/Rows';
import { Screen } from '@/shared/ui/Screen';
import { Sheet } from '@/shared/ui/Sheet';
import { EmptyState, ErrorState, SkeletonList } from '@/shared/ui/States';
import { TextField } from '@/shared/ui/TextField';

import { useAccountDetail } from './hooks/useAccountDetail';

export function AccountDetail() {
  const { t } = useTranslation();
  const detail = useAccountDetail();
  const { account } = detail;

  if (detail.accounts.isError) {
    return (
      <Screen back>
        <ErrorState error={detail.accounts.error} onRetry={() => void detail.accounts.refetch()} />
      </Screen>
    );
  }
  if (detail.notFound) {
    return (
      <Screen back>
        <EmptyState icon="help-circle-outline" title={t('accounts.notFound')} />
      </Screen>
    );
  }
  if (!account) {
    return (
      <Screen back>
        <SkeletonList rows={3} />
      </Screen>
    );
  }

  return (
    <Screen title={account.name} back onRefresh={detail.movements.refetch}>
      <View className="flex-row gap-2">
        <Chip label={t(`accounts.kind.${account.kind}`)} tone={account.kind === 'project' ? 'brand' : 'neutral'} />
        {account.archivedAt ? <Chip label={t('accounts.archivedChip')} /> : null}
      </View>
      {account.balances.map((balance) => (
        <Card key={balance.currency} className="gap-3">
          <View className="flex-row items-center justify-between">
            <AppText variant="label" tone="muted">
              {t('home.available')}
            </AppText>
            <CurrencyChip currency={balance.currency} />
          </View>
          <BigBalance minor={balance.available} currency={balance.currency} />
          <View className="flex-row justify-between">
            <AppText variant="caption" tone="muted">
              {t('home.held')}
            </AppText>
            <Money minor={balance.held} currency={balance.currency} variant="caption" tone="secondary" />
          </View>
        </Card>
      ))}
      {detail.canEdit && !account.archivedAt ? (
        <View className="flex-row gap-3">
          <View className="flex-1">
            <Button label={t('accounts.rename')} variant="secondary" icon="create-outline" onPress={detail.openRename} compact />
          </View>
          {account.kind === 'project' ? (
            <View className="flex-1">
              <Button label={t('accounts.archive')} variant="secondary" icon="archive-outline" onPress={detail.openArchive} compact />
            </View>
          ) : null}
        </View>
      ) : null}

      {account.kind === 'project' ? (
        <>
          <SectionTitle title={t('accounts.payers')} />
          <Card className="py-1">
            {detail.payers.isLoading ? <SkeletonList rows={2} /> : null}
            {detail.payers.isSuccess && detail.payerPreview.length === 0 ? <EmptyState icon="people-outline" title={t('accounts.noPayers')} /> : null}
            {detail.payerPreview.map((payer, index) => (
              <View key={payer.id}>
                {index > 0 ? <Divider /> : null}
                <ListRow title={payer.name} subtitle={payer.cvus.map((cvu) => cvu.alias).join(' · ')} leading={<Avatar name={payer.name} />} onPress={() => detail.openPayer(payer)} />
              </View>
            ))}
          </Card>
        </>
      ) : null}

      <SectionTitle title={t('accounts.movements')} />
      <Card className="py-1">
        {detail.movements.isLoading ? <SkeletonList rows={3} /> : null}
        {detail.movements.isError ? <ErrorState error={detail.movements.error} onRetry={() => void detail.movements.refetch()} /> : null}
        {detail.movements.isSuccess && detail.movementPreview.length === 0 ? <EmptyState icon="receipt-outline" title={t('accounts.noMovements')} /> : null}
        {detail.movementPreview.map((movement, index) => (
          <View key={movement.id}>
            {index > 0 ? <Divider /> : null}
            <MovementRow movement={movement} onPress={detail.openMovement} />
          </View>
        ))}
      </Card>

      <Sheet visible={detail.sheet === 'rename'} title={t('accounts.rename')} onClose={detail.closeSheet}>
        <TextField
          label={t('fields.accountName')}
          value={detail.name}
          onChangeText={detail.setName}
          error={detail.nameError ? t(detail.nameError) : null}
          autoCapitalize="words"
          returnKeyType="done"
          onSubmitEditing={detail.rename}
          autoFocus
        />
        <Button label={t('common.save')} onPress={detail.rename} loading={detail.saving} />
      </Sheet>
      <Sheet visible={detail.sheet === 'archive'} title={t('accounts.archiveTitle', { name: account.name })} onClose={detail.closeSheet}>
        {detail.archive ? (
          <Notice tone="warning" icon="information-circle-outline" title={t(`accounts.archiveBlocked.${detail.archive}`)} />
        ) : (
          <AppText tone="secondary">{t('accounts.archiveBody')}</AppText>
        )}
        <Button label={t('accounts.archive')} variant="danger" onPress={detail.confirmArchive} loading={detail.saving} disabled={detail.archive !== null} />
      </Sheet>
    </Screen>
  );
}
