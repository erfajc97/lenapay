import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import type { Account } from '@/shared/api/types';
import { formatMoney } from '@/shared/money/format';
import { AppText } from '@/shared/ui/AppText';
import { Card } from '@/shared/ui/Card';
import { ProposalRibbon } from '@/shared/ui/ProposalRibbon';
import { Divider, SectionTitle } from '@/shared/ui/Rows';

/** Example limit of the proposal (not stored, not sent): it only illustrates the field. */
const EXAMPLE_PER_ACCOUNT_MAX = 10_000_000_00;

/**
 * "Límites por cuenta (Propuesta)": per-account limits on top of the real policy. Pure UI: nothing
 * is saved or sent (no endpoint exists yet).
 */
export function AccountLimitsProposal({ accounts }: { accounts: Account[] }) {
  const { t } = useTranslation();
  return (
    <View className="gap-3">
      <SectionTitle title={t('proposal.accountLimits.title')} />
      <ProposalRibbon body={t('proposal.accountLimits.body')} />
      <Card className="py-1">
        {accounts.map((account, index) => (
          <View key={account.id}>
            {index > 0 ? <Divider /> : null}
            <View className="flex-row items-center gap-3 py-3">
              <View className="flex-1 gap-0.5">
                <AppText variant="bodyStrong" numberOfLines={1}>
                  {account.name}
                </AppText>
                <AppText variant="caption" tone="muted">
                  {t('proposal.maxPerApproval')}
                </AppText>
              </View>
              <AppText variant="bodyStrong" tone="disabled">
                {formatMoney(EXAMPLE_PER_ACCOUNT_MAX, 'ARS')}
              </AppText>
            </View>
          </View>
        ))}
      </Card>
      <AppText variant="caption" tone="muted">
        {t('proposal.accountLimits.footnote')}
      </AppText>
    </View>
  );
}
