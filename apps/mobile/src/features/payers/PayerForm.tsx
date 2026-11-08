import { Controller } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { CURRENCIES } from '@/shared/api/types';
import { AppText } from '@/shared/ui/AppText';
import { Button } from '@/shared/ui/Button';
import { FilterChip } from '@/shared/ui/Chip';
import { FormField } from '@/shared/ui/FormField';
import { Notice } from '@/shared/ui/Rows';
import { Screen } from '@/shared/ui/Screen';
import { Segmented } from '@/shared/ui/Segmented';
import { ErrorState, SkeletonList } from '@/shared/ui/States';

import { usePayerForm } from './hooks/usePayerForm';

/** New payer + CVU per currency (R-CVU-1); the CVU step asks for step-up. */
export function PayerForm() {
  const { t } = useTranslation();
  const screen = usePayerForm();
  const { control, formState } = screen.form;

  return (
    <Screen title={t('payers.new')} back footer={<Button label={t('payers.createSubmit')} icon="qr-code-outline" onPress={screen.submit} loading={screen.pending} />}>
      {screen.accounts.isLoading ? <SkeletonList rows={1} /> : null}
      {screen.accounts.isError ? <ErrorState error={screen.accounts.error} onRetry={() => void screen.accounts.refetch()} /> : null}
      {screen.accounts.isSuccess && screen.projects.length === 0 ? <Notice tone="warning" icon="business-outline" title={t('payers.noProjects')} body={t('payers.noProjectsBody')} /> : null}
      <Controller
        control={control}
        name="accountId"
        render={({ field }) => (
          <View className="gap-2">
            <AppText variant="label" tone="secondary">
              {t('fields.project')}
            </AppText>
            <View className="flex-row flex-wrap gap-2">
              {screen.projects.map((account) => (
                <FilterChip key={account.id} label={account.name} selected={field.value === account.id} onPress={() => field.onChange(account.id)} />
              ))}
            </View>
            {formState.errors.accountId?.message ? (
              <AppText variant="caption" tone="danger">
                {t(formState.errors.accountId.message)}
              </AppText>
            ) : null}
          </View>
        )}
      />
      <FormField control={control} name="name" label={t('fields.payerName')} autoCapitalize="words" autoComplete="name" textContentType="name" returnKeyType="next" />
      <Controller
        control={control}
        name="docType"
        render={({ field }) => (
          <Segmented
            label={t('fields.docType')}
            value={field.value}
            onChange={field.onChange}
            options={[
              { value: 'cuit', label: t('docType.cuit') },
              { value: 'dni', label: t('docType.dni') },
            ]}
          />
        )}
      />
      <FormField control={control} name="docNumber" label={t('fields.docNumber')} keyboardType="number-pad" autoComplete="off" mono returnKeyType="next" />
      <FormField control={control} name="email" label={t('fields.emailOptional')} keyboardType="email-address" autoCapitalize="none" autoComplete="email" textContentType="emailAddress" returnKeyType="done" />
      <Controller
        control={control}
        name="currencies"
        render={({ field }) => (
          <View className="gap-2">
            <AppText variant="label" tone="secondary">
              {t('payers.issueIn')}
            </AppText>
            <View className="flex-row gap-2">
              {CURRENCIES.map((currency) => (
                <FilterChip key={currency} label={t(`currency.${currency}`)} selected={field.value.includes(currency)} onPress={() => screen.toggleCurrency(currency)} />
              ))}
            </View>
            {formState.errors.currencies?.message ? (
              <AppText variant="caption" tone="danger">
                {t(formState.errors.currencies.message)}
              </AppText>
            ) : null}
          </View>
        )}
      />
      <AppText variant="caption" tone="muted">
        {t('payers.stepUpNote')}
      </AppText>
    </Screen>
  );
}
