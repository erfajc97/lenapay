import { describe, expect, it } from 'vitest';

import { defaultAccountChoice, fullNameUpdate } from '../profileRules';

// Why: PATCH /me refuses a blank name (validation.failed) and caps it at 120; the app sends null
// to clear it instead, so an emptied field means "show my email", never an error (R-ID-5).
describe('fullNameUpdate', () => {
  it('trims the name, clears it when blank, and refuses more than 120 characters', () => {
    expect(fullNameUpdate('  Ricardo Benítez ')).toEqual({ value: 'Ricardo Benítez', error: null });
    expect(fullNameUpdate('   ')).toEqual({ value: null, error: null });
    expect(fullNameUpdate('x'.repeat(120))).toEqual({ value: 'x'.repeat(120), error: null });
    expect(fullNameUpdate('x'.repeat(121))).toEqual({ value: null, error: 'validation.fullNameLength' });
  });
});

// Why: the API validates the merged state (an account needs its org, archived is 409); sending
// an account of another org would fail after the person thought it was saved.
describe('defaultAccountChoice', () => {
  const accounts = [{ id: 'main' }, { id: 'torre' }];

  it('keeps the picked account only when it belongs to the picked org', () => {
    expect(defaultAccountChoice({ orgId: 'a', accountId: 'torre' }, accounts)).toEqual({ defaultOrgId: 'a', defaultAccountId: 'torre' });
    expect(defaultAccountChoice({ orgId: 'a', accountId: 'elsewhere' }, accounts)).toEqual({ defaultOrgId: 'a', defaultAccountId: null });
    expect(defaultAccountChoice({ orgId: 'a', accountId: null }, accounts)).toEqual({ defaultOrgId: 'a', defaultAccountId: null });
  });

  it('clears both when no org is picked', () => {
    expect(defaultAccountChoice({ orgId: null, accountId: 'torre' }, accounts)).toEqual({ defaultOrgId: null, defaultAccountId: null });
  });
});
