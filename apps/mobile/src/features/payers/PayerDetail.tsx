import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { formatCuit, formatDni } from '@/shared/helpers/cuit';
import { AppText } from '@/shared/ui/AppText';
import { Button } from '@/shared/ui/Button';
import { Card } from '@/shared/ui/Card';
import { Chip } from '@/shared/ui/Chip';
import { Avatar, Field, Notice } from '@/shared/ui/Rows';
import { Screen } from '@/shared/ui/Screen';
import { Sheet } from '@/shared/ui/Sheet';
import { ErrorState, SkeletonList } from '@/shared/ui/States';
import { TextField } from '@/shared/ui/TextField';

import { CvuCard } from './components/CvuCard';
import { CvuQr } from './components/CvuQr';
import { cvuQrPayload } from './helpers/payerRules';
import { usePayerDetail } from './hooks/usePayerDetail';

export function PayerDetail() {
  const { t } = useTranslation();
  const detail = usePayerDetail();
  const payer = detail.payer.data;

  if (detail.payer.isError) {
    return (
      <Screen back>
        <ErrorState error={detail.payer.error} onRetry={() => void detail.payer.refetch()} />
      </Screen>
    );
  }
  if (!payer) {
    return (
      <Screen back>
        <SkeletonList rows={3} />
      </Screen>
    );
  }

  const doc = payer.docType === 'CUIT' ? formatCuit(payer.docNumber) : formatDni(payer.docNumber);
  const { sheet } = detail;
  return (
    <Screen title={payer.name} back onRefresh={detail.payer.refetch}>
      <Card className="gap-2">
        <View className="flex-row items-center gap-3">
          <Avatar name={payer.name} />
          <View className="flex-1 gap-1">
            <AppText variant="subheading">{payer.name}</AppText>
            {payer.erpRef ? <Chip label={t('payers.fromErp')} tone="brand" /> : null}
          </View>
        </View>
        <Field label={payer.docType === 'CUIT' ? t('docType.CUIT') : t('docType.DNI')} value={doc} mono />
        <Field label={t('fields.project')} value={payer.accountName} />
        {payer.email ? <Field label={t('fields.email')} value={payer.email} /> : null}
      </Card>

      {payer.cvus.length === 0 ? <Notice tone="warning" icon="qr-code-outline" title={t('payers.noCvuTitle')} body={t('payers.noCvuBody')} /> : null}
      {payer.cvus.map((cvu) => (
        <CvuCard
          key={cvu.id}
          cvu={cvu}
          canEdit={detail.canEdit}
          onCopy={detail.copy}
          onShare={detail.share}
          onQr={detail.openQr}
          onAlias={detail.openAlias}
          onDisable={detail.openDisable}
        />
      ))}
      {detail.canEdit
        ? detail.missing.map((currency) => (
            <Button
              key={currency}
              label={t('payers.issueCvu', { currency: t(`currency.${currency}`) })}
              variant="secondary"
              icon="add-circle-outline"
              onPress={() => detail.issue(currency)}
              loading={detail.issuing}
            />
          ))
        : null}

      <Sheet visible={sheet?.kind === 'qr'} title={t('payers.qrTitle')} onClose={detail.closeSheet}>
        {sheet ? (
          <View className="items-center gap-3">
            <CvuQr value={cvuQrPayload(sheet.cvu)} label={t('payers.qrA11y', { name: payer.name })} />
            <AppText variant="bodyStrong">{payer.name}</AppText>
            <AppText variant="mono" selectable>
              {sheet.cvu.alias}
            </AppText>
            <AppText variant="monoSmall" tone="muted" selectable>
              {sheet.cvu.cvu}
            </AppText>
            <Button label={t('common.share')} icon="share-outline" onPress={() => detail.share(sheet.cvu)} />
          </View>
        ) : null}
      </Sheet>
      <Sheet visible={sheet?.kind === 'alias'} title={t('payers.editAlias')} onClose={detail.closeSheet}>
        <TextField
          label={t('fields.alias')}
          value={detail.alias}
          onChangeText={detail.setAlias}
          error={detail.aliasError ? t(detail.aliasError) : null}
          hint={t('payers.aliasHint')}
          autoCapitalize="none"
          autoCorrect={false}
          autoComplete="off"
          mono
          returnKeyType="done"
          onSubmitEditing={detail.saveAlias}
          autoFocus
        />
        <Button label={t('common.save')} onPress={detail.saveAlias} loading={detail.savingAlias} disabled={detail.aliasError !== null} />
      </Sheet>
      <Sheet visible={sheet?.kind === 'disable'} title={t('payers.disableTitle')} onClose={detail.closeSheet}>
        <AppText tone="secondary">{t('payers.disableBody')}</AppText>
        <Button label={t('payers.disable')} variant="danger" onPress={detail.confirmDisable} loading={detail.disabling} />
      </Sheet>
    </Screen>
  );
}
