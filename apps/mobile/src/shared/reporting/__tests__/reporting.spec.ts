import { describe, expect, it } from 'vitest';

import { missingKeyReporter, scrub } from '../reporting';

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

// Why: a missing i18n key shows "home.myCvu.title" to the person; in development it must shout,
// in a release build it must not log copy keys to device logs.
describe('missing translation reporter', () => {
  it('warns with the key and the locales in development', () => {
    const warnings: string[] = [];
    missingKeyReporter({ dev: true, warn: (message) => warnings.push(message) })(['es-AR'], 'translation', 'home.myCvu.title');
    expect(warnings).toEqual(['[i18n] missing key "home.myCvu.title" (es-AR)']);
  });

  it('stays silent outside development', () => {
    const warnings: string[] = [];
    missingKeyReporter({ dev: false, warn: (message) => warnings.push(message) })(['es-AR'], 'translation', 'x.y');
    expect(warnings).toEqual([]);
  });
});
