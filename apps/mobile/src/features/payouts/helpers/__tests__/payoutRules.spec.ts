import { describe, expect, it } from 'vitest';

import { buildCvu } from '@/shared/helpers/cbu';

import { approvalNote, canResolve, detailsErrors, hasErrors, payoutOutcome, readAmount, reviewBlocker } from '../payoutRules';

// Why: this screen decides how much money leaves. `5,50` must be 550 minor units, an empty
// field must not become $ 0, and a payout whose amount + fee exceeds the available balance
// must be stopped before the step-up, not after (R-OUT-1, R-OUT-2, M16).
describe('readAmount', () => {
  it('reads es-AR input to minor units', () => {
    expect(readAmount('150.000,50')).toEqual({ minor: 15_000_050, error: null });
    expect(readAmount('5,50')).toEqual({ minor: 550, error: null });
  });

  it('separates empty, invalid and zero', () => {
    expect(readAmount('')).toEqual({ minor: null, error: 'validation.amountRequired' });
    expect(readAmount('12,345')).toEqual({ minor: null, error: 'validation.amountInvalid' });
    expect(readAmount('0,00')).toEqual({ minor: null, error: 'validation.amountZero' });
    expect(readAmount('0,01')).toEqual({ minor: 1, error: null });
  });
});

describe('detailsErrors', () => {
  it('passes a complete form', () => {
    expect(hasErrors(detailsErrors({ amountText: '100', accountId: 'acc', concept: 'Hormigón' }))).toBe(false);
  });

  it('requires an account and bounds the concept at 3 and 80 characters', () => {
    expect(detailsErrors({ amountText: '100', accountId: null, concept: 'abc' }).account).toBe('validation.account');
    expect(detailsErrors({ amountText: '100', accountId: 'a', concept: 'ab' }).concept).toBe('validation.concept');
    expect(detailsErrors({ amountText: '100', accountId: 'a', concept: 'a'.repeat(80) }).concept).toBeNull();
    expect(detailsErrors({ amountText: '100', accountId: 'a', concept: 'a'.repeat(81) }).concept).toBe('validation.concept');
  });
});

describe('reviewBlocker', () => {
  it('lets the last affordable total through and stops one minor unit more', () => {
    expect(reviewBlocker({ availableAfter: 0 })).toBeNull();
    expect(reviewBlocker({ availableAfter: -1 })).toBe('insufficient');
  });
});

describe('payoutOutcome and canResolve', () => {
  it('tells "waiting for approval" from "sent"', () => {
    expect(payoutOutcome({ status: 'pending_approval' })).toBe('pending_approval');
    expect(payoutOutcome({ status: 'approved' })).toBe('sent');
  });

  it('resolves only valid keys and aliases', () => {
    expect(canResolve(buildCvu('0031', '7'))).toBe(true);
    expect(canResolve('hormigon.plata')).toBe(true);
    expect(canResolve('123')).toBe(false);
    expect(canResolve('no vale')).toBe(false);
  });
});

// Why: the review must tell the truth before the step-up (R-OUT-7): "sale sin aprobación" for
// a payout that then waits, or the reverse, misleads the person about when the money leaves.
describe('approvalNote', () => {
  const policy = { mode: 'threshold' as const, thresholdMinor: { ARS: 500_000_00, USD: null } };
  const company = { orgKind: 'company' as const, currency: 'ARS' as const };

  it('says it leaves without approval strictly below the threshold, and needs it from the threshold', () => {
    expect(approvalNote(policy, { ...company, amount: 500_000_00 - 1 })).toEqual({ kind: 'auto', threshold: 500_000_00 });
    expect(approvalNote(policy, { ...company, amount: 500_000_00 })).toEqual({ kind: 'required', threshold: 500_000_00 });
  });

  it('needs approval with no threshold for the currency or in always mode, and says nothing for a personal wallet', () => {
    expect(approvalNote(policy, { ...company, currency: 'USD', amount: 1 })).toEqual({ kind: 'required', threshold: null });
    expect(approvalNote({ ...policy, mode: 'always' }, { ...company, amount: 1 })).toEqual({ kind: 'required', threshold: null });
    expect(approvalNote(policy, { ...company, orgKind: 'personal', amount: 900_000_00 })).toEqual({ kind: 'none' });
  });

  it('is unknown while the policy is not loaded (never a guess)', () => {
    expect(approvalNote(undefined, { ...company, amount: 1 })).toEqual({ kind: 'unknown' });
    expect(approvalNote(undefined, { ...company, orgKind: 'personal', amount: 1 })).toEqual({ kind: 'none' });
  });
});
