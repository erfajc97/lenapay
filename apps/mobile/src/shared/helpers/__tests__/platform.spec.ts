import { describe, expect, it } from 'vitest';

import { keyboardBehavior, modalReportsDismiss, pushPlatform } from '../platform';

// Why: the wrong keyboard behaviour hides the "Confirmar" button under the keyboard on one
// platform only, which is the one nobody tested (ME11).
describe('keyboardBehavior', () => {
  it('pads on iOS, resizes on Android and does nothing on web', () => {
    expect(keyboardBehavior('ios')).toBe('padding');
    expect(keyboardBehavior('android')).toBe('height');
    expect(keyboardBehavior('web')).toBeUndefined();
  });
});

// Why: navigation waits for a sheet's dismissal; on the platform that never reports it the
// sheet must report it itself, or the screen never leaves (and on iOS reporting it early
// navigates under a Modal still on screen, which freezes the app).
describe('modalReportsDismiss', () => {
  it('trusts the Modal on iOS and web, not on Android', () => {
    expect(modalReportsDismiss('ios')).toBe(true);
    expect(modalReportsDismiss('web')).toBe(true);
    expect(modalReportsDismiss('android')).toBe(false);
  });
});

// Why: the API drops a push token whose platform it does not know (strict enum), so a new OS
// name must map to `unknown` instead of failing the registration.
describe('pushPlatform', () => {
  it('passes the contract platforms through and maps anything else to unknown', () => {
    expect(pushPlatform('ios')).toBe('ios');
    expect(pushPlatform('android')).toBe('android');
    expect(pushPlatform('web')).toBe('web');
    expect(pushPlatform('windows')).toBe('unknown');
  });
});
