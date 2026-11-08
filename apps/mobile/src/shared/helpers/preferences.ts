import type { Locale, UserPreferences } from '@/shared/api/types';
import type { LanguagePreference } from '@/shared/i18n/locale';
import type { ThemePreference } from '@/shared/theme/themePreference';

/**
 * Language and theme synced with the server (R-ID-5). The server has no "system" language: an
 * unset locale (null) means "follow the device"; the theme keeps `system` as a real value.
 */
export function localFromServer(preferences: Pick<UserPreferences, 'locale' | 'theme'> | null): { language?: LanguagePreference; theme?: ThemePreference } {
  if (!preferences) return {};
  return {
    ...(preferences.locale === null ? {} : { language: preferences.locale }),
    ...(preferences.theme === null ? {} : { theme: preferences.theme }),
  };
}

export function serverLanguage(language: LanguagePreference): Locale | null {
  return language === 'system' ? null : language;
}
