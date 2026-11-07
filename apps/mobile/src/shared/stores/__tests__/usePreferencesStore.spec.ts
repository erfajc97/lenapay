import AsyncStorage from '@react-native-async-storage/async-storage';
import { describe, expect, it } from 'vitest';

import { usePreferencesStore } from '../usePreferencesStore';

const KEY = 'lenapay.preferences';

// Why: the theme the person picked must survive a restart, and a corrupt or older saved value
// must fall back to following the device instead of crashing the boot or painting nothing.
describe('usePreferencesStore — theme', () => {
  it('follows the device until the person picks a theme', () => {
    expect(usePreferencesStore.getState().themePreference).toBe('system');
  });

  it('persists the picked theme', async () => {
    usePreferencesStore.getState().setThemePreference('dark');
    const saved = JSON.parse((await AsyncStorage.getItem(KEY)) ?? '{}') as { state: Record<string, unknown> };
    expect(saved.state.themePreference).toBe('dark');
  });

  it('restores a saved theme on the next start', async () => {
    await AsyncStorage.setItem(KEY, JSON.stringify({ version: 1, state: { activeOrgId: 'org-9', personalModePreview: false, themePreference: 'light' } }));
    await usePreferencesStore.persist.rehydrate();
    expect(usePreferencesStore.getState()).toMatchObject({ themePreference: 'light', activeOrgId: 'org-9' });
  });

  it('falls back to the device when the saved theme is unknown or missing', async () => {
    await AsyncStorage.setItem(KEY, JSON.stringify({ version: 1, state: { activeOrgId: 'org-9', personalModePreview: false, themePreference: 'sepia' } }));
    await usePreferencesStore.persist.rehydrate();
    expect(usePreferencesStore.getState().themePreference).toBe('system');

    usePreferencesStore.getState().setThemePreference('dark');
    await AsyncStorage.setItem(KEY, JSON.stringify({ version: 1, state: { activeOrgId: 'org-9', personalModePreview: false } }));
    await usePreferencesStore.persist.rehydrate();
    expect(usePreferencesStore.getState().themePreference).toBe('system');
  });
});
