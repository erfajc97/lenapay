import { FlashList } from '@shopify/flash-list';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { ActivityIndicator, View } from 'react-native';

import { usePullToRefresh } from '@/shared/hooks/usePullToRefresh';
import { formatMoney } from '@/shared/money/format';
import { AppText } from '@/shared/ui/AppText';
import { Button } from '@/shared/ui/Button';
import { Card } from '@/shared/ui/Card';
import { Chip } from '@/shared/ui/Chip';
import { groupedRowClass, LIST_CONTENT_STYLE } from '@/shared/ui/listStyle';
import { Notice } from '@/shared/ui/Rows';
import { Screen } from '@/shared/ui/Screen';
import { Segmented } from '@/shared/ui/Segmented';
import { EmptyState, ErrorState, SkeletonList } from '@/shared/ui/States';

import { InstallmentRow } from './components/InstallmentRow';
import { PayinRow } from './components/PayinRow';
import { PaySheet } from './components/PaySheet';
import { useMyPaymentsCompany } from './hooks/useMyPaymentsCompany';

type CompanyScreen = ReturnType<typeof useMyPaymentsCompany>;

function CompanyHeader({ screen }: { screen: CompanyScreen }) {
  const { t } = useTranslation();
  const { company } = screen;
  if (!company) return null;
  const totals = company.totalPaid.map((total) => formatMoney(total.amount, total.currency)).join(' · ');
  return (
    <View className="gap-3 pb-3">
      <Card className="gap-2">
        <AppText variant="subheading">{company.companyName}</AppText>
        <AppText variant="label" tone="secondary">
          {company.projectName ?? t('myPayments.mainAccount')}
        </AppText>
        <View className="flex-row flex-wrap gap-1.5">{company.erpLinked ? <Chip label={t('myPayments.erpBadge')} tone="brand" /> : null}</View>
        <AppText variant="caption" tone="muted">
          {t('myPayments.totalPaid')}
        </AppText>
        <AppText variant="bodyStrong">{totals === '' ? t('myPayments.nothingPaid') : totals}</AppText>
      </Card>
      {screen.currencies.length > 0 ? <Button label={t('myPayments.pay')} icon="arrow-up" variant="secondary" onPress={() => screen.openPay(null)} /> : null}
      <Segmented
        label={t('myPayments.tabs')}
        value={screen.tab}
        onChange={screen.setTab}
        options={[
          { value: 'installments', label: t('myPayments.tabInstallments') },
          { value: 'payins', label: t('myPayments.tabPayins') },
        ]}
      />
      {screen.tab === 'installments' && screen.installmentsView.overdueCount > 0 ? (
        <Notice tone="danger" icon="alert-circle-outline" title={t('myPayments.overdueTitle', { count: screen.installmentsView.overdueCount })} body={t('myPayments.overdueBody')} />
      ) : null}
    </View>
  );
}

interface ListProps {
  screen: CompanyScreen;
  onRefresh: () => void;
  refreshing: boolean;
}

/** The empty slot of a list: skeleton while loading, error with retry, else the empty state. */
function ListState({ loading, error, onRetry, empty }: { loading: boolean; error: unknown; onRetry: () => void; empty: ReactNode }) {
  if (loading) return <SkeletonList rows={4} />;
  if (error) return <ErrorState error={error} onRetry={onRetry} />;
  return empty;
}

function Installments({ screen, onRefresh, refreshing }: ListProps) {
  const { t } = useTranslation();
  const { installments } = screen;
  const items = screen.installmentsView.items;
  return (
    <FlashList
      data={items}
      keyExtractor={(item) => String(item.number)}
      renderItem={({ item, index }) => (
        <View className={groupedRowClass(index, items.length)}>
          <InstallmentRow installment={item} onPay={screen.openPay} />
        </View>
      )}
      contentContainerStyle={LIST_CONTENT_STYLE}
      onRefresh={onRefresh}
      refreshing={refreshing}
      ListHeaderComponent={<CompanyHeader screen={screen} />}
      ListEmptyComponent={
        <ListState
          loading={installments.isLoading}
          error={installments.error}
          onRetry={() => void installments.refetch()}
          empty={
            installments.data?.erpLinked ? (
              <EmptyState icon="calendar-outline" title={t('myPayments.noInstallmentsTitle')} />
            ) : (
              <EmptyState icon="git-merge-outline" title={t('myPayments.notErpTitle')} body={t('myPayments.notErpBody')} />
            )
          }
        />
      }
    />
  );
}

function Payins({ screen, onRefresh, refreshing }: ListProps) {
  const { t } = useTranslation();
  const { payins, payinItems } = screen;
  return (
    <FlashList
      data={payinItems}
      keyExtractor={(item) => item.id}
      renderItem={({ item, index }) => (
        <View className={groupedRowClass(index, payinItems.length)}>
          <PayinRow payin={item} />
        </View>
      )}
      contentContainerStyle={LIST_CONTENT_STYLE}
      onEndReached={screen.loadMorePayins}
      onEndReachedThreshold={0.4}
      onRefresh={onRefresh}
      refreshing={refreshing}
      ListFooterComponent={payins.isFetchingNextPage ? <ActivityIndicator colorClassName="accent-primary" /> : null}
      ListHeaderComponent={<CompanyHeader screen={screen} />}
      ListEmptyComponent={
        <ListState
          loading={payins.isLoading}
          error={payins.error}
          onRetry={() => void payins.refetch()}
          empty={<EmptyState icon="receipt-outline" title={t('myPayments.noPayinsTitle')} body={t('myPayments.noPayinsBody')} />}
        />
      }
    />
  );
}

/** A company I pay: Cuotas (installments, ERP) and Pagos (my history), and the pay sheet. */
export function MyPaymentsCompany() {
  const { t } = useTranslation();
  const screen = useMyPaymentsCompany();
  const pull = usePullToRefresh(screen.refresh);

  if (screen.companies.isError) {
    return (
      <Screen back>
        <ErrorState error={screen.companies.error} onRetry={() => void screen.companies.refetch()} />
      </Screen>
    );
  }
  if (screen.notFound || !screen.linkId) {
    return (
      <Screen back>
        <EmptyState icon="business-outline" title={t('errors.payer_link_not_found')} />
      </Screen>
    );
  }
  if (!screen.company) {
    return (
      <Screen back>
        <SkeletonList rows={3} />
      </Screen>
    );
  }
  return (
    <Screen title={screen.company.companyName} back scroll={false}>
      {screen.tab === 'installments' ? (
        <Installments screen={screen} onRefresh={pull.onRefresh} refreshing={pull.refreshing} />
      ) : (
        <Payins screen={screen} onRefresh={pull.onRefresh} refreshing={pull.refreshing} />
      )}
      <PaySheet screen={screen} />
    </Screen>
  );
}
