import type { Account, UpdatePreferencesRequest } from '@/shared/api/types';

/** Contract cap of `fullName` after trimming. */
const MAX_FULL_NAME = 120;

/** What PATCH /me gets: the trimmed name, or null to clear it (blank is never sent as ""). */
export function fullNameUpdate(text: string): { value: string | null; error: 'validation.fullNameLength' | null } {
  const trimmed = text.trim();
  if (trimmed.length > MAX_FULL_NAME) return { value: null, error: 'validation.fullNameLength' };
  return { value: trimmed === '' ? null : trimmed, error: null };
}

/** The default org + account to store: an account only when it is one of that org's open accounts. */
export function defaultAccountChoice(
  choice: { orgId: string | null; accountId: string | null },
  orgAccounts: Pick<Account, 'id'>[],
): Required<Pick<UpdatePreferencesRequest, 'defaultOrgId' | 'defaultAccountId'>> {
  if (choice.orgId === null) return { defaultOrgId: null, defaultAccountId: null };
  const account = orgAccounts.find((item) => item.id === choice.accountId);
  return { defaultOrgId: choice.orgId, defaultAccountId: account ? account.id : null };
}
