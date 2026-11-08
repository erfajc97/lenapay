import { router } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { useIsMockApi } from '@/shared/api';
import { AppText } from '@/shared/ui/AppText';
import { AccountPicker } from '@/shared/ui/AccountPicker';
import { Button } from '@/shared/ui/Button';
import { Card } from '@/shared/ui/Card';
import { Money } from '@/shared/ui/Money';
import { isInstantLenapay } from '@/shared/helpers/rail';
import { RailBadge } from '@/shared/ui/RailBadge';
import { Divider, Field, LinkButton, Notice } from '@/shared/ui/Rows';
import { Screen } from '@/shared/ui/Screen';
import { Segmented } from '@/shared/ui/Segmented';
import { ErrorState, Skeleton, SkeletonList } from '@/shared/ui/States';
import { TextField } from '@/shared/ui/TextField';

import { DestinationCard } from './components/DestinationCard';
import { PayoutResult } from './components/PayoutResult';
import { usePayout } from './hooks/usePayout';

/** Pagar: destination → amount → quote → confirm with step-up → result (M5 lane). */
export function Payout() {
  const { t } = useTranslation();
  const flow = usePayout();
  const isMock = useIsMockApi();

  if (flow.blocker) {
    return (
      <Screen title={t('payout.title')} back>
        <Notice tone="warning" icon="lock-closed-outline" title={t(`blockers.${flow.blocker}`)} />
      </Screen>
    );
  }

  if (flow.step === 'result' && flow.result) {
    const payout = flow.result;
    return (
      <Screen
        footer={
          <View className="gap-3">
            <Button label={t('payout.done')} onPress={flow.close} />
            {payout.status === 'pending_approval' ? <Button label={t('payout.seeApprovals')} variant="ghost" onPress={() => router.replace('/approvals')} /> : null}
          </View>
        }
      >
        <PayoutResult payout={payout} />
      </Screen>
    );
  }

  if (flow.step === 'destination') {
    const resolved = flow.destination.data;
    return (
      <Screen
        title={t('payout.title')}
        back
        onBack={flow.back}
        footer={
          resolved ? (
            <Button label={t('common.continue')} onPress={flow.toDetails} />
          ) : (
            <Button label={t('payout.resolve')} onPress={flow.resolve} disabled={!flow.canResolve} loading={flow.destination.isFetching} />
          )
        }
      >
        <TextField
          label={t('payout.destinationLabel')}
          value={flow.destinationInput}
          onChangeText={flow.setDestinationInput}
          placeholder={t('payout.destinationPlaceholder')}
          hint={t(`payout.destinationHint.${flow.destinationKind}`)}
          autoCapitalize="none"
          autoCorrect={false}
          autoComplete="off"
          mono
          returnKeyType="search"
          onSubmitEditing={flow.canResolve ? flow.resolve : undefined}
          autoFocus
        />
        {isMock ? <Notice tone="info" icon="flask-outline" title={t('demo.title')} body={t('demo.destinations')} /> : null}
        {flow.destination.isLoading ? <SkeletonList rows={1} /> : null}
        {flow.destination.isError ? <ErrorState error={flow.destination.error} onRetry={() => void flow.destination.refetch()} /> : null}
        {resolved ? <DestinationCard destination={resolved} /> : null}
      </Screen>
    );
  }

  if (flow.step === 'details') {
    return (
      <Screen title={t('payout.amountTitle')} back onBack={flow.back} footer={<Button label={t('payout.toReview')} onPress={flow.toReview} />}>
        <Segmented
          label={t('fields.currency')}
          value={flow.currency}
          onChange={flow.setCurrency}
          options={[
            { value: 'ARS', label: t('currency.ARS') },
            { value: 'USD', label: t('currency.USD') },
          ]}
        />
        <TextField
          label={t('fields.amount')}
          value={flow.amountText}
          onChangeText={flow.setAmountText}
          placeholder={t('fields.amountPlaceholder')}
          keyboardType="decimal-pad"
          inputMode="decimal"
          autoComplete="off"
          large
          error={flow.errors?.amount ? t(flow.errors.amount) : null}
          returnKeyType="next"
        />
        {flow.accounts.isLoading ? <SkeletonList rows={2} /> : null}
        <AccountPicker label={t('payout.fromAccount')} accounts={flow.openAccounts} selectedId={flow.selectedAccountId} currency={flow.currency} onSelect={flow.setAccountId} />
        <TextField
          label={t('fields.concept')}
          value={flow.concept}
          onChangeText={flow.setConcept}
          placeholder={t('payout.conceptPlaceholder')}
          error={flow.errors?.concept ? t(flow.errors.concept) : null}
          maxLength={80}
          autoComplete="off"
          returnKeyType="done"
        />
        {flow.attachmentName ? (
          <Card className="flex-row items-center justify-between">
            <AppText variant="label" numberOfLines={1} className="flex-1">
              {flow.attachmentName}
            </AppText>
            <LinkButton label={t('common.remove')} onPress={flow.removeAttachment} />
          </Card>
        ) : (
          <Button label={t('payout.attach')} variant="secondary" icon="attach-outline" onPress={() => void flow.pickAttachment()} compact />
        )}
      </Screen>
    );
  }

  const quote = flow.quote.data;
  return (
    <Screen
      title={t('payout.reviewTitle')}
      back
      onBack={flow.back}
      footer={<Button label={t('payout.confirm')} icon="finger-print" onPress={flow.confirm} loading={flow.confirming} disabled={!quote || flow.blockedByBalance !== null} />}
    >
      {flow.destination.data ? <DestinationCard destination={flow.destination.data} /> : null}
      {isInstantLenapay(quote) ? <RailBadge rail="internal" long /> : null}
      {flow.quote.isError ? <ErrorState error={flow.quote.error} onRetry={() => void flow.quote.refetch()} /> : null}
      <Card className="gap-1">
        {quote ? (
          <>
            <View className="flex-row items-center justify-between py-1">
              <AppText tone="secondary">{t('payout.amount')}</AppText>
              <Money minor={quote.amount} currency={quote.currency} />
            </View>
            <View className="flex-row items-center justify-between py-1">
              <AppText tone="secondary">{t('payout.fee')}</AppText>
              <Money minor={quote.fee} currency={quote.currency} />
            </View>
            <Divider />
            <View className="flex-row items-center justify-between py-1">
              <AppText variant="bodyStrong">{t('payout.total')}</AppText>
              <Money minor={quote.total} currency={quote.currency} variant="subheading" />
            </View>
            <View className="flex-row items-center justify-between py-1">
              <AppText variant="caption" tone="muted">
                {t('payout.availableAfter')}
              </AppText>
              <Money minor={quote.availableAfter} currency={quote.currency} variant="caption" tone={flow.blockedByBalance ? 'danger' : 'muted'} />
            </View>
          </>
        ) : (
          <View className="gap-3 py-2">
            <Skeleton className="h-5 w-full" />
            <Skeleton className="h-5 w-full" />
            <Skeleton className="h-6 w-full" />
          </View>
        )}
      </Card>
      {flow.blockedByBalance ? <Notice tone="danger" icon="alert-circle-outline" title={t('errors.ledger_insufficient_funds')} /> : null}
      <Card className="gap-1">
        <Field label={t('fields.concept')} value={flow.concept.trim()} />
        {flow.attachmentName ? <Field label={t('fields.attachment')} value={flow.attachmentName} /> : null}
      </Card>
      {flow.org?.kind === 'company' ? (
        <AppText variant="caption" tone="muted">
          {t('payout.approvalNote')}
        </AppText>
      ) : null}
    </Screen>
  );
}
