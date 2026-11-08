import { describe, expect, it } from 'vitest';

import type { Account } from '@/shared/api/types';

import { balanceAfter, canContinue, filterAccounts, nextStep, previousStep, searchKey, stepNumber, TRANSFER_STEPS } from '../transferWizard';

// Why: Mover is a 3-step wizard (Desde → Hacia → Monto). A step that lets the person continue
// without its choice, a destination list that offers the source again, or a "saldo después"
// computed on the wrong currency sends money to the wrong place or overdraws a project.
const account = (id: string, name: string, ars = 0, usd = 0): Account => ({
  id,
  kind: 'project',
  name,
  archived: false,
  erpProjectRef: null,
  payersCount: 0,
  cvus: [],
  createdAt: '2026-11-01T12:00:00Z',
  balances: [
    { currency: 'ARS', posted: ars, held: 0, available: ars },
    { currency: 'USD', posted: usd, held: 0, available: usd },
  ],
});

const accounts = [
  account('a', 'Cuenta principal', 5_000_00),
  account('b', 'Obra Belgrano'),
  account('c', 'Torre Palermo'),
  account('d', 'Edificio Núñez'),
  account('e', 'Caja chica'),
];

describe('steps', () => {
  it('runs Desde → Hacia → Monto and back, numbered 1 to 3', () => {
    expect(TRANSFER_STEPS).toEqual(['from', 'to', 'amount']);
    expect(nextStep('from')).toBe('to');
    expect(nextStep('to')).toBe('amount');
    expect(previousStep('amount')).toBe('to');
    expect(previousStep('to')).toBe('from');
    expect(previousStep('from')).toBeNull();
    expect([stepNumber('from'), stepNumber('to'), stepNumber('amount')]).toEqual([1, 2, 3]);
  });

  it('continues from Desde only with a source, and from Hacia only with a different destination', () => {
    expect(canContinue({ step: 'from', fromId: null, toId: null })).toBe(false);
    expect(canContinue({ step: 'from', fromId: 'a', toId: null })).toBe(true);
    expect(canContinue({ step: 'to', fromId: 'a', toId: null })).toBe(false);
    expect(canContinue({ step: 'to', fromId: 'a', toId: 'a' })).toBe(false);
    expect(canContinue({ step: 'to', fromId: 'a', toId: 'b' })).toBe(true);
  });
});

describe('filterAccounts', () => {
  it('shows the first 3 by default and counts the rest behind "Ver todas"', () => {
    const list = filterAccounts({ accounts, query: '', excludeId: null, selectedId: null, expanded: false });
    expect(list.visible.map((item) => item.id)).toEqual(['a', 'b', 'c']);
    expect(list.hiddenCount).toBe(2);
  });

  it('shows every account once expanded', () => {
    const list = filterAccounts({ accounts, query: '', excludeId: null, selectedId: null, expanded: true });
    expect(list.visible).toHaveLength(5);
    expect(list.hiddenCount).toBe(0);
  });

  it('keeps the selected account visible even when it is past the first 3', () => {
    const list = filterAccounts({ accounts, query: '', excludeId: null, selectedId: 'e', expanded: false });
    expect(list.visible.map((item) => item.id)).toEqual(['e', 'a', 'b']);
    expect(list.hiddenCount).toBe(2);
  });

  it('never offers the source as destination', () => {
    const list = filterAccounts({ accounts, query: '', excludeId: 'a', selectedId: null, expanded: true });
    expect(list.visible.map((item) => item.id)).toEqual(['b', 'c', 'd', 'e']);
  });

  it('searches by name ignoring case and accents, with every match shown', () => {
    expect(filterAccounts({ accounts, query: 'nunez', excludeId: null, selectedId: null, expanded: false }).visible.map((item) => item.id)).toEqual(['d']);
    const many = filterAccounts({ accounts, query: 'O', excludeId: null, selectedId: null, expanded: false });
    expect(many.visible.map((item) => item.id)).toEqual(['b', 'c', 'd']);
    expect(many.hiddenCount).toBe(0);
    expect(filterAccounts({ accounts, query: 'zzz', excludeId: null, selectedId: null, expanded: false }).visible).toEqual([]);
  });

  it('normalizes the search key', () => {
    expect(searchKey('  Edificio NÚÑEZ ')).toBe('edificio nunez');
  });
});

describe('balanceAfter', () => {
  it('subtracts the amount from the available balance of the chosen currency', () => {
    expect(balanceAfter({ from: accounts[0] ?? null, currency: 'ARS', amountText: '1.000,50' })).toBe(3_999_50);
    expect(balanceAfter({ from: accounts[0] ?? null, currency: 'ARS', amountText: '5.000' })).toBe(0);
    expect(balanceAfter({ from: account('x', 'x', 0, 10_00), currency: 'USD', amountText: '2,5' })).toBe(7_50);
  });

  it('is null without a source or a valid amount, never a fake balance', () => {
    expect(balanceAfter({ from: null, currency: 'ARS', amountText: '1' })).toBeNull();
    expect(balanceAfter({ from: accounts[0] ?? null, currency: 'ARS', amountText: '' })).toBeNull();
    expect(balanceAfter({ from: accounts[0] ?? null, currency: 'ARS', amountText: 'abc' })).toBeNull();
  });
});
