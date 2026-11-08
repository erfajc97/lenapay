import { router } from 'expo-router';
import { useState } from 'react';

import { useAccounts } from '@/shared/queries/accounts';
import { useVisibleOrgs } from '@/shared/queries/orgs';
import { useMe } from '@/shared/queries/session';

import { defaultAccountChoice } from '../helpers/profileRules';
import { useSaveDefaultAccount } from '../mutations/useProfileMutations';

/**
 * Settings → Cuenta predeterminada (R-ID-5): one of my orgs, then one of its open accounts (or
 * none). The choice starts from the stored preferences and is saved with PATCH /me/preferences.
 */
export function useDefaultAccount() {
  const me = useMe();
  const { query: orgsQuery, orgs } = useVisibleOrgs();
  const stored = me.data?.preferences;
  const [choice, setChoice] = useState<{ orgId: string | null; accountId: string | null } | null>(null);
  const current = choice ?? { orgId: stored?.defaultOrgId ?? null, accountId: stored?.defaultAccountId ?? null };
  const accounts = useAccounts(current.orgId ?? undefined);
  const openAccounts = (accounts.data ?? []).filter((account) => !account.archived);
  const save = useSaveDefaultAccount(() => router.back());
  const body = defaultAccountChoice(current, openAccounts);
  return {
    me,
    orgsQuery,
    orgs: orgs ?? [],
    accounts,
    openAccounts,
    orgId: current.orgId,
    accountId: body.defaultAccountId,
    pickOrg: (orgId: string | null) => setChoice({ orgId, accountId: null }),
    pickAccount: (accountId: string) => setChoice({ orgId: current.orgId, accountId }),
    dirty: choice !== null && (body.defaultOrgId !== (stored?.defaultOrgId ?? null) || body.defaultAccountId !== (stored?.defaultAccountId ?? null)),
    save: () => save.mutate(body),
    saving: save.isPending,
  };
}
