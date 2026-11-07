import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { formatCuit } from '@/shared/helpers/cuit';
import { MOVEMENT_STATUS_TONE, PAYOUT_STATUS_TONE } from '@/shared/helpers/status';
import { formatDateTime } from '@/shared/i18n/dates';
import { formatMoney } from '@/shared/money/format';
import { AppText } from '@/shared/ui/AppText';
import { Button } from '@/shared/ui/Button';
import { Card } from '@/shared/ui/Card';
import { Chip } from '@/shared/ui/Chip';
import { Icon } from '@/shared/ui/Icon';
import { Field, SectionTitle } from '@/shared/ui/Rows';
import { Screen } from '@/shared/ui/Screen';
import { EmptyState, SkeletonList } from '@/shared/ui/States';

import { useMovementDetail } from './hooks/useMovementDetail';

/** Movement detail with the payout timeline and the proof PDF (R-MOV-1, R-OUT-6). */
export function MovementDetail() {
  const { t } = useTranslation();
  const detail = useMovementDetail();
  const { movement } = detail;

  if (!movement) {
    return (
      <Screen back>
        <EmptyState icon="help-circle-outline" title={t('movements.notFound')} />
      </Screen>
    );
  }

  const incoming = movement.direction === 'in';
  const payout = detail.payout.data;
  return (
    <Screen
      title={t(`movements.type.${movement.type}`)}
      back
      footer={detail.canShareProof ? <Button label={t('movements.shareProof')} icon="document-text-outline" onPress={detail.shareProof} loading={detail.sharing} /> : undefined}
    >
      <Card className="items-center gap-2 py-6">
        <AppText variant="title" tone={incoming ? 'success' : 'primary'}>
          {formatMoney(incoming ? movement.amount : -movement.amount, movement.currency, { signed: incoming })}
        </AppText>
        <AppText tone="secondary">{t(`movements.title.${movement.type}.${movement.direction}`, { name: movement.counterparty })}</AppText>
        <Chip label={t(`status.movement.${movement.status}`)} tone={MOVEMENT_STATUS_TONE[movement.status]} />
      </Card>
      <Card className="gap-1">
        <Field label={t('movements.date')} value={formatDateTime(movement.createdAt)} />
        <Field label={t('movements.account')} value={movement.accountName} />
        <Field label={incoming ? t('movements.from') : t('movements.to')} value={movement.counterparty} />
        {movement.counterpartyDoc ? <Field label={t('fields.cuit')} value={formatCuit(movement.counterpartyDoc)} mono /> : null}
        {movement.fee > 0 ? <Field label={t('payout.fee')} value={formatMoney(movement.fee, movement.currency)} /> : null}
        {movement.concept ? <Field label={t('fields.concept')} value={movement.concept} /> : null}
        <Field label={t('movements.reference')} value={movement.refId} mono />
      </Card>
      {detail.payin.data ? (
        <Card className="gap-1">
          <Field label={t('movements.gross')} value={formatMoney(detail.payin.data.amount, detail.payin.data.currency)} />
          <Field label={t('payout.fee')} value={formatMoney(detail.payin.data.fee, detail.payin.data.currency)} />
          <Field label={t('movements.net')} value={formatMoney(detail.payin.data.net, detail.payin.data.currency)} />
        </Card>
      ) : null}
      {movement.type === 'payout' ? (
        <>
          <SectionTitle title={t('movements.timeline')} />
          <Card className="gap-3">
            {detail.payout.isLoading ? <SkeletonList rows={2} /> : null}
            {payout?.events.map((event, index) => (
              <View key={`${event.status}-${event.at}`} className="flex-row items-center gap-3">
                <Icon name={index === payout.events.length - 1 ? 'radio-button-on' : 'checkmark-circle'} size={20} tone={index === payout.events.length - 1 ? 'primary' : 'success'} />
                <View className="flex-1">
                  <Chip label={t(`status.payout.${event.status}`)} tone={PAYOUT_STATUS_TONE[event.status]} />
                </View>
                <AppText variant="caption" tone="muted">
                  {formatDateTime(event.at)}
                </AppText>
              </View>
            ))}
          </Card>
        </>
      ) : null}
    </Screen>
  );
}
