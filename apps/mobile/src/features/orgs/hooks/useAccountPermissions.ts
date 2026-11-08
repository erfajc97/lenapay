import { useAccounts } from '@/shared/queries/accounts';
import { useActiveOrg } from '@/shared/queries/orgs';

/** The active company's accounts, for the per-account permissions proposal (nothing is saved). */
export function useAccountPermissions() {
  const { org } = useActiveOrg();
  return { accounts: useAccounts(org?.kind === 'company' ? org.id : undefined) };
}
