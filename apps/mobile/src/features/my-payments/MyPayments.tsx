import { useTranslation } from 'react-i18next';
import { Switch, View } from 'react-native';

import { AppText } from '@/shared/ui/AppText';
import { Card } from '@/shared/ui/Card';
import { LinkButton, Notice } from '@/shared/ui/Rows';
import { Screen } from '@/shared/ui/Screen';
import { EmptyState, ErrorState, SkeletonList } from '@/shared/ui/States';

import { CompanyCard } from './components/CompanyCard';
import { useMyPayments } from './hooks/useMyPayments';

/** "Mis pagos": the companies that registered me as a payer, and what I paid them (R-MP-2). */
export function MyPayments() {
  const { t } = useTranslation();
  const screen = useMyPayments();
  const { companies } = screen;

  if (screen.me.data && !screen.verified) {
    return (
      <Screen title={t('myPayments.title')} back>
        <Notice
          tone="info"
          icon="shield-checkmark-outline"
          title={t('myPayments.notVerifiedTitle')}
          body={t('myPayments.notVerifiedBody')}
          action={screen.canVerify ? <LinkButton label={t('myPayments.verify')} onPress={screen.openVerification} /> : undefined}
        />
      </Screen>
    );
  }

  return (
    <Screen title={t('myPayments.title')} back onRefresh={companies.refetch}>
      <AppText tone="secondary">{t('myPayments.intro')}</AppText>
      <Card className="flex-row items-center gap-3 py-2">
        <AppText variant="label" className="flex-1">
          {t('myPayments.showHidden')}
        </AppText>
        <Switch value={screen.showHidden} onValueChange={screen.toggleShowHidden} accessibilityLabel={t('myPayments.showHidden')} trackColorOnClassName="accent-primary" />
      </Card>
      {companies.isLoading || screen.me.isLoading ? <SkeletonList rows={2} /> : null}
      {companies.isError ? <ErrorState error={companies.error} onRetry={() => void companies.refetch()} /> : null}
      {companies.isSuccess && companies.data.length === 0 ? (
        <EmptyState icon="business-outline" title={t('myPayments.emptyTitle')} body={t('myPayments.emptyBody')} />
      ) : null}
      <View className="gap-4">
        {companies.data?.map((company) => (
          <CompanyCard
            key={company.linkId}
            company={company}
            onOpen={screen.open}
            onCopy={screen.copy}
            onShare={screen.share}
            onToggleHidden={screen.toggleHidden}
            toggling={screen.togglingId === company.linkId}
          />
        ))}
      </View>
    </Screen>
  );
}
