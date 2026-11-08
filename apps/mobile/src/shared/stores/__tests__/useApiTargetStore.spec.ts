import AsyncStorage from '@react-native-async-storage/async-storage';
import { describe, expect, it } from 'vitest';

import { useApiTargetStore } from '../useApiTargetStore';

const KEY = 'lenapay.apiTarget';

// Why: the API picked in Settings → Desarrollador must survive a restart (the owner flips a
// phone once), and a corrupt saved value must fall back to the build's API, never crash the boot.
describe('useApiTargetStore', () => {
  it('starts with no override and persists the one picked, trimmed', async () => {
    expect(useApiTargetStore.getState().override).toBeNull();
    useApiTargetStore.getState().setOverride({ mode: 'live', url: ' http://192.168.68.53:18080 ' });
    const saved = JSON.parse((await AsyncStorage.getItem(KEY)) ?? '{}') as { state: Record<string, unknown> };
    expect(saved.state).toEqual({ override: { mode: 'live', url: 'http://192.168.68.53:18080' } });
  });

  it('clears the override back to the build default', () => {
    useApiTargetStore.getState().setOverride({ mode: 'live', url: 'http://x:1' });
    useApiTargetStore.getState().clearOverride();
    expect(useApiTargetStore.getState().override).toBeNull();
  });

  it('restores a saved override and drops a corrupt one', async () => {
    await AsyncStorage.setItem(KEY, JSON.stringify({ version: 1, state: { override: { mode: 'live', url: 'http://10.0.0.5:18080' } } }));
    await useApiTargetStore.persist.rehydrate();
    expect(useApiTargetStore.getState().override).toEqual({ mode: 'live', url: 'http://10.0.0.5:18080' });
    await AsyncStorage.setItem(KEY, JSON.stringify({ version: 1, state: { override: { mode: 'staging', url: 'http://x' } } }));
    await useApiTargetStore.persist.rehydrate();
    expect(useApiTargetStore.getState().override).toBeNull();
    await AsyncStorage.setItem(KEY, JSON.stringify({ version: 1, state: { override: { mode: 'live', url: 42 } } }));
    await useApiTargetStore.persist.rehydrate();
    expect(useApiTargetStore.getState().override).toBeNull();
  });
});
