import { useTranslation } from 'react-i18next';
import { Switch, View } from 'react-native';

import type { Account } from '@/shared/api/types';
import { formatMoney } from '@/shared/money/format';
import { AppText } from '@/shared/ui/AppText';
import { ProposalRibbon } from '@/shared/ui/ProposalRibbon';
import { Divider, SectionTitle } from '@/shared/ui/Rows';
import { TextField } from '@/shared/ui/TextField';

/** Example limits of the proposal (not stored, not sent): they only illustrate the fields. */
const EXAMPLE_MAX_APPROVAL = 5_000_000_00;
const EXAMPLE_NO_APPROVAL = 250_000_00;

/**
 * "Permisos por cuenta (Propuesta)": how account-scoped permissions would look for one member.
 * Pure UI: every control is disabled and nothing is saved or sent (no endpoint exists yet).
 */
export function AccountPermissionsProposal({ accounts }: { accounts: Account[] }) {
  const { t } = useTranslation();
  const open = accounts.filter((account) => !account.archived);
  return (
    <View className="gap-3">
      <SectionTitle title={t('proposal.accountPermissions.title')} />
      <ProposalRibbon body={t('proposal.accountPermissions.body')} />
      <View className="rounded-card border border-border-subtle px-3 py-1">
        {open.map((account, index) => (
          <View key={account.id}>
            {index > 0 ? <Divider /> : null}
            <View className="min-h-12 flex-row items-center gap-3 py-1">
              <AppText variant="label" className="flex-1" numberOfLines={1}>
                {account.name}
              </AppText>
              <Switch
                value={account.kind !== 'main'}
                disabled
                accessibilityLabel={t('proposal.accountPermissions.accountA11y', { name: account.name })}
                accessibilityHint={t('proposal.disabledHint')}
                trackColorOnClassName="accent-primary"
              />
            </View>
          </View>
        ))}
      </View>
      <TextField
        label={t('proposal.maxPerApproval')}
        value={formatMoney(EXAMPLE_MAX_APPROVAL, 'ARS')}
        editable={false}
        hint={t('proposal.maxPerApprovalHint')}
        accessibilityHint={t('proposal.disabledHint')}
      />
      <TextField
        label={t('proposal.withoutApproval')}
        value={formatMoney(EXAMPLE_NO_APPROVAL, 'ARS')}
        editable={false}
        hint={t('proposal.withoutApprovalHint')}
        accessibilityHint={t('proposal.disabledHint')}
      />
    </View>
  );
}
