import * as SecureStore from 'expo-secure-store';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { clearSession, getAccessToken, readDeviceKey, readRefreshToken, SECURE_KEYS, saveDeviceKey, saveTokens } from '../session';

function tokens(accessToken: string, refreshToken: string) {
  return { accessToken, refreshToken, accessTokenExpiresAt: '2026-11-07T12:15:00Z', refreshTokenExpiresAt: '2026-12-07T12:00:00Z', tokenType: 'Bearer' as const };
}

// Why: a refresh token left in the keychain after logout signs the next person into the
// company's money (M4); a keystore error at launch must read as "signed out", not a crash.
describe('session', () => {
  beforeEach(() => vi.clearAllMocks());

  it('keeps the refresh token in the secure store and the access token only in memory', async () => {
    await saveTokens(tokens('at-1', 'rt-1'));
    expect(SecureStore.setItemAsync).toHaveBeenCalledWith(SECURE_KEYS.refreshToken, 'rt-1');
    expect(SecureStore.setItemAsync).not.toHaveBeenCalledWith(expect.anything(), 'at-1');
    expect(getAccessToken()).toBe('at-1');
    expect(await readRefreshToken()).toBe('rt-1');
  });

  it('logout deletes every secure key and the in-memory token', async () => {
    await saveTokens(tokens('at-2', 'rt-2'));
    await clearSession();
    for (const key of Object.values(SECURE_KEYS)) {
      expect(SecureStore.deleteItemAsync).toHaveBeenCalledWith(key);
    }
    expect(getAccessToken()).toBeNull();
    expect(await readRefreshToken()).toBeNull();
  });

  it('still deletes the other keys when one delete fails', async () => {
    vi.mocked(SecureStore.deleteItemAsync).mockRejectedValueOnce(new Error('keystore locked'));
    await expect(clearSession()).resolves.toBeUndefined();
    expect(SecureStore.deleteItemAsync).toHaveBeenCalledTimes(Object.values(SECURE_KEYS).length);
  });

  it('keeps the step-up device key in the secure store and drops it on logout', async () => {
    await saveDeviceKey({ deviceId: 'dev-1', secretKey: 'c2VjcmV0' });
    expect(await readDeviceKey()).toEqual({ deviceId: 'dev-1', secretKey: 'c2VjcmV0' });
    await clearSession();
    expect(SecureStore.deleteItemAsync).toHaveBeenCalledWith(SECURE_KEYS.deviceKey);
    expect(await readDeviceKey()).toBeNull();
  });

  it('reads a corrupt device key entry as no key (password fallback)', async () => {
    await SecureStore.setItemAsync(SECURE_KEYS.deviceKey, '{"deviceId":42}');
    expect(await readDeviceKey()).toBeNull();
    await SecureStore.setItemAsync(SECURE_KEYS.deviceKey, 'not json');
    expect(await readDeviceKey()).toBeNull();
  });

  it('reads a keystore error as no session', async () => {
    vi.mocked(SecureStore.getItemAsync).mockRejectedValueOnce(new Error('corrupted keystore'));
    expect(await readRefreshToken()).toBeNull();
  });
});
