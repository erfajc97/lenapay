import { describe, expect, it } from 'vitest';

import { onboardingTarget } from '../onboardingTarget';

// Why: signup already opened the person's wallet; asking the API for another one is a 409
// dead end, and opening the wrong org would send the DNI to a company's KYC.
describe('onboarding target', () => {
  it('opens the existing personal wallet instead of creating a second one', () => {
    expect(onboardingTarget({ kind: 'personal', existingPersonal: { id: 'p1' } })).toEqual({ action: 'open', orgId: 'p1' });
  });

  it('creates a personal wallet only when there is none, and always creates a company', () => {
    expect(onboardingTarget({ kind: 'personal', existingPersonal: null })).toEqual({ action: 'create' });
    expect(onboardingTarget({ kind: 'company', existingPersonal: { id: 'p1' } })).toEqual({ action: 'create' });
  });
});
