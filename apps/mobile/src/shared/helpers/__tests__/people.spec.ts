import { describe, expect, it } from 'vitest';

import { displayName, firstName } from '../people';

// Why: names replace emails on Home, Team and approvals (R-ID-5); a blank name must never show an
// empty greeting or an empty row title.
describe('displayName', () => {
  it('is the chosen name, else the email', () => {
    expect(displayName({ email: 'demo@lenapay.app', fullName: 'Ricardo Benítez' })).toBe('Ricardo Benítez');
    expect(displayName({ email: 'demo@lenapay.app', fullName: null })).toBe('demo@lenapay.app');
  });
});

describe('firstName', () => {
  it('takes the first word and never falls back to the email', () => {
    expect(firstName('Ricardo Benítez')).toBe('Ricardo');
    expect(firstName('  Lucía   Fernández ')).toBe('Lucía');
    expect(firstName(null)).toBeNull();
    expect(firstName('   ')).toBeNull();
    expect(firstName(undefined)).toBeNull();
  });
});
