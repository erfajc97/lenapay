import { memo } from 'react';
import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import type { ReconciliationRun } from '@/shared/api/types';
import { needsAttention, runDifference } from '@/shared/helpers/reconciliation';
import { formatDay } from '@/shared/i18n/dates';
import { formatMoney } from '@/shared/money/format';
import { AppText } from '@/shared/ui/AppText';
import { PressableCard } from '@/shared/ui/Card';
import { Chip, CurrencyChip } from '@/shared/ui/Chip';

/** One day × currency: matched and missing counts, the difference highlighted (R-REC-2). */
export const RunCard = memo(function RunCard({ run, onPress }: { run: ReconciliationRun; onPress: (run: ReconciliationRun) => void }) {
  const { t } = useTranslation();
  const attention = needsAttention(run);
  const difference = runDifference(run);
  return (
    <PressableCard onPress={() => onPress(run)} accessibilityLabel={`${formatDay(run.day)} ${run.currency}`} className={`mb-3 gap-3 ${attention ? 'border-danger' : ''}`}>
      <View className="flex-row items-center justify-between">
        <AppText variant="bodyStrong">
          {formatDay(run.day)}
        </AppText>
        <View className="flex-row items-center gap-2">
          {run.status === 'completed' ? null : <Chip label={t(`reconciliation.status.${run.status}`)} tone={run.status === 'failed' ? 'danger' : 'warning'} />}
          <CurrencyChip currency={run.currency} />
        </View>
      </View>
      <View className="flex-row gap-2">
        <Chip label={t('reconciliation.matched', { count: run.matched })} tone="success" />
        {run.missingInLedger > 0 ? <Chip label={t('reconciliation.missingInLedger', { count: run.missingInLedger })} tone="danger" /> : null}
        {run.missingAtPsp > 0 ? <Chip label={t('reconciliation.missingAtPsp', { count: run.missingAtPsp })} tone="danger" /> : null}
      </View>
      <View className={`flex-row items-center justify-between rounded-chip px-3 py-2 ${attention ? 'bg-danger-bg' : 'bg-success-bg'}`}>
        <AppText variant="label">{attention ? t('reconciliation.difference') : t('reconciliation.balanced')}</AppText>
        <AppText variant="bodyStrong" tone={attention ? 'danger' : 'success'}>
          {formatMoney(difference, run.currency, { signed: difference > 0 })}
        </AppText>
      </View>
    </PressableCard>
  );
});
