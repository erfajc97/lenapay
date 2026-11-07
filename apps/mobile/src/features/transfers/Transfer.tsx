import { useTranslation } from 'react-i18next';

import { AccountPicker } from '@/shared/ui/AccountPicker';
import { Button } from '@/shared/ui/Button';
import { Notice } from '@/shared/ui/Rows';
import { Screen } from '@/shared/ui/Screen';
import { Segmented } from '@/shared/ui/Segmented';
import { ErrorState, SkeletonList } from '@/shared/ui/States';
import { TextField } from '@/shared/ui/TextField';

import { useTransfer } from './hooks/useTransfer';

/** Mover: between own accounts, same currency, immediate (R-ACC-4). */
export function Transfer() {
  const { t } = useTranslation();
  const screen = useTransfer();

  if (screen.orgBlocker) {
    return (
      <Screen title={t('transfer.title')} back>
        <Notice tone="warning" icon="lock-closed-outline" title={t(`blockers.${screen.orgBlocker}`)} />
      </Screen>
    );
  }

  return (
    <Screen title={t('transfer.title')} back footer={<Button label={t('transfer.submit')} icon="finger-print" onPress={screen.submit} loading={screen.pending} />}>
      {screen.accounts.isLoading ? <SkeletonList rows={3} /> : null}
      {screen.accounts.isError ? <ErrorState error={screen.accounts.error} onRetry={() => void screen.accounts.refetch()} /> : null}
      <Segmented
        label={t('fields.currency')}
        value={screen.currency}
        onChange={screen.setCurrency}
        options={[
          { value: 'ARS', label: t('currency.ARS') },
          { value: 'USD', label: t('currency.USD') },
        ]}
      />
      <AccountPicker label={t('transfer.from')} accounts={screen.open} selectedId={screen.fromId} currency={screen.currency} onSelect={screen.setFromId} />
      <AccountPicker label={t('transfer.to')} accounts={screen.open} selectedId={screen.toId} currency={screen.currency} onSelect={screen.setToId} disabledId={screen.fromId} />
      <TextField
        label={t('fields.amount')}
        value={screen.amountText}
        onChangeText={screen.setAmountText}
        placeholder="0,00"
        keyboardType="decimal-pad"
        inputMode="decimal"
        autoComplete="off"
        large
        returnKeyType="next"
      />
      <TextField label={t('fields.conceptOptional')} value={screen.concept} onChangeText={screen.setConcept} maxLength={80} autoComplete="off" returnKeyType="done" />
      {screen.blocker ? <Notice tone="danger" icon="alert-circle-outline" title={t(`transfer.blocker.${screen.blocker}`)} /> : null}
    </Screen>
  );
}
