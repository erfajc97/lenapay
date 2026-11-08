import AsyncStorage from '@react-native-async-storage/async-storage';
import { useSyncExternalStore } from 'react';
import { create } from 'zustand';
import { createJSONStorage, persist } from 'zustand/middleware';

import { isLanguagePreference, type LanguagePreference } from '@/shared/i18n/locale';
import { isThemePreference, type ThemePreference } from '@/shared/theme/themePreference';

/**
 * Non-secret preferences that survive a restart (M17): the organization last used, the
 * personal-mode preview flag, the theme and the language. Never tokens (M4).
 */
interface PreferencesState {
  /** The org used last on this device (persisted). */
  activeOrgId: string | null;
  /** The org picked during this run of the app (not persisted): it beats the synced default. */
  sessionOrgId: string | null;
  personalModePreview: boolean;
  themePreference: ThemePreference;
  languagePreference: LanguagePreference;
  setActiveOrgId(orgId: string | null): void;
  /** A new sign-in starts again from the person's default org. */
  resetSessionOrg(): void;
  setPersonalModePreview(enabled: boolean): void;
  setThemePreference(theme: ThemePreference): void;
  setLanguagePreference(language: LanguagePreference): void;
}

export const usePreferencesStore = create<PreferencesState>()(
  persist(
    (set) => ({
      activeOrgId: null,
      sessionOrgId: null,
      personalModePreview: false,
      themePreference: 'system',
      languagePreference: 'system',
      setActiveOrgId: (activeOrgId) => set({ activeOrgId, sessionOrgId: activeOrgId }),
      resetSessionOrg: () => set({ sessionOrgId: null }),
      setPersonalModePreview: (personalModePreview) => set({ personalModePreview }),
      setThemePreference: (themePreference) => set({ themePreference }),
      setLanguagePreference: (languagePreference) => set({ languagePreference }),
    }),
    {
      name: 'lenapay.preferences',
      version: 1,
      storage: createJSONStorage(() => AsyncStorage),
      partialize: ({ activeOrgId, personalModePreview, themePreference, languagePreference }) => ({ activeOrgId, personalModePreview, themePreference, languagePreference }),
      // A saved theme uniwind does not know would throw at boot, and an unknown language would
      // show raw keys: anything else follows the device.
      merge: (persisted, current) => {
        const saved = (persisted ?? {}) as Partial<PreferencesState>;
        return {
          ...current,
          ...saved,
          themePreference: isThemePreference(saved.themePreference) ? saved.themePreference : 'system',
          languagePreference: isLanguagePreference(saved.languagePreference) ? saved.languagePreference : 'system',
        };
      },
    },
  ),
);

/** True once the saved preferences are read back (theme and language apply before the app shows). */
export function usePreferencesHydrated(): boolean {
  return useSyncExternalStore(
    (onChange) => usePreferencesStore.persist.onFinishHydration(onChange),
    () => usePreferencesStore.persist.hasHydrated(),
  );
}
