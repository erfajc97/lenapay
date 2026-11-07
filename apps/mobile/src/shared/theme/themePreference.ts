/** Settings → Apariencia (owner request): follow the device, or force light or dark. */
export const THEME_PREFERENCES = ['system', 'light', 'dark'] as const;

export type ThemePreference = (typeof THEME_PREFERENCES)[number];

export function isThemePreference(value: unknown): value is ThemePreference {
  return THEME_PREFERENCES.includes(value as ThemePreference);
}

/** Status bar content contrasts with the page: light icons on the dark theme. */
export function statusBarStyleFor(theme: string): 'light' | 'dark' {
  return theme === 'dark' ? 'light' : 'dark';
}
