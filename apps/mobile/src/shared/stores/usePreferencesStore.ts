import AsyncStorage from '@react-native-async-storage/async-storage';
import { create } from 'zustand';
import { createJSONStorage, persist } from 'zustand/middleware';

/**
 * Non-secret preferences that survive a restart (M17): the organization last used and the
 * personal-mode preview flag. Never tokens (M4).
 */
interface PreferencesState {
  activeOrgId: string | null;
  personalModePreview: boolean;
  setActiveOrgId(orgId: string | null): void;
  setPersonalModePreview(enabled: boolean): void;
}

export const usePreferencesStore = create<PreferencesState>()(
  persist(
    (set) => ({
      activeOrgId: null,
      personalModePreview: false,
      setActiveOrgId: (activeOrgId) => set({ activeOrgId }),
      setPersonalModePreview: (personalModePreview) => set({ personalModePreview }),
    }),
    {
      name: 'lenapay.preferences',
      version: 1,
      storage: createJSONStorage(() => AsyncStorage),
      partialize: ({ activeOrgId, personalModePreview }) => ({ activeOrgId, personalModePreview }),
    },
  ),
);
