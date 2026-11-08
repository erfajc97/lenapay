import { router, useFocusEffect } from 'expo-router';
import { useCallback, useState } from 'react';
import { BackHandler } from 'react-native';

import type { Currency } from '@/shared/api/types';
import { defaultAccountId } from '@/shared/helpers/orgs';
import { moneyActionBlocker } from '@/shared/helpers/permissions';
import { parseAmountInput } from '@/shared/money/parse';
import { useAccounts } from '@/shared/queries/accounts';
import { useActiveOrg } from '@/shared/queries/orgs';
import { useMe } from '@/shared/queries/session';

import { transferBlocker } from '../helpers/transferRules';
import { balanceAfter, canContinue, filterAccounts, nextStep, previousStep, stepNumber, TRANSFER_STEPS, type TransferStep } from '../helpers/transferWizard';
import { useCreateTransfer } from '../mutations/useCreateTransfer';

type ListStep = Exclude<TransferStep, 'amount'>;

/**
 * Mover as a wizard (R-ACC-4): Desde → Hacia → Monto, then the result. Every choice survives
 * going back and forth; the money rules (transferRules) gate the final confirmation.
 */
export function useTransfer() {
  const { org } = useActiveOrg();
  const accounts = useAccounts(org?.id);
  const open = (accounts.data ?? []).filter((account) => !account.archived);
  const me = useMe();
  const [step, setStep] = useState<TransferStep>('from');
  const [pickedFromId, setPickedFromId] = useState<string | null>(null);
  // Mover starts from the default account (R-ID-5) until the person picks another.
  const fromId = pickedFromId ?? defaultAccountId(open, org?.id, me.data?.preferences);
  const [toId, setToId] = useState<string | null>(null);
  const [currency, setCurrency] = useState<Currency>('ARS');
  const [queries, setQueries] = useState<Record<ListStep, string>>({ from: '', to: '' });
  const [expanded, setExpanded] = useState<Record<ListStep, boolean>>({ from: false, to: false });
  const [amountText, setAmountText] = useState('');
  const [concept, setConcept] = useState('');
  const [submitted, setSubmitted] = useState(false);
  const create = useCreateTransfer(org?.id);

  const from = open.find((account) => account.id === fromId) ?? null;
  const to = open.find((account) => account.id === toId) ?? null;
  const blocker = transferBlocker({ from, to, currency, amountText });
  const listFor = (list: ListStep) =>
    filterAccounts({ accounts: open, query: queries[list], excludeId: list === 'to' ? fromId : null, selectedId: list === 'from' ? fromId : toId, expanded: expanded[list] });

  const back = useCallback(() => {
    const previous = previousStep(step);
    if (previous) setStep(previous);
    else router.back();
  }, [step]);

  // Android hardware back goes one step back too (ME11); from Desde it leaves Mover as usual.
  useFocusEffect(
    useCallback(() => {
      const subscription = BackHandler.addEventListener('hardwareBackPress', () => {
        if (previousStep(step) === null) return false;
        back();
        return true;
      });
      return () => subscription.remove();
    }, [step, back]),
  );

  const goNext = () => {
    const next = nextStep(step);
    if (next) setStep(next);
  };

  return {
    orgBlocker: org ? moneyActionBlocker(org) : null,
    accounts,
    step,
    stepNumber: stepNumber(step),
    stepCount: TRANSFER_STEPS.length,
    back,
    canContinue: canContinue({ step, fromId, toId }),
    next: goNext,
    currency,
    setCurrency,
    query: (list: ListStep) => queries[list],
    setQuery: (list: ListStep, text: string) => setQueries((current) => ({ ...current, [list]: text })),
    list: listFor,
    expand: (list: ListStep) => setExpanded((current) => ({ ...current, [list]: true })),
    fromId,
    toId,
    from,
    to,
    selectFrom: (accountId: string) => {
      setPickedFromId(accountId);
      // The destination can never be the new source.
      if (toId === accountId) setToId(null);
      setStep('to');
    },
    selectTo: (accountId: string) => {
      setToId(accountId);
      setStep('amount');
    },
    amountText,
    setAmountText,
    concept,
    setConcept,
    balanceAfter: balanceAfter({ from, currency, amountText }),
    blocker: submitted ? blocker : null,
    submit: () => {
      setSubmitted(true);
      const parsed = parseAmountInput(amountText);
      if (blocker !== null || !from || !to || parsed.kind !== 'ok') return;
      create.mutate({ fromAccountId: from.id, toAccountId: to.id, currency, amount: parsed.minor, concept: concept.trim() });
    },
    pending: create.isPending,
    result: create.data ?? null,
    close: () => router.back(),
  };
}
