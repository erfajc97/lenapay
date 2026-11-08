import { describe, expect, it } from 'vitest';

import type { Account } from '@/shared/api/types';

import { transferBlocker } from '../transferRules';

// Why: moving more than the available balance between projects would overdraw a project
// that has payouts on hold (R-ACC-2, R-ACC-4).
const account = (id: string, availableArs: number): Account => ({
  id,
  kind: 'project',
  name: id,
  archived: false,
  erpProjectRef: null,
  payersCount: 0,
  createdAt: '2026-11-01T12:00:00Z',
  balances: [
    { currency: 'ARS', posted: availableArs, held: 0, available: availableArs },
    { currency: 'USD', posted: 0, held: 0, available: 0 },
  ],
});

describe('transferBlocker', () => {
  const from = account('a', 1_000_00);
  const to = account('b', 0);

  it('lets the whole available balance through and stops one minor unit more', () => {
    expect(transferBlocker({ from, to, currency: 'ARS', amountText: '1.000,00' })).toBeNull();
    expect(transferBlocker({ from, to, currency: 'ARS', amountText: '1.000,01' })).toBe('insufficient');
  });

  it('checks the balance of the selected currency', () => {
    expect(transferBlocker({ from, to, currency: 'USD', amountText: '1' })).toBe('insufficient');
  });

  it('requires both accounts, different ones, and a positive amount', () => {
    expect(transferBlocker({ from: null, to, currency: 'ARS', amountText: '1' })).toBe('from');
    expect(transferBlocker({ from, to: null, currency: 'ARS', amountText: '1' })).toBe('to');
    expect(transferBlocker({ from, to: from, currency: 'ARS', amountText: '1' })).toBe('same_account');
    expect(transferBlocker({ from, to, currency: 'ARS', amountText: '' })).toBe('amount');
    expect(transferBlocker({ from, to, currency: 'ARS', amountText: '0' })).toBe('amount');
  });
});
