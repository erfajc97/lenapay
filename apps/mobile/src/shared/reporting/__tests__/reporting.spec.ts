import { describe, expect, it } from 'vitest';

import { scrub } from '../reporting';

// Why: crash dashboards have more readers than the database; a name, email or token in an
// error context is a data leak (M12).
describe('scrub', () => {
  it('keeps exactly the allowed id and enum keys', () => {
    expect(
      scrub({ code: 'payout.insufficient_funds', status: 422, orgId: 'org_1', email: 'a@b.c', fullName: 'Juan', body: '{}', screen: 'Pay' }),
    ).toEqual({ code: 'payout.insufficient_funds', status: 422, orgId: 'org_1', screen: 'Pay' });
  });

  it('drops values that carry tokens or query strings', () => {
    expect(scrub({ name: 'Bearer abc', screen: '/x?token=1' })).toEqual({});
  });
});
