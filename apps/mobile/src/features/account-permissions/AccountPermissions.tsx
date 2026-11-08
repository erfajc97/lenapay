import { useTranslation } from 'react-i18next';

import { AccountPermissionsProposal } from '@/shared/ui/AccountPermissionsProposal';
import { Screen } from '@/shared/ui/Screen';
import { ErrorState, SkeletonList } from '@/shared/ui/States';

import { useAccountPermissions } from './hooks/useAccountPermissions';

/** Más → Permisos por cuenta (Propuesta): the per-account permissions mock, reachable on its own. */
export function AccountPermissions() {
  const { t } = useTranslation();
  const screen = useAccountPermissions();
  return (
    <Screen title={t('proposal.accountPermissions.title')} back>
      {screen.accounts.isLoading ? <SkeletonList rows={3} /> : null}
      {screen.accounts.isError ? <ErrorState error={screen.accounts.error} onRetry={() => void screen.accounts.refetch()} /> : null}
      {screen.accounts.data ? <AccountPermissionsProposal accounts={screen.accounts.data} /> : null}
    </Screen>
  );
}
