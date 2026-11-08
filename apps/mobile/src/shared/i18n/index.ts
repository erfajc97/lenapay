import { createInstance } from 'i18next';
import { initReactI18next } from 'react-i18next';

import { SOURCE_LOCALE } from './locale';
import enUS from './locales/en-US.json';
import esAR from './locales/es-AR.json';

/**
 * All copy comes from `locales/*.json` through `t()` with dotted keys (M8). The language is
 * switched at runtime by `useApplyLanguagePreference`; es-AR is the fallback.
 */
const instance = createInstance();
void instance.use(initReactI18next).init({
  resources: { 'es-AR': { translation: esAR }, 'en-US': { translation: enUS } },
  lng: SOURCE_LOCALE,
  fallbackLng: SOURCE_LOCALE,
  interpolation: { escapeValue: false },
  returnNull: false,
});

export const i18n = instance;
