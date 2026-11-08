import { useState } from 'react';

import type { ApprovalMode, Currency } from '@/shared/api/types';
import { canEditApprovalPolicy } from '@/shared/helpers/permissions';
import { parseAmountInput } from '@/shared/money/parse';
import { useAccounts } from '@/shared/queries/accounts';
import { useActiveOrg, useApprovalPolicy } from '@/shared/queries/orgs';

import { formFromPolicy, type PolicyForm, policyFormResult } from '../helpers/policyForm';
import { useUpdateApprovalPolicy } from '../mutations/useUpdateApprovalPolicy';

/**
 * Settings → Política de aprobación (R-OUT-7): the owner edits it (step-up on save), everyone
 * else reads it. The draft starts from the server policy and is dropped once saved.
 */
export function useApprovalPolicyScreen() {
  const { org } = useActiveOrg();
  const policy = useApprovalPolicy(org?.kind === 'company' ? org.id : undefined);
  const accounts = useAccounts(org?.kind === 'company' ? org.id : undefined);
  const [draft, setDraft] = useState<PolicyForm | null>(null);
  const [showErrors, setShowErrors] = useState(false);
  const form = draft ?? (policy.data ? formFromPolicy(policy.data) : null);
  const result = form ? policyFormResult(form) : null;
  const update = useUpdateApprovalPolicy(org?.id, () => {
    setDraft(null);
    setShowErrors(false);
  });
  const edit = (patch: Partial<PolicyForm>) => {
    if (form) setDraft({ ...form, ...patch });
  };

  return {
    org,
    policy,
    openAccounts: (accounts.data ?? []).filter((account) => !account.archived),
    canEdit: org ? canEditApprovalPolicy(org.role) : false,
    form,
    errors: showErrors ? (result?.errors ?? null) : null,
    /** The threshold the owner is typing, once it reads as a valid amount (shown formatted). */
    preview: (currency: Currency): number | null => {
      const parsed = parseAmountInput(form?.[currency] ?? '');
      return parsed.kind === 'ok' && parsed.minor > 0 ? parsed.minor : null;
    },
    setMode: (mode: ApprovalMode) => edit({ mode }),
    setThreshold: (currency: Currency, text: string) => edit({ [currency]: text }),
    setAllowSelf: (allowOwnerSelfApproval: boolean) => edit({ allowOwnerSelfApproval }),
    dirty: draft !== null,
    save: () => {
      setShowErrors(true);
      if (result?.body) update.mutate(result.body);
    },
    saving: update.isPending,
  };
}
