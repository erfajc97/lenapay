import { useTranslation } from 'react-i18next';
import { Switch, View } from 'react-native';

import { CURRENCIES } from '@/shared/api/types';
import { formatDateTime } from '@/shared/i18n/dates';
import { formatMoney } from '@/shared/money/format';
import { AppText } from '@/shared/ui/AppText';
import { Button } from '@/shared/ui/Button';
import { Card } from '@/shared/ui/Card';
import { Divider, Notice } from '@/shared/ui/Rows';
import { Screen } from '@/shared/ui/Screen';
import { Segmented } from '@/shared/ui/Segmented';
import { ErrorState, SkeletonList } from '@/shared/ui/States';
import { TextField } from '@/shared/ui/TextField';

import { AccountLimitsProposal } from './components/AccountLimitsProposal';
import { useApprovalPolicyScreen } from './hooks/useApprovalPolicyScreen';

/** Política de aprobación (R-OUT-7): real policy (owner edits, others read) + per-account limits proposal. */
export function ApprovalPolicy() {
  const { t } = useTranslation();
  const screen = useApprovalPolicyScreen();
  const { form, policy } = screen;

  return (
    <Screen
      title={t('policy.title')}
      back
      onRefresh={policy.refetch}
      footer={screen.canEdit && form ? <Button label={t('policy.save')} icon="finger-print" onPress={screen.save} loading={screen.saving} disabled={!screen.dirty} /> : undefined}
    >
      <AppText tone="secondary">{t('policy.intro')}</AppText>
      {!screen.canEdit ? <Notice tone="info" icon="lock-closed-outline" title={t('policy.readOnly')} /> : null}
      {policy.isLoading ? <SkeletonList rows={3} /> : null}
      {policy.isError ? <ErrorState error={policy.error} onRetry={() => void policy.refetch()} /> : null}

      {form && policy.data ? (
        <>
          <Card className="gap-4">
            {screen.canEdit ? (
              <Segmented
                label={t('policy.modeLabel')}
                value={form.mode}
                onChange={screen.setMode}
                options={[
                  { value: 'always', label: t('policy.mode.always') },
                  { value: 'threshold', label: t('policy.mode.threshold') },
                ]}
              />
            ) : (
              <View className="gap-1">
                <AppText variant="label" tone="secondary">
                  {t('policy.modeLabel')}
                </AppText>
                <AppText variant="subheading">{t(`policy.mode.${form.mode}`)}</AppText>
              </View>
            )}
            <AppText tone="secondary">{t(form.mode === 'always' ? 'policy.alwaysBody' : 'policy.thresholdBody')}</AppText>
            {form.mode === 'threshold'
              ? CURRENCIES.map((currency) => {
                  const preview = screen.preview(currency);
                  const label = t('policy.thresholdLabel', { currency: t(`currency.${currency}`) });
                  if (!screen.canEdit) {
                    return (
                      <View key={currency} className="gap-1">
                        <AppText variant="label" tone="secondary">
                          {label}
                        </AppText>
                        <AppText variant="bodyStrong">
                          {preview === null ? t('policy.noThreshold', { currency: t(`currency.${currency}`) }) : formatMoney(preview, currency)}
                        </AppText>
                      </View>
                    );
                  }
                  const error = screen.errors?.[currency];
                  return (
                    <TextField
                      key={currency}
                      label={label}
                      value={form[currency]}
                      onChangeText={(text) => screen.setThreshold(currency, text)}
                      placeholder={t('fields.amountPlaceholder')}
                      keyboardType="decimal-pad"
                      inputMode="decimal"
                      autoComplete="off"
                      returnKeyType="done"
                      error={error ? t(error) : null}
                      hint={preview === null ? t('policy.thresholdHint') : t('policy.thresholdPreview', { amount: formatMoney(preview, currency) })}
                    />
                  );
                })
              : null}
            {screen.errors?.thresholds ? <Notice tone="danger" icon="alert-circle-outline" title={t(screen.errors.thresholds)} /> : null}
          </Card>

          <Card className="gap-2">
            <View className="flex-row items-center gap-3">
              <AppText variant="bodyStrong" className="flex-1">
                {t('policy.selfLabel')}
              </AppText>
              <Switch
                value={form.allowOwnerSelfApproval}
                onValueChange={screen.setAllowSelf}
                disabled={!screen.canEdit}
                accessibilityLabel={t('policy.selfLabel')}
                accessibilityHint={t('policy.selfBody')}
                trackColorOnClassName="accent-primary"
              />
            </View>
            <AppText variant="caption" tone="muted">
              {t('policy.selfBody')}
            </AppText>
            {policy.data.allowOwnerSelfApproval ? (
              <>
                <Divider />
                <AppText variant="caption" tone={policy.data.ownerSelfApprovalEffective ? 'success' : 'secondary'}>
                  {t(policy.data.ownerSelfApprovalEffective ? 'policy.selfEffective' : 'policy.selfBlocked')}
                </AppText>
              </>
            ) : null}
          </Card>
          {policy.data.updatedAt ? (
            <AppText variant="caption" tone="muted">
              {t('policy.updatedAt', { date: formatDateTime(policy.data.updatedAt) })}
            </AppText>
          ) : null}
        </>
      ) : null}

      {screen.openAccounts.length > 0 ? <AccountLimitsProposal accounts={screen.openAccounts} /> : null}
    </Screen>
  );
}
