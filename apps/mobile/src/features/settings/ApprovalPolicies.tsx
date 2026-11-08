import { useTranslation } from 'react-i18next';
import { View } from 'react-native';

import { formatMoney } from '@/shared/money/format';
import { useAccounts } from '@/shared/queries/accounts';
import { useActiveOrg } from '@/shared/queries/orgs';
import { AppText } from '@/shared/ui/AppText';
import { Card } from '@/shared/ui/Card';
import { Chip } from '@/shared/ui/Chip';
import { ProposalRibbon } from '@/shared/ui/ProposalRibbon';
import { Divider, SectionTitle } from '@/shared/ui/Rows';
import { Screen } from '@/shared/ui/Screen';
import { SkeletonList } from '@/shared/ui/States';
import { TextField } from '@/shared/ui/TextField';

/** Example policy of the proposal (not stored, not sent): it only illustrates the fields. */
const EXAMPLE = { withoutApproval: 250_000_00, oneApprover: 5_000_000_00, perAccountMax: 10_000_000_00 } as const;

/**
 * "Políticas de aprobación (Propuesta)": how approval limits would be configured per org and
 * per account. Pure UI: every control is disabled and nothing is saved or sent.
 */
export function ApprovalPolicies() {
  const { t } = useTranslation();
  const { org } = useActiveOrg();
  const accounts = useAccounts(org?.id);
  const open = (accounts.data ?? []).filter((account) => !account.archived);
  return (
    <Screen title={t('proposal.policies.title')} back>
      <ProposalRibbon body={t('proposal.policies.body')} />
      <SectionTitle title={t('proposal.policies.general')} />
      <Card className="gap-4">
        <TextField
          label={t('proposal.withoutApproval')}
          value={formatMoney(EXAMPLE.withoutApproval, 'ARS')}
          editable={false}
          hint={t('proposal.withoutApprovalHint')}
          accessibilityHint={t('proposal.disabledHint')}
        />
        <TextField
          label={t('proposal.policies.oneApprover')}
          value={formatMoney(EXAMPLE.oneApprover, 'ARS')}
          editable={false}
          hint={t('proposal.policies.oneApproverHint')}
          accessibilityHint={t('proposal.disabledHint')}
        />
        <View className="flex-row flex-wrap items-center gap-2">
          <AppText variant="label" tone="secondary">
            {t('proposal.policies.above')}
          </AppText>
          <Chip label={t('proposal.policies.twoApprovers')} tone="brand" />
        </View>
      </Card>
      <SectionTitle title={t('proposal.policies.perAccount')} />
      {accounts.isLoading ? <SkeletonList rows={2} /> : null}
      <Card className="py-1">
        {open.map((account, index) => (
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
                {formatMoney(EXAMPLE.perAccountMax, 'ARS')}
              </AppText>
            </View>
          </View>
        ))}
      </Card>
      <AppText variant="caption" tone="muted">
        {t('proposal.policies.footnote')}
      </AppText>
    </Screen>
  );
}
