import { describe, expect, it } from 'vitest';

import { isThemePreference, statusBarStyleFor } from '../themePreference';

// Why: a dark status bar on a dark page hides the clock and battery; an unknown preference
// handed to uniwind throws ("not registered") and takes the whole app down at boot.
describe('themePreference', () => {
  it('accepts only the three options of Apariencia', () => {
    expect(['system', 'light', 'dark'].every(isThemePreference)).toBe(true);
    expect(isThemePreference('sepia')).toBe(false);
    expect(isThemePreference(undefined)).toBe(false);
  });

  it('uses light status bar content on the dark theme and dark content on the light one', () => {
    expect(statusBarStyleFor('dark')).toBe('light');
    expect(statusBarStyleFor('light')).toBe('dark');
  });
});
