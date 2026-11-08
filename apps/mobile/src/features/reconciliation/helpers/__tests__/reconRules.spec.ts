import { describe, expect, it } from 'vitest';

import { itemAmount, needsAttention, runDifference } from '../reconRules';

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

// Why: an item missing on one side has a null amount there; showing 0 (or the wrong sign)
// hides which way the money moved.
describe('itemAmount', () => {
  it('reads the PSP amount, falls back to the ledger, and signs it by direction', () => {
    expect(itemAmount({ direction: 'in', pspAmount: 85_00, ledgerAmount: null })).toBe(85_00);
    expect(itemAmount({ direction: 'out', pspAmount: null, ledgerAmount: 200_00 })).toBe(-200_00);
    expect(itemAmount({ direction: 'in', pspAmount: 10_00, ledgerAmount: 9_00 })).toBe(10_00);
    expect(itemAmount({ direction: 'out', pspAmount: null, ledgerAmount: null })).toBe(-0);
  });
});
