import { describe, expect, it } from 'vitest';

import { stepUpMode } from '../stepUpMode';

// Why: a phone without biometrics, or a build that cannot sign with the device key, must
// land on the password path; otherwise the person is stuck and the payout never confirms.
describe('stepUpMode', () => {
  it('offers biometrics when the device has them', () => {
    expect(stepUpMode({ biometrics: true, choosePassword: false, lastErrorCode: null })).toBe('biometric');
  });

  it('falls back to the password without biometrics, on request, or when device keys are unavailable', () => {
    expect(stepUpMode({ biometrics: false, choosePassword: false, lastErrorCode: null })).toBe('password');
    expect(stepUpMode({ biometrics: true, choosePassword: true, lastErrorCode: null })).toBe('password');
    expect(stepUpMode({ biometrics: true, choosePassword: false, lastErrorCode: 'stepup.device_key_unavailable' })).toBe('password');
  });

  it('keeps biometrics after an unrelated error', () => {
    expect(stepUpMode({ biometrics: true, choosePassword: false, lastErrorCode: 'network.timeout' })).toBe('biometric');
  });
});
