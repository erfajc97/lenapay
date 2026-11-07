import * as SecureStore from 'expo-secure-store';

import type { AuthTokens } from '@/shared/api/types';

/**
 * The ONE session module (M4, ME2). The refresh token lives in the OS secure store; the
 * short-lived access token only in memory. A keystore error reads as "no session".
 */
export const SECURE_KEYS = {
  refreshToken: 'lenapay.refreshToken',
  biometricsEnabled: 'lenapay.biometricsEnabled',
} as const;

let accessToken: string | null = null;

export function getAccessToken(): string | null {
  return accessToken;
}

/**
 * Without a keystore (web preview, broken keychain) the session simply is not persisted: it
 * lives in memory until the app closes. Never a fallback to plain storage (M4).
 */
async function writeSecure(key: string, value: string): Promise<void> {
  try {
    await SecureStore.setItemAsync(key, value);
  } catch {
    // Not persisted; see above.
  }
}

export async function saveTokens(tokens: AuthTokens): Promise<void> {
  accessToken = tokens.accessToken;
  await writeSecure(SECURE_KEYS.refreshToken, tokens.refreshToken);
}

async function readSecure(key: string): Promise<string | null> {
  try {
    return await SecureStore.getItemAsync(key);
  } catch {
    return null;
  }
}

export function readRefreshToken(): Promise<string | null> {
  return readSecure(SECURE_KEYS.refreshToken);
}

/** Biometric unlock is a per-device choice tied to the stored session, so it lives beside it. */
export async function readBiometricsEnabled(): Promise<boolean> {
  return (await readSecure(SECURE_KEYS.biometricsEnabled)) !== 'false';
}

export async function setBiometricsEnabled(enabled: boolean): Promise<void> {
  await writeSecure(SECURE_KEYS.biometricsEnabled, enabled ? 'true' : 'false');
}

/** Logout: forget the in-memory token and delete EVERY secure entry, even if one delete fails. */
export async function clearSession(): Promise<void> {
  accessToken = null;
  await Promise.all(
    Object.values(SECURE_KEYS).map(async (key) => {
      try {
        await SecureStore.deleteItemAsync(key);
      } catch {
        // One failed delete must not keep the others (or the logout) from running.
      }
    }),
  );
}
