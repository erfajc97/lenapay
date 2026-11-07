import { createInstance } from 'i18next';
import { initReactI18next } from 'react-i18next';

import esAR from './locales/es-AR.json';
import { LOCALE } from './locale';

/** All copy comes from `locales/*.json` through `t()` with dotted keys (M8). */
const instance = createInstance();
void instance.use(initReactI18next).init({
  resources: { [LOCALE]: { translation: esAR } },
  lng: LOCALE,
  fallbackLng: LOCALE,
  interpolation: { escapeValue: false },
  returnNull: false,
});

export const i18n = instance;
