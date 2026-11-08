import AsyncStorage from '@react-native-async-storage/async-storage';
import { useSyncExternalStore } from 'react';
import { create } from 'zustand';
import { createJSONStorage, persist } from 'zustand/middleware';

import type { ApiMode, ApiTarget } from '@/shared/config/env';

/**
 * Settings → Desarrollador (development builds only): the API mode and URL picked on the
 * device, so the owner can flip a phone between mock and the local API without rebuilding.
 * Not a secret (a URL and a mode), so AsyncStorage is the right store (M17).
 */
interface ApiTargetState {
  override: ApiTarget | null;
  setOverride(target: ApiTarget): void;
  clearOverride(): void;
}

function isApiMode(value: unknown): value is ApiMode {
  return value === 'mock' || value === 'live';
}

export const useApiTargetStore = create<ApiTargetState>()(
  persist(
    (set) => ({
      override: null,
      setOverride: (target) => set({ override: { mode: target.mode, url: target.url.trim() } }),
      clearOverride: () => set({ override: null }),
    }),
    {
      name: 'lenapay.apiTarget',
      version: 1,
      storage: createJSONStorage(() => AsyncStorage),
      partialize: ({ override }) => ({ override }),
      // A corrupt or older saved value means "use the build's env", never a crash at boot.
      merge: (persisted, current) => {
        const saved = (persisted ?? {}) as { override?: Partial<ApiTarget> | null };
        const value = saved.override;
        const override = value && isApiMode(value.mode) && typeof value.url === 'string' ? { mode: value.mode, url: value.url } : null;
        return { ...current, override };
      },
    },
  ),
);

/** True once the saved override is read back: the first request must already use it. */
export function useApiTargetHydrated(): boolean {
  return useSyncExternalStore(
    (onChange) => useApiTargetStore.persist.onFinishHydration(onChange),
    () => useApiTargetStore.persist.hasHydrated(),
  );
}
