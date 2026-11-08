import { useTranslation } from 'react-i18next';

import { AccountSearchList } from '@/shared/ui/AccountSearchList';
import { Button } from '@/shared/ui/Button';
import { Notice } from '@/shared/ui/Rows';
import { Screen } from '@/shared/ui/Screen';
import { Segmented } from '@/shared/ui/Segmented';
import { ErrorState, SkeletonList } from '@/shared/ui/States';
import { StepProgress } from '@/shared/ui/StepProgress';
import { TextField } from '@/shared/ui/TextField';

import { TransferResult } from './components/TransferResult';
import { TransferSummary } from './components/TransferSummary';
import { useTransfer } from './hooks/useTransfer';

/** Mover: Desde → Hacia → Monto → result; between own accounts, same currency, immediate (R-ACC-4). */
export function Transfer() {
  const { t } = useTranslation();
  const flow = useTransfer();

  if (flow.orgBlocker) {
    return (
      <Screen title={t('transfer.title')} back>
        <Notice tone="warning" icon="lock-closed-outline" title={t(`blockers.${flow.orgBlocker}`)} />
      </Screen>
    );
  }

  if (flow.result) {
    return (
      <Screen footer={<Button label={t('transfer.close')} onPress={flow.close} />}>
        <TransferResult transfer={flow.result} fromName={flow.from?.name ?? ''} toName={flow.to?.name ?? ''} />
      </Screen>
    );
  }

  const progress = <StepProgress current={flow.stepNumber} labels={[t('transfer.from'), t('transfer.to'), t('transfer.amountStep')]} />;
  const loading = flow.accounts.isLoading ? <SkeletonList rows={3} /> : null;
  const error = flow.accounts.isError ? <ErrorState error={flow.accounts.error} onRetry={() => void flow.accounts.refetch()} /> : null;

  if (flow.step === 'from' || flow.step === 'to') {
    const list = flow.list(flow.step);
    const isFrom = flow.step === 'from';
    return (
      <Screen
        title={t('transfer.title')}
        back
        onBack={flow.back}
        footer={<Button label={t('common.continue')} onPress={flow.next} disabled={!flow.canContinue} />}
      >
        {progress}
        {isFrom ? (
          <Segmented
            label={t('fields.currency')}
            value={flow.currency}
            onChange={flow.setCurrency}
            options={[
              { value: 'ARS', label: t('currency.ARS') },
              { value: 'USD', label: t('currency.USD') },
            ]}
          />
        ) : null}
        {loading}
        {error}
        {flow.accounts.data ? (
          <AccountSearchList
            label={isFrom ? t('transfer.fromQuestion') : t('transfer.toQuestion')}
            query={flow.query(flow.step)}
            onQueryChange={(text) => flow.setQuery(isFrom ? 'from' : 'to', text)}
            accounts={list.visible}
            hiddenCount={list.hiddenCount}
            onShowAll={() => flow.expand(isFrom ? 'from' : 'to')}
            selectedId={isFrom ? flow.fromId : flow.toId}
            currency={flow.currency}
            onSelect={isFrom ? flow.selectFrom : flow.selectTo}
          />
        ) : null}
      </Screen>
    );
  }

  return (
    <Screen
      title={t('transfer.title')}
      back
      onBack={flow.back}
      footer={<Button label={t('transfer.submit')} icon="finger-print" onPress={flow.submit} loading={flow.pending} />}
    >
      {progress}
      <TextField
        label={t('fields.amount')}
        value={flow.amountText}
        onChangeText={flow.setAmountText}
        placeholder={t('fields.amountPlaceholder')}
        keyboardType="decimal-pad"
        inputMode="decimal"
        autoComplete="off"
        large
        returnKeyType="next"
        autoFocus
      />
      <TextField label={t('fields.conceptOptional')} value={flow.concept} onChangeText={flow.setConcept} maxLength={80} autoComplete="off" returnKeyType="done" />
      {flow.from && flow.to ? <TransferSummary from={flow.from} to={flow.to} currency={flow.currency} balanceAfter={flow.balanceAfter} /> : null}
      {flow.blocker ? <Notice tone="danger" icon="alert-circle-outline" title={t(`transfer.blocker.${flow.blocker}`)} /> : null}
    </Screen>
  );
}
