import { useEffect } from 'react';
import { Uniwind, useUniwind } from 'uniwind';

import { usePreferencesStore } from '@/shared/stores/usePreferencesStore';

import { statusBarStyleFor } from './themePreference';

/**
 * Applies the saved theme app-wide through uniwind ('system' follows the device again) and
 * returns the status bar style that matches the theme actually on screen. Mounted once at the root.
 */
export function useApplyThemePreference(): 'light' | 'dark' {
  const preference = usePreferencesStore((state) => state.themePreference);
  const { theme } = useUniwind();
  useEffect(() => {
    Uniwind.setTheme(preference);
  }, [preference]);
  return statusBarStyleFor(theme);
}
