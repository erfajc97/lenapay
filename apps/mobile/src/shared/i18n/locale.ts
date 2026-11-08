/**
 * The app's languages (M8, P3). es-AR is the source catalog and the fallback; Settings → Idioma
 * picks one or follows the device ('system'). Formatting reads `activeLocale()` at call time,
 * so money and dates follow the language; the currency itself (ARS/USD) never changes.
 */
export const SOURCE_LOCALE = 'es-AR';
export const APP_LOCALES = ['es-AR', 'en-US'] as const;
export type AppLocale = (typeof APP_LOCALES)[number];

export const LANGUAGE_PREFERENCES = ['system', ...APP_LOCALES] as const;
export type LanguagePreference = (typeof LANGUAGE_PREFERENCES)[number];

/** i18n key of each option's name in Idioma (endonyms live in the catalogs, M8). */
export const LANGUAGE_LABEL_KEY: Record<LanguagePreference, string> = {
  system: 'settings.languages.system',
  'es-AR': 'settings.languages.esAR',
  'en-US': 'settings.languages.enUS',
};

export function isLanguagePreference(value: unknown): value is LanguagePreference {
  return LANGUAGE_PREFERENCES.includes(value as LanguagePreference);
}

/** The device's ordered languages (expo-localization `getLocales()` shape). */
interface DeviceLocale {
  languageTag: string;
  languageCode: string | null;
}

/** Picked language, or the first device language we ship (exact tag, else same language). */
export function resolveLocale(preference: LanguagePreference, device: DeviceLocale[]): AppLocale {
  if (preference !== 'system') return preference;
  for (const locale of device) {
    const exact = APP_LOCALES.find((tag) => tag === locale.languageTag);
    if (exact) return exact;
    const sameLanguage = APP_LOCALES.find((tag) => tag.split('-')[0] === locale.languageCode);
    if (sameLanguage) return sameLanguage;
  }
  return SOURCE_LOCALE;
}

let current: AppLocale = SOURCE_LOCALE;

export function activeLocale(): AppLocale {
  return current;
}

/** Set before `i18n.changeLanguage`, so the re-render it triggers already formats in the new locale. */
export function setActiveLocale(locale: AppLocale): void {
  current = locale;
}
