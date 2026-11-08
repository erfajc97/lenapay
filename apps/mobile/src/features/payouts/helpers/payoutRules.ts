import type { ApprovalPolicy, Currency, OrgKind, Payout, PayoutQuote } from '@/shared/api/types';
import { approvalThreshold, needsApproval } from '@/shared/helpers/approvalPolicy';
import { classifyDestination } from '@/shared/helpers/cbu';
import { parseAmountInput } from '@/shared/money/parse';

type AmountError = 'validation.amountRequired' | 'validation.amountInvalid' | 'validation.amountZero';

/** The amount field (M16): empty is "no value", invalid is never corrected, 0 is not a payout. */
export function readAmount(text: string): { minor: number | null; error: AmountError | null } {
  const parsed = parseAmountInput(text);
  if (parsed.kind === 'empty') return { minor: null, error: 'validation.amountRequired' };
  if (parsed.kind === 'invalid') return { minor: null, error: 'validation.amountInvalid' };
  if (parsed.minor <= 0) return { minor: null, error: 'validation.amountZero' };
  return { minor: parsed.minor, error: null };
}

interface DetailsErrors {
  amount: AmountError | null;
  account: 'validation.account' | null;
  concept: 'validation.concept' | null;
}

/** Concept: required, 3–80 characters (it travels to the bank statement). */
export function detailsErrors(input: { amountText: string; accountId: string | null; concept: string }): DetailsErrors {
  const length = input.concept.trim().length;
  return {
    amount: readAmount(input.amountText).error,
    account: input.accountId ? null : 'validation.account',
    concept: length >= 3 && length <= 80 ? null : 'validation.concept',
  };
}

export function hasErrors(errors: DetailsErrors): boolean {
  return Object.values(errors).some((error) => error !== null);
}

/** R-OUT-2: amount + fee is held on creation, so what is left after holding it cannot go below 0. */
export function reviewBlocker(quote: Pick<PayoutQuote, 'availableAfter'>): 'insufficient' | null {
  return quote.availableAfter < 0 ? 'insufficient' : null;
}

/** What the person sees after confirming: company payouts wait for a second person (R-OUT-4). */
export function payoutOutcome(payout: Pick<Payout, 'status'>): 'pending_approval' | 'sent' {
  return payout.status === 'pending_approval' ? 'pending_approval' : 'sent';
}

/** The destination can be resolved only when it is a valid key or alias. */
export function canResolve(input: string): boolean {
  const kind = classifyDestination(input);
  return kind === 'cbu' || kind === 'cvu' || kind === 'alias';
}

type ApprovalNote = { kind: 'none' } | { kind: 'unknown' } | { kind: 'auto'; threshold: number } | { kind: 'required'; threshold: number | null };

/**
 * What the review says about approval (R-OUT-7), decided like the API: a personal wallet never
 * waits; a company payout leaves at once only strictly below its currency's threshold.
 */
export function approvalNote(
  policy: Pick<ApprovalPolicy, 'mode' | 'thresholdMinor'> | undefined,
  payout: { orgKind: OrgKind; currency: Currency; amount: number },
): ApprovalNote {
  if (payout.orgKind === 'personal') return { kind: 'none' };
  if (!policy) return { kind: 'unknown' };
  const threshold = approvalThreshold(policy, payout.currency);
  if (!needsApproval(policy, payout) && threshold !== null) return { kind: 'auto', threshold };
  return { kind: 'required', threshold };
}
