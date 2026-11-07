import * as DocumentPicker from 'expo-document-picker';
import { router } from 'expo-router';
import { useState } from 'react';

import type { Currency, Payout } from '@/shared/api/types';
import { classifyDestination } from '@/shared/helpers/cbu';
import { moneyActionBlocker } from '@/shared/helpers/permissions';
import { useAccounts } from '@/shared/queries/accounts';
import { useActiveOrg } from '@/shared/queries/orgs';
import { usePayoutQuote, useResolveDestination } from '@/shared/queries/payouts';

import { canResolve, detailsErrors, hasErrors, readAmount, reviewBlocker } from '../helpers/payoutRules';
import { useCreatePayout } from '../mutations/useCreatePayout';

type PayoutStep = 'destination' | 'details' | 'review' | 'result';

/** The payout flow: destination → amount and concept → quote → step-up → result (R-OUT-1..4). */
export function usePayout() {
  const { org } = useActiveOrg();
  const accounts = useAccounts(org?.id);
  const [step, setStep] = useState<PayoutStep>('destination');
  const [destinationInput, setDestinationInput] = useState('');
  const [submittedTo, setSubmittedTo] = useState<string | null>(null);
  const [accountId, setAccountId] = useState<string | null>(null);
  const [currency, setCurrency] = useState<Currency>('ARS');
  const [amountText, setAmountText] = useState('');
  const [concept, setConcept] = useState('');
  const [attachmentName, setAttachmentName] = useState<string | null>(null);
  const [showErrors, setShowErrors] = useState(false);
  const [result, setResult] = useState<Payout | null>(null);

  const destination = useResolveDestination(org?.id, submittedTo);
  const openAccounts = (accounts.data ?? []).filter((account) => !account.archivedAt);
  const selectedAccountId = accountId ?? openAccounts.find((account) => account.kind !== 'project')?.id ?? openAccounts[0]?.id ?? null;
  const errors = detailsErrors({ amountText, accountId: selectedAccountId, concept });
  const amount = readAmount(amountText).minor;
  const quoteRequest = step === 'review' && selectedAccountId && amount ? { accountId: selectedAccountId, currency, amount } : null;
  const quote = usePayoutQuote(org?.id, quoteRequest);
  const create = useCreatePayout(org?.id, (payout) => {
    setResult(payout);
    setStep('result');
  });

  return {
    org,
    blocker: org ? moneyActionBlocker(org) : null,
    step,
    destinationInput,
    setDestinationInput: (value: string) => {
      setDestinationInput(value);
      setSubmittedTo(null);
    },
    destinationKind: classifyDestination(destinationInput),
    canResolve: canResolve(destinationInput),
    resolve: () => setSubmittedTo(destinationInput.trim().toLowerCase()),
    destination,
    accounts,
    openAccounts,
    selectedAccountId,
    setAccountId,
    currency,
    setCurrency,
    amountText,
    setAmountText,
    concept,
    setConcept,
    attachmentName,
    pickAttachment: async () => {
      const picked = await DocumentPicker.getDocumentAsync({ type: ['application/pdf', 'image/*'], copyToCacheDirectory: false });
      if (!picked.canceled) setAttachmentName(picked.assets[0]?.name ?? null);
    },
    removeAttachment: () => setAttachmentName(null),
    errors: showErrors ? errors : null,
    toReview: () => {
      setShowErrors(true);
      if (!hasErrors(errors)) setStep('review');
    },
    quote,
    blockedByBalance: quote.data ? reviewBlocker(quote.data) : null,
    confirm: () => {
      if (!selectedAccountId || !amount || !submittedTo) return;
      create.mutate({ accountId: selectedAccountId, currency, amount, to: submittedTo, concept: concept.trim(), attachmentName: attachmentName ?? undefined });
    },
    confirming: create.isPending,
    result,
    back: () => {
      if (step === 'details') setStep('destination');
      else if (step === 'review') setStep('details');
      else router.back();
    },
    toDetails: () => setStep('details'),
    close: () => router.back(),
  };
}
