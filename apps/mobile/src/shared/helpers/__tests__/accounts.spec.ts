import { describe, expect, it } from 'vitest';

import { accountNameError, archiveBlocker, sortAccounts } from '../accounts';

// Why: archiving an account that still holds money (or a hold in flight) hides that money
// from every screen (R-ACC-3).
const zero = [
  { currency: 'ARS' as const, posted: 0, held: 0, available: 0 },
  { currency: 'USD' as const, posted: 0, held: 0, available: 0 },
];

describe('archiveBlocker', () => {
  it('allows an empty project account', () => {
    expect(archiveBlocker({ kind: 'project', balances: zero, archived: false })).toBeNull();
  });

  it('blocks a single minor unit posted or held in any currency', () => {
    expect(archiveBlocker({ kind: 'project', balances: [zero[0]!, { ...zero[1]!, posted: 1, available: 1 }], archived: false })).toBe('not_empty');
    expect(archiveBlocker({ kind: 'project', balances: [{ ...zero[0]!, held: 1, available: -1 }, zero[1]!], archived: false })).toBe('not_empty');
  });

  it('never archives the main account or an archived one', () => {
    expect(archiveBlocker({ kind: 'main', balances: zero, archived: false })).toBe('main');
    expect(archiveBlocker({ kind: 'project', balances: zero, archived: true })).toBe('archived');
  });
});

describe('account helpers', () => {
  it('sorts main, projects by name, archived last', () => {
    const sorted = sortAccounts([
      { kind: 'project', name: 'Torre', archived: false },
      { kind: 'project', name: 'Caballito', archived: true },
      { kind: 'main', name: 'Principal', archived: false },
      { kind: 'project', name: 'Barrio', archived: false },
    ]);
    expect(sorted.map((item) => item.name)).toEqual(['Principal', 'Barrio', 'Torre', 'Caballito']);
  });

  it('bounds the name at 3 and 40 characters', () => {
    expect(accountNameError('ab')).toBe('validation.accountName');
    expect(accountNameError('abc')).toBeNull();
    expect(accountNameError('a'.repeat(40))).toBeNull();
    expect(accountNameError('a'.repeat(41))).toBe('validation.accountName');
  });
});
