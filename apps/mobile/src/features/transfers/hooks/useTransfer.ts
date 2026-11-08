import { useState } from 'react';

import type { Currency } from '@/shared/api/types';
import { moneyActionBlocker } from '@/shared/helpers/permissions';
import { parseAmountInput } from '@/shared/money/parse';
import { useAccounts } from '@/shared/queries/accounts';
import { useActiveOrg } from '@/shared/queries/orgs';

import { transferBlocker } from '../helpers/transferRules';
import { useCreateTransfer } from '../mutations/useCreateTransfer';

export function useTransfer() {
  const { org } = useActiveOrg();
  const accounts = useAccounts(org?.id);
  const open = (accounts.data ?? []).filter((account) => !account.archived);
  const [fromId, setFromId] = useState<string | null>(null);
  const [toId, setToId] = useState<string | null>(null);
  const [currency, setCurrency] = useState<Currency>('ARS');
  const [amountText, setAmountText] = useState('');
  const [concept, setConcept] = useState('');
  const [submitted, setSubmitted] = useState(false);
  const create = useCreateTransfer(org?.id);

  const from = open.find((account) => account.id === fromId) ?? null;
  const to = open.find((account) => account.id === toId) ?? null;
  const blocker = transferBlocker({ from, to, currency, amountText });

  return {
    orgBlocker: org ? moneyActionBlocker(org) : null,
    accounts,
    open,
    fromId,
    setFromId,
    toId,
    setToId,
    currency,
    setCurrency,
    amountText,
    setAmountText,
    concept,
    setConcept,
    blocker: submitted ? blocker : null,
    submit: () => {
      setSubmitted(true);
      const parsed = parseAmountInput(amountText);
      if (blocker !== null || !from || !to || parsed.kind !== 'ok') return;
      create.mutate({ fromAccountId: from.id, toAccountId: to.id, currency, amount: parsed.minor, concept: concept.trim() });
    },
    pending: create.isPending,
  };
}
