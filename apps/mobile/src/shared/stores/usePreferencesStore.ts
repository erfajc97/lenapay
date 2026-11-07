import AsyncStorage from '@react-native-async-storage/async-storage';
import { create } from 'zustand';
import { createJSONStorage, persist } from 'zustand/middleware';

import { isThemePreference, type ThemePreference } from '@/shared/theme/themePreference';

/**
 * Non-secret preferences that survive a restart (M17): the organization last used, the
 * personal-mode preview flag and the theme. Never tokens (M4).
 */
interface PreferencesState {
  activeOrgId: string | null;
  personalModePreview: boolean;
  themePreference: ThemePreference;
  setActiveOrgId(orgId: string | null): void;
  setPersonalModePreview(enabled: boolean): void;
  setThemePreference(theme: ThemePreference): void;
}

export const usePreferencesStore = create<PreferencesState>()(
  persist(
    (set) => ({
      activeOrgId: null,
      personalModePreview: false,
      themePreference: 'system',
      setActiveOrgId: (activeOrgId) => set({ activeOrgId }),
      setPersonalModePreview: (personalModePreview) => set({ personalModePreview }),
      setThemePreference: (themePreference) => set({ themePreference }),
    }),
    {
      name: 'lenapay.preferences',
      version: 1,
      storage: createJSONStorage(() => AsyncStorage),
      partialize: ({ activeOrgId, personalModePreview, themePreference }) => ({ activeOrgId, personalModePreview, themePreference }),
      // A saved theme uniwind does not know would throw at boot: anything else follows the device.
      merge: (persisted, current) => {
        const saved = (persisted ?? {}) as Partial<PreferencesState>;
        return { ...current, ...saved, themePreference: isThemePreference(saved.themePreference) ? saved.themePreference : 'system' };
      },
    },
  ),
);
