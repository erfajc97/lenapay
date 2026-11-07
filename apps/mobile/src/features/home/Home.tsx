import { Redirect, router } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { isMockApi } from '@/shared/api';
import { Button } from '@/shared/ui/Button';
import { Card } from '@/shared/ui/Card';
import { Chip } from '@/shared/ui/Chip';
import { MovementRow } from '@/shared/ui/MovementRow';
import { Divider, LinkButton, Notice, SectionTitle } from '@/shared/ui/Rows';
import { Screen } from '@/shared/ui/Screen';
import { EmptyState, ErrorState, SkeletonList } from '@/shared/ui/States';

import { BalanceCard } from './components/BalanceCard';
import { OrgHeader } from './components/OrgHeader';
import { QuickActions } from './components/QuickActions';
import { useHome } from './hooks/useHome';

export function Home() {
  const { t } = useTranslation();
  const home = useHome();

  if (home.noOrgs) return <Redirect href="/onboarding" />;
  if (home.orgsQuery.isError) {
    return (
      <Screen inTabs>
        <ErrorState error={home.orgsQuery.error} onRetry={home.refresh} />
      </Screen>
    );
  }
  if (!home.org || !home.actions) {
    return (
      <Screen inTabs>
        <SkeletonList rows={4} />
      </Screen>
    );
  }

  const { org } = home;
  return (
    <Screen inTabs refreshing={home.refreshing} onRefresh={home.refresh}>
      <OrgHeader org={org} firstName={home.firstName} onSwitch={() => router.push('/orgs')} onProfile={() => router.push('/settings/profile')} />
      {isMockApi ? <Chip label={t('common.demoMode')} tone="warning" /> : null}
      {home.kycNotice ? (
        <Notice
          tone={home.kycNotice === 'rejected' ? 'danger' : 'warning'}
          icon="shield-outline"
          title={t(`home.kyc.${home.kycNotice}.title`)}
          body={t(`home.kyc.${home.kycNotice}.body`)}
          action={<LinkButton label={t(`home.kyc.${home.kycNotice}.action`)} onPress={() => router.push(home.kycNotice === 'draft' ? '/onboarding' : '/onboarding/status')} />}
        />
      ) : null}
      {home.balances.isError ? (
        <ErrorState error={home.balances.error} onRetry={() => void home.balances.refetch()} />
      ) : (
        <BalanceCard currency={home.currency} onCurrency={home.setCurrency} balance={home.balance} loading={home.balances.isLoading} onOpenHeld={home.openHeld} onOpenTotal={home.openTotal} />
      )}
      <QuickActions state={home.actions} onCollect={() => router.push('/payers')} onPay={() => router.push('/pay')} onMove={() => router.push('/move')} />
      {home.pendingApprovals > 0 ? (
        <Notice
          tone="info"
          icon="checkmark-done-outline"
          title={t('home.approvalsTitle', { count: home.pendingApprovals })}
          body={t('home.approvalsBody')}
          action={<LinkButton label={t('home.approvalsAction')} onPress={() => router.push('/approvals')} />}
        />
      ) : null}
      <SectionTitle title={t('home.recent')} action={home.recent.length > 0 ? <LinkButton label={t('common.seeAll')} onPress={() => router.push('/movements')} /> : undefined} />
      <Card className="py-1">
        {home.movements.isLoading ? <SkeletonList rows={3} /> : null}
        {home.movements.isError ? <ErrorState error={home.movements.error} onRetry={() => void home.movements.refetch()} /> : null}
        {(home.movements.isSuccess || org.kycStatus !== 'approved') && home.recent.length === 0 ? (
          <EmptyState icon="receipt-outline" title={t('home.emptyTitle')} body={t('home.emptyBody')} actionLabel={t('home.collect')} onAction={() => router.push('/payers')} />
        ) : null}
        {home.recent.map((movement, index) => (
          <View key={movement.id}>
            {index > 0 ? <Divider /> : null}
            <MovementRow movement={movement} onPress={home.openMovement} />
          </View>
        ))}
      </Card>
      {org.kycStatus === 'approved' && !org.erpLinked && org.role === 'admin' ? (
        <Card className="gap-3">
          <SectionTitle title={t('home.erpTitle')} />
          <Button label={t('home.erpAction')} variant="secondary" icon="link-outline" onPress={() => router.push('/erp-link')} compact />
        </Card>
      ) : null}
    </Screen>
  );
}
