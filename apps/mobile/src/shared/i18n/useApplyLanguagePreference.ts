import { useLocales } from 'expo-localization';
import { useEffect } from 'react';

import { usePreferencesStore } from '@/shared/stores/usePreferencesStore';

import { i18n } from './index';
import { resolveLocale, setActiveLocale } from './locale';

/**
 * Applies Settings → Idioma app-wide: the picked language, or the device's ('system', which
 * follows a change of the phone's language too). Mounted once at the root.
 */
export function useApplyLanguagePreference(): void {
  const preference = usePreferencesStore((state) => state.languagePreference);
  const device = useLocales();
  const locale = resolveLocale(preference, device);
  useEffect(() => {
    setActiveLocale(locale);
    i18n.changeLanguage(locale).catch(() => {
      // Both catalogs are bundled, so this cannot fail to load; the previous language stays.
    });
  }, [locale]);
}
