import { memo } from 'react';
import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import type { Installment } from '@/shared/api/types';
import { formatDate } from '@/shared/i18n/dates';
import { formatMoney } from '@/shared/money/format';
import { AppText } from '@/shared/ui/AppText';
import { Button } from '@/shared/ui/Button';
import { Chip } from '@/shared/ui/Chip';

import { canPayInstallment, INSTALLMENT_TONE } from '../helpers/myPaymentsRules';

/** One installment (R-MP-3): number, due date, amount, status chip, CAC/UVA label; overdue on a danger tint. */
export const InstallmentRow = memo(function InstallmentRow({ installment, onPay }: { installment: Installment; onPay: (installment: Installment) => void }) {
  const { t } = useTranslation();
  const overdue = installment.status === 'overdue';
  return (
    <View className={`gap-2 py-3 ${overdue ? '-mx-4 bg-danger-bg px-4' : ''}`}>
      <View className="flex-row items-center gap-3">
        <View className="flex-1 gap-0.5">
          <AppText variant="bodyStrong">{t('myPayments.installment', { number: installment.number })}</AppText>
          <AppText variant="caption" tone={overdue ? 'danger' : 'muted'}>
            {t('myPayments.dueDate', { date: formatDate(installment.dueDate) })}
          </AppText>
        </View>
        <View className="items-end gap-1">
          <AppText variant="bodyStrong">{formatMoney(installment.amount, installment.currency)}</AppText>
          <View className="flex-row gap-1">
            {installment.adjustmentIndex ? <Chip label={t('myPayments.adjusted', { index: installment.adjustmentIndex })} /> : null}
            <Chip label={t(`status.installment.${installment.status}`)} tone={INSTALLMENT_TONE[installment.status]} />
          </View>
        </View>
      </View>
      {canPayInstallment(installment) ? (
        <View className="flex-row justify-end">
          <Button
            label={t('myPayments.payInstallment')}
            variant={overdue ? 'primary' : 'secondary'}
            icon="arrow-up"
            onPress={() => onPay(installment)}
            compact
            accessibilityHint={t('myPayments.installment', { number: installment.number })}
          />
        </View>
      ) : null}
    </View>
  );
});
