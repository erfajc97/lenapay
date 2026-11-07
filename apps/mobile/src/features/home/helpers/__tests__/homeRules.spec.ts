import { describe, expect, it } from 'vitest';

import { balanceFor, kycNotice, quickActions } from '../homeRules';

// Why: Home is the first screen; showing a fake $ 0 while loading reads as "the money is
// gone", and an enabled "Pagar" before KYC sends people into a dead end (M6, R-KYC-3).
describe('home rules', () => {
  const totals = [{ currency: 'ARS' as const, posted: 10, held: 2, available: 8 }];

  it('returns the currency balance or null, never a fake zero', () => {
    expect(balanceFor(totals, 'ARS')?.available).toBe(8);
    expect(balanceFor(totals, 'USD')).toBeNull();
    expect(balanceFor(undefined, 'ARS')).toBeNull();
  });

  it('opens every quick action for an approved operator', () => {
    expect(quickActions({ kycStatus: 'approved', role: 'operator' })).toEqual({ collect: null, pay: null, move: null });
  });

  it('keeps Cobrar open but blocks Pagar and Mover for a viewer', () => {
    expect(quickActions({ kycStatus: 'approved', role: 'viewer' })).toEqual({ collect: null, pay: 'role', move: 'role' });
  });

  it('blocks everything before KYC approval and says why', () => {
    expect(quickActions({ kycStatus: 'submitted', role: 'admin' })).toEqual({ collect: 'kyc', pay: 'kyc', move: 'kyc' });
    expect(kycNotice({ kycStatus: 'submitted' })).toBe('submitted');
    expect(kycNotice({ kycStatus: 'approved' })).toBeNull();
  });
});
