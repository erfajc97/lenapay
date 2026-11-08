import { useTranslation } from 'react-i18next';
import { Pressable, View } from 'react-native';

import type { MyPaymentCompany, MyPaymentCvu } from '@/shared/api/types';
import { groupCbu } from '@/shared/helpers/cbu';
import { formatDateTime } from '@/shared/i18n/dates';
import { formatMoney } from '@/shared/money/format';
import { AppText } from '@/shared/ui/AppText';
import { Card } from '@/shared/ui/Card';
import { Chip, CurrencyChip } from '@/shared/ui/Chip';
import { Icon } from '@/shared/ui/Icon';
import { IconAction } from '@/shared/ui/IconAction';
import { Avatar, Divider, LinkButton } from '@/shared/ui/Rows';

/** One of my CVUs at the company: number and alias in mono, copy and share. */
function MyCvuRow({ cvu, onCopy, onShare }: { cvu: MyPaymentCvu; onCopy: (value: string) => void; onShare: (cvu: MyPaymentCvu) => void }) {
  const { t } = useTranslation();
  return (
    <View className="gap-2">
      <View className="flex-row items-center gap-2">
        <CurrencyChip currency={cvu.currency} />
        <AppText variant="monoSmall" selectable className="flex-1" accessibilityLabel={t('payers.cvuA11y', { value: cvu.cvu.split('').join(' ') })}>
          {groupCbu(cvu.cvu)}
        </AppText>
      </View>
      {cvu.alias ? (
        <AppText variant="monoSmall" tone="secondary" selectable>
          {cvu.alias}
        </AppText>
      ) : null}
      <View className="flex-row">
        <IconAction icon="copy-outline" label={t('payers.copyCvu')} onPress={() => onCopy(cvu.cvu)} />
        {cvu.alias ? <IconAction icon="at-outline" label={t('payers.copyAlias')} onPress={() => onCopy(cvu.alias ?? '')} /> : null}
        <IconAction icon="share-outline" label={t('common.share')} onPress={() => onShare(cvu)} />
      </View>
    </View>
  );
}

/** A company I pay (R-MP-2): name, project, my CVUs, total paid per currency, last payment, ERP badge. */
export function CompanyCard({
  company,
  onOpen,
  onCopy,
  onShare,
  onToggleHidden,
  toggling,
}: {
  company: MyPaymentCompany;
  onOpen: (company: MyPaymentCompany) => void;
  onCopy: (value: string) => void;
  onShare: (company: MyPaymentCompany, cvu: MyPaymentCvu) => void;
  onToggleHidden: (company: MyPaymentCompany) => void;
  toggling: boolean;
}) {
  const { t } = useTranslation();
  const totals = company.totalPaid.map((total) => formatMoney(total.amount, total.currency)).join(' · ');
  return (
    <Card className={`gap-3 ${company.hidden ? 'opacity-70' : ''}`}>
      <Pressable
        onPress={() => onOpen(company)}
        accessibilityRole="button"
        accessibilityLabel={t('myPayments.openCompany', { name: company.companyName })}
        className="flex-row items-center gap-3 rounded-chip active:bg-primary-soft"
      >
        <Avatar name={company.companyName} />
        <View className="flex-1 gap-1">
          <AppText variant="bodyStrong" numberOfLines={2}>
            {company.companyName}
          </AppText>
          <AppText variant="caption" tone="muted" numberOfLines={1}>
            {company.projectName ?? t('myPayments.mainAccount')}
          </AppText>
          <View className="flex-row flex-wrap gap-1.5">
            {company.erpLinked ? <Chip label={t('myPayments.erpBadge')} tone="brand" /> : null}
            {company.hidden ? <Chip label={t('myPayments.hiddenBadge')} /> : null}
          </View>
        </View>
        <Icon name="chevron-forward" size={18} tone="text-muted" />
      </Pressable>
      <View className="flex-row gap-3">
        <View className="flex-1 gap-0.5">
          <AppText variant="caption" tone="muted">
            {t('myPayments.totalPaid')}
          </AppText>
          <AppText variant="bodyStrong">{totals === '' ? t('myPayments.nothingPaid') : totals}</AppText>
        </View>
        <View className="flex-1 gap-0.5">
          <AppText variant="caption" tone="muted">
            {t('myPayments.lastPayment')}
          </AppText>
          <AppText variant="bodyStrong">{company.lastPaymentAt ? formatDateTime(company.lastPaymentAt) : t('myPayments.never')}</AppText>
        </View>
      </View>
      <Divider />
      <AppText variant="caption" tone="muted">
        {t('myPayments.myCvus', { count: company.cvus.length })}
      </AppText>
      {company.cvus.map((cvu) => (
        <MyCvuRow key={cvu.cvu} cvu={cvu} onCopy={onCopy} onShare={(item) => onShare(company, item)} />
      ))}
      <View className="flex-row justify-end">
        <LinkButton label={toggling ? t('common.loading') : company.hidden ? t('myPayments.show') : t('myPayments.hide')} onPress={() => onToggleHidden(company)} />
      </View>
    </Card>
  );
}
