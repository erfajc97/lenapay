import { describe, expect, it } from 'vitest';

import { fullNameUpdate } from '../profileRules';

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
