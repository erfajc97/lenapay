import * as SecureStore from 'expo-secure-store';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { clearSession, getAccessToken, readRefreshToken, SECURE_KEYS, saveTokens } from '../session';

// Why: a refresh token left in the keychain after logout signs the next person into the
// company's money (M4); a keystore error at launch must read as "signed out", not a crash.
describe('session', () => {
  beforeEach(() => vi.clearAllMocks());

  it('keeps the refresh token in the secure store and the access token only in memory', async () => {
    await saveTokens({ accessToken: 'at-1', refreshToken: 'rt-1', expiresIn: 900 });
    expect(SecureStore.setItemAsync).toHaveBeenCalledWith(SECURE_KEYS.refreshToken, 'rt-1');
    expect(SecureStore.setItemAsync).not.toHaveBeenCalledWith(expect.anything(), 'at-1');
    expect(getAccessToken()).toBe('at-1');
    expect(await readRefreshToken()).toBe('rt-1');
  });

  it('logout deletes every secure key and the in-memory token', async () => {
    await saveTokens({ accessToken: 'at-2', refreshToken: 'rt-2', expiresIn: 900 });
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

  it('reads a keystore error as no session', async () => {
    vi.mocked(SecureStore.getItemAsync).mockRejectedValueOnce(new Error('corrupted keystore'));
    expect(await readRefreshToken()).toBeNull();
  });
});
