import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { isInstantLenapay } from '@/shared/helpers/rail';
import { formatMoney } from '@/shared/money/format';
import { AppText } from '@/shared/ui/AppText';
import { Button } from '@/shared/ui/Button';
import { Field, Notice } from '@/shared/ui/Rows';
import { Icon } from '@/shared/ui/Icon';
import { RailBadge } from '@/shared/ui/RailBadge';
import { Segmented } from '@/shared/ui/Segmented';
import { Sheet } from '@/shared/ui/Sheet';
import { TextField } from '@/shared/ui/TextField';

import type { useMyPaymentsCompany } from '../hooks/useMyPaymentsCompany';
import { MAX_PAY_CONCEPT } from '../helpers/myPaymentsRules';

type Screen = ReturnType<typeof useMyPaymentsCompany>;

/** "Pagar cuota" / "Pagar": amount (prefilled from the installment), confirm with step-up, result (R-MP-4). */
export function PaySheet({ screen }: { screen: Screen }) {
  const { t } = useTranslation();
  const { sheet, draft, company } = screen;
  const title =
    sheet?.step === 'result' ? t('myPayments.paidTitle') : draft?.installmentNumber ? t('myPayments.payInstallmentTitle', { number: draft.installmentNumber }) : t('myPayments.payTitle');
  return (
    <Sheet visible={sheet !== null} title={title} onClose={screen.closePay}>
      {sheet?.step === 'result' ? (
        <View className="items-center gap-3 pb-2" accessibilityLiveRegion="polite">
          <View className="h-20 w-20 items-center justify-center rounded-full bg-success-bg">
            <Icon name="checkmark-circle" size={44} tone="success" />
          </View>
          <AppText variant="heading" className="text-center">
            {formatMoney(sheet.payout.amount, sheet.payout.currency)}
          </AppText>
          <AppText tone="secondary" className="text-center">
            {t('myPayments.paidBody', { name: company?.companyName ?? sheet.payout.destination.holderName })}
          </AppText>
          {isInstantLenapay(sheet.payout) ? <RailBadge rail="internal" long /> : null}
          <AppText variant="caption" tone="muted" className="text-center">
            {sheet.payout.concept}
          </AppText>
          <Button label={t('payout.done')} onPress={screen.closePay} />
        </View>
      ) : null}
      {draft ? (
        <>
          {company ? <Field label={t('myPayments.payTo')} value={company.projectName ? `${company.companyName} · ${company.projectName}` : company.companyName} /> : null}
          {draft.installmentNumber ? (
            <Field label={t('fields.concept')} value={t('myPayments.installment', { number: draft.installmentNumber })} />
          ) : screen.currencies.length > 1 ? (
            <Segmented
              label={t('fields.currency')}
              value={draft.currency}
              onChange={screen.setCurrency}
              options={screen.currencies.map((currency) => ({ value: currency, label: t(`currency.${currency}`) }))}
            />
          ) : null}
          <TextField
            label={t('fields.amount')}
            value={draft.amountText}
            onChangeText={screen.setAmountText}
            placeholder={t('fields.amountPlaceholder')}
            keyboardType="decimal-pad"
            inputMode="decimal"
            autoComplete="off"
            large
            error={screen.errors?.amount ? t(screen.errors.amount) : null}
            hint={draft.installmentNumber ? t('myPayments.prefilledHint') : undefined}
            returnKeyType="done"
          />
          {draft.installmentNumber ? null : (
            <TextField
              label={t('myPayments.conceptOptional')}
              value={draft.concept}
              onChangeText={screen.setConcept}
              placeholder={t('myPayments.conceptPlaceholder')}
              error={screen.errors?.concept ? t(screen.errors.concept, { max: MAX_PAY_CONCEPT }) : null}
              maxLength={MAX_PAY_CONCEPT}
              autoComplete="off"
              returnKeyType="done"
            />
          )}
          <Notice tone="info" icon="person-outline" title={t('myPayments.fromPersonal')} body={t('myPayments.noApproval')} />
          <Button label={t('myPayments.confirmPay')} icon="finger-print" onPress={screen.confirm} loading={screen.paying} />
        </>
      ) : null}
    </Sheet>
  );
}
