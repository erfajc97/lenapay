import { describe, expect, it } from 'vitest';

import { accountDisplayName, accountNameError, archiveBlocker, sortAccounts } from '../accounts';

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

// Why: the API seeds the main and personal accounts with English names ('Main account'),
// which showed up untranslated in an es-AR app; a name the person chose must stay as typed.
describe('accountDisplayName', () => {
  const t = (key: string) => `<${key}>`;

  it('labels the API-seeded main and personal accounts by kind through i18n', () => {
    expect(accountDisplayName({ kind: 'main', name: 'Main account' }, t)).toBe('<accounts.defaultName.main>');
    expect(accountDisplayName({ kind: 'personal', name: 'Personal account' }, t)).toBe('<accounts.defaultName.personal>');
  });

  it('keeps a renamed main account and every project account as named', () => {
    expect(accountDisplayName({ kind: 'main', name: 'Caja central' }, t)).toBe('Caja central');
    expect(accountDisplayName({ kind: 'project', name: 'Main account' }, t)).toBe('Main account');
    expect(accountDisplayName({ kind: 'personal', name: 'Main account' }, t)).toBe('Main account');
  });
});
