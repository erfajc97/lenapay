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

// Why: the language picked in Idioma must survive a restart; a saved language we no longer
// ship (or garbage) must fall back to the device, never to raw keys.
describe('usePreferencesStore — language', () => {
  it('follows the device until the person picks a language, and persists the pick', async () => {
    usePreferencesStore.setState({ languagePreference: 'system' });
    expect(usePreferencesStore.getState().languagePreference).toBe('system');
    usePreferencesStore.getState().setLanguagePreference('en-US');
    const saved = JSON.parse((await AsyncStorage.getItem(KEY)) ?? '{}') as { state: Record<string, unknown> };
    expect(saved.state.languagePreference).toBe('en-US');
  });

  it('restores a saved language and drops an unknown one', async () => {
    await AsyncStorage.setItem(KEY, JSON.stringify({ version: 1, state: { activeOrgId: null, personalModePreview: false, languagePreference: 'en-US' } }));
    await usePreferencesStore.persist.rehydrate();
    expect(usePreferencesStore.getState().languagePreference).toBe('en-US');
    await AsyncStorage.setItem(KEY, JSON.stringify({ version: 1, state: { activeOrgId: null, personalModePreview: false, languagePreference: 'pt-BR' } }));
    await usePreferencesStore.persist.rehydrate();
    expect(usePreferencesStore.getState().languagePreference).toBe('system');
  });
});
