import { describe, expect, it } from 'vitest';

import type { ApprovalPolicy } from '@/shared/api/types';

import { formFromPolicy, policyFormResult } from '../policyForm';

// Why: the owner types the threshold in local format (M16); stripping the comma or sending 0
// would let far bigger payouts leave without approval (R-OUT-7), and threshold mode without any
// threshold is refused by the API (validation.failed).
const policy: ApprovalPolicy = {
  mode: 'threshold',
  thresholdMinor: { ARS: 500_000_00, USD: null },
  allowOwnerSelfApproval: true,
  ownerSelfApprovalEffective: false,
  updatedBy: null,
  updatedAt: null,
};

describe('formFromPolicy', () => {
  it('turns the stored thresholds into editable text, empty when there is none', () => {
    expect(formFromPolicy(policy)).toEqual({ mode: 'threshold', ARS: '500000', USD: '', allowOwnerSelfApproval: true });
    expect(formFromPolicy({ ...policy, thresholdMinor: { ARS: 1_234_56, USD: 1_00 } })).toMatchObject({ ARS: '1234,56', USD: '1' });
  });
});

describe('policyFormResult', () => {
  const form = { mode: 'threshold' as const, ARS: '', USD: '', allowOwnerSelfApproval: false };

  it('parses both separators and sends only the currencies with a threshold', () => {
    expect(policyFormResult({ ...form, ARS: '500.000' }).body).toEqual({ mode: 'threshold', thresholdMinor: { ARS: 500_000_00 }, allowOwnerSelfApproval: false });
    expect(policyFormResult({ ...form, ARS: '1.234,56', USD: '5,50' }).body?.thresholdMinor).toEqual({ ARS: 1_234_56, USD: 5_50 });
    expect(policyFormResult({ ...form, USD: '1,234.56' }).body?.thresholdMinor).toEqual({ USD: 1_234_56 });
  });

  it('refuses threshold mode without any threshold, and invalid or zero amounts', () => {
    expect(policyFormResult(form)).toEqual({ body: null, errors: { ARS: null, USD: null, thresholds: 'validation.thresholdRequired' } });
    expect(policyFormResult({ ...form, ARS: 'abc' }).errors.ARS).toBe('validation.amountInvalid');
    expect(policyFormResult({ ...form, ARS: '0' }).errors.ARS).toBe('validation.amountZero');
    expect(policyFormResult({ ...form, ARS: '0,01' }).body?.thresholdMinor).toEqual({ ARS: 1 });
  });

  it('accepts always mode with no threshold and keeps typed thresholds (ignored while always)', () => {
    expect(policyFormResult({ ...form, mode: 'always' }).body).toEqual({ mode: 'always', thresholdMinor: {}, allowOwnerSelfApproval: false });
    expect(policyFormResult({ ...form, mode: 'always', ARS: '100' }).body?.thresholdMinor).toEqual({ ARS: 100_00 });
    expect(policyFormResult({ ...form, mode: 'always', ARS: 'x' }).body).toBeNull();
  });
});
