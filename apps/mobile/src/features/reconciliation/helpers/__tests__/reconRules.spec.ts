import { describe, expect, it } from 'vitest';

import { needsAttention, runDifference } from '../reconRules';

// Why: a reconciliation day that hides a one-cent difference is the day money went missing
// unnoticed (R-REC-1).
describe('reconciliation rules', () => {
  const clean = { pspTotal: 1_000_00, ledgerTotal: 1_000_00, missingInLedger: 0, missingAtPsp: 0 };

  it('computes PSP minus ledger', () => {
    expect(runDifference({ pspTotal: 1_085_00, ledgerTotal: 1_000_00 })).toBe(85_00);
    expect(runDifference({ pspTotal: 900_00, ledgerTotal: 1_000_00 })).toBe(-100_00);
  });

  it('flags a single cent of difference or any unmatched item', () => {
    expect(needsAttention(clean)).toBe(false);
    expect(needsAttention({ ...clean, pspTotal: 1_000_01 })).toBe(true);
    expect(needsAttention({ ...clean, missingInLedger: 1 })).toBe(true);
    expect(needsAttention({ ...clean, missingAtPsp: 1 })).toBe(true);
  });
});
