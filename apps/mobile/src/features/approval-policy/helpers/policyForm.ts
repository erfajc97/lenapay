import type { ApprovalMode, ApprovalPolicy, Currency, UpdateApprovalPolicyRequest } from '@/shared/api/types';
import { minorToInput } from '@/shared/money/format';
import { parseAmountInput } from '@/shared/money/parse';

/** The editable policy: thresholds as typed text, so the owner's own format survives (M16). */
export interface PolicyForm {
  mode: ApprovalMode;
  ARS: string;
  USD: string;
  allowOwnerSelfApproval: boolean;
}

type AmountError = 'validation.amountInvalid' | 'validation.amountZero';

interface PolicyFormErrors {
  ARS: AmountError | null;
  USD: AmountError | null;
  thresholds: 'validation.thresholdRequired' | null;
}

export function formFromPolicy(policy: ApprovalPolicy): PolicyForm {
  const text = (currency: Currency) => {
    const minor = policy.thresholdMinor[currency];
    return minor === null ? '' : minorToInput(minor);
  };
  return { mode: policy.mode, ARS: text('ARS'), USD: text('USD'), allowOwnerSelfApproval: policy.allowOwnerSelfApproval };
}

function readThreshold(text: string): { minor: number | null; error: AmountError | null } {
  const parsed = parseAmountInput(text);
  if (parsed.kind === 'empty') return { minor: null, error: null };
  if (parsed.kind === 'invalid') return { minor: null, error: 'validation.amountInvalid' };
  if (parsed.minor < 1) return { minor: null, error: 'validation.amountZero' };
  return { minor: parsed.minor, error: null };
}

/**
 * The PUT body, or the errors that block it. Empty = that currency always needs approval; the
 * typed thresholds travel in `always` mode too (the API keeps them, ignored, for later).
 */
export function policyFormResult(form: PolicyForm): { body: UpdateApprovalPolicyRequest | null; errors: PolicyFormErrors } {
  const ars = readThreshold(form.ARS);
  const usd = readThreshold(form.USD);
  const thresholdMinor: NonNullable<UpdateApprovalPolicyRequest['thresholdMinor']> = {};
  if (ars.minor !== null) thresholdMinor.ARS = ars.minor;
  if (usd.minor !== null) thresholdMinor.USD = usd.minor;
  const missing = form.mode === 'threshold' && ars.minor === null && usd.minor === null && !ars.error && !usd.error;
  const errors: PolicyFormErrors = { ARS: ars.error, USD: usd.error, thresholds: missing ? 'validation.thresholdRequired' : null };
  const blocked = errors.ARS !== null || errors.USD !== null || errors.thresholds !== null;
  return { body: blocked ? null : { mode: form.mode, thresholdMinor, allowOwnerSelfApproval: form.allowOwnerSelfApproval }, errors };
}
