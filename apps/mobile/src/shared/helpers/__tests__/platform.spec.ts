import { describe, expect, it } from 'vitest';

import { keyboardBehavior } from '../platform';

// Why: the wrong keyboard behaviour hides the "Confirmar" button under the keyboard on one
// platform only, which is the one nobody tested (ME11).
describe('keyboardBehavior', () => {
  it('pads on iOS, resizes on Android and does nothing on web', () => {
    expect(keyboardBehavior('ios')).toBe('padding');
    expect(keyboardBehavior('android')).toBe('height');
    expect(keyboardBehavior('web')).toBeUndefined();
  });
});
