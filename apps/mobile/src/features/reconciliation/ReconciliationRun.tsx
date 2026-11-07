import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { formatDay } from '@/shared/i18n/dates';
import { formatMoney } from '@/shared/money/format';
import { AppText } from '@/shared/ui/AppText';
import { Card } from '@/shared/ui/Card';
import { Chip, type ChipTone } from '@/shared/ui/Chip';
import { Divider, Field, SectionTitle } from '@/shared/ui/Rows';
import { Screen } from '@/shared/ui/Screen';
import { EmptyState, ErrorState, SkeletonList } from '@/shared/ui/States';

import { runDifference } from './helpers/reconRules';
import { useReconciliationRun } from './hooks/useReconciliation';

const ITEM_TONE: Record<string, ChipTone> = { matched: 'success', missing_in_ledger: 'danger', missing_at_psp: 'danger' };

export function ReconciliationRun() {
  const { t } = useTranslation();
  const { runs, run } = useReconciliationRun();

  if (runs.isError) {
    return (
      <Screen back>
        <ErrorState error={runs.error} onRetry={() => void runs.refetch()} />
      </Screen>
    );
  }
  if (!run) {
    return <Screen back>{runs.isLoading ? <SkeletonList rows={4} /> : <EmptyState icon="help-circle-outline" title={t('reconciliation.notFound')} />}</Screen>;
  }

  const difference = runDifference(run);
  const unmatched = run.items.filter((item) => item.status !== 'matched');
  const matched = run.items.filter((item) => item.status === 'matched');
  return (
    <Screen title={`${formatDay(run.day)} · ${run.currency}`} back>
      <Card className="gap-1">
        <Field label={t('reconciliation.pspTotal')} value={formatMoney(run.pspTotal, run.currency)} />
        <Field label={t('reconciliation.ledgerTotal')} value={formatMoney(run.ledgerTotal, run.currency)} />
        <Divider />
        <View className="flex-row items-center justify-between py-2">
          <AppText variant="bodyStrong">{t('reconciliation.difference')}</AppText>
          <AppText variant="subheading" tone={difference === 0 ? 'success' : 'danger'}>
            {formatMoney(difference, run.currency, { signed: difference > 0 })}
          </AppText>
        </View>
      </Card>
      {[
        { title: t('reconciliation.unmatched'), items: unmatched },
        { title: t('reconciliation.matchedTitle'), items: matched },
      ].map((section) =>
        section.items.length > 0 ? (
          <View key={section.title} className="gap-2">
            <SectionTitle title={section.title} />
            <Card className="gap-3">
              {section.items.map((item) => (
                <View key={item.id} className="gap-1">
                  <View className="flex-row items-center justify-between">
                    <Chip label={t(`reconciliation.item.${item.status}`)} tone={ITEM_TONE[item.status] ?? 'neutral'} />
                    <AppText variant="bodyStrong">{formatMoney(item.amount, run.currency, { signed: item.amount > 0 })}</AppText>
                  </View>
                  <AppText variant="label">{item.description}</AppText>
                  <AppText variant="monoSmall" tone="muted">
                    {item.providerTransferId}
                  </AppText>
                </View>
              ))}
            </Card>
          </View>
        ) : null,
      )}
    </Screen>
  );
}
