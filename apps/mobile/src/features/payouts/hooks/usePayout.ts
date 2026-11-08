import * as DocumentPicker from 'expo-document-picker';
import { router } from 'expo-router';
import { useState } from 'react';

import type { Currency, LocalUpload, Payout } from '@/shared/api/types';
import { classifyDestination } from '@/shared/helpers/cbu';
import { toUpload } from '@/shared/helpers/files';
import { defaultAccountId } from '@/shared/helpers/orgs';
import { moneyActionBlocker } from '@/shared/helpers/permissions';
import { useAccounts } from '@/shared/queries/accounts';
import { useActiveOrg, useApprovalPolicy } from '@/shared/queries/orgs';
import { usePayoutQuote, useResolveDestination } from '@/shared/queries/payouts';
import { useMe } from '@/shared/queries/session';

import { approvalNote, canResolve, detailsErrors, hasErrors, readAmount, reviewBlocker } from '../helpers/payoutRules';
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
  const [attachment, setAttachment] = useState<LocalUpload | null>(null);
  const [showErrors, setShowErrors] = useState(false);
  const [result, setResult] = useState<Payout | null>(null);

  const destination = useResolveDestination(org?.id, submittedTo);
  const policy = useApprovalPolicy(org?.kind === 'company' ? org.id : undefined);
  const openAccounts = (accounts.data ?? []).filter((account) => !account.archived);
  const me = useMe();
  const selectedAccountId = accountId ?? defaultAccountId(openAccounts, org?.id, me.data?.preferences) ?? openAccounts.find((account) => account.kind !== 'project')?.id ?? openAccounts[0]?.id ?? null;
  const errors = detailsErrors({ amountText, accountId: selectedAccountId, concept });
  const amount = readAmount(amountText).minor;
  const quoteRequest = step === 'review' && selectedAccountId && amount && submittedTo ? { accountId: selectedAccountId, currency, amount, to: submittedTo } : null;
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
    attachmentName: attachment?.fileName ?? null,
    // A copy in the app's cache: the upload runs after the step-up, when the picker's URI may be gone.
    pickAttachment: async () => {
      const picked = await DocumentPicker.getDocumentAsync({ type: ['application/pdf', 'image/jpeg', 'image/png'], copyToCacheDirectory: true });
      const asset = picked.canceled ? undefined : picked.assets[0];
      if (asset) setAttachment(toUpload(asset));
    },
    removeAttachment: () => setAttachment(null),
    errors: showErrors ? errors : null,
    toReview: () => {
      setShowErrors(true);
      if (!hasErrors(errors)) setStep('review');
    },
    quote,
    blockedByBalance: quote.data ? reviewBlocker(quote.data) : null,
    approval: org && amount ? approvalNote(policy.data, { orgKind: org.kind, currency, amount }) : ({ kind: 'none' } as const),
    confirm: () => {
      if (!selectedAccountId || !amount || !submittedTo) return;
      create.mutate({ body: { accountId: selectedAccountId, currency, amount, to: submittedTo, concept: concept.trim() }, attachment });
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
