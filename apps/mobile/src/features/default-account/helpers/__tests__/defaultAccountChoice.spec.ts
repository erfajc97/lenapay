import { describe, expect, it } from 'vitest';

import { defaultAccountChoice } from '../defaultAccountChoice';

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
