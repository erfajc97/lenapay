import type { Account, UpdatePreferencesRequest } from '@/shared/api/types';

/** The default org + account to store: an account only when it is one of that org's open accounts. */
export function defaultAccountChoice(
  choice: { orgId: string | null; accountId: string | null },
  orgAccounts: Pick<Account, 'id'>[],
): Required<Pick<UpdatePreferencesRequest, 'defaultOrgId' | 'defaultAccountId'>> {
  if (choice.orgId === null) return { defaultOrgId: null, defaultAccountId: null };
  const account = orgAccounts.find((item) => item.id === choice.accountId);
  return { defaultOrgId: choice.orgId, defaultAccountId: account ? account.id : null };
}
