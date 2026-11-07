import AsyncStorage from '@react-native-async-storage/async-storage';
import * as Crypto from 'expo-crypto';

/**
 * Client-generated Idempotency-Key per money action (R-OUT-3, MO2). The key survives
 * closing the sheet, unmounting and restarting the app (persisted per scope for 24 h), so a
 * retry after a timeout replays the same operation instead of paying twice. It is bound to
 * the payload: a changed amount or destination is a new operation with a new key.
 * Not a secret, so AsyncStorage is the right store.
 */
const PREFIX = 'lenapay.idempotency.';
export const IDEMPOTENCY_TTL_MS = 24 * 60 * 60 * 1000;

interface StoredKey {
  key: string;
  hash: string;
  createdAt: number;
}

/** Key order must not change the hash: `{a, b}` and `{b, a}` are the same payload. */
function stableStringify(value: unknown): string {
  if (Array.isArray(value)) return `[${value.map(stableStringify).join(',')}]`;
  if (value && typeof value === 'object') {
    const entries = Object.entries(value as Record<string, unknown>)
      .filter(([, item]) => item !== undefined)
      .sort(([a], [b]) => a.localeCompare(b));
    return `{${entries.map(([key, item]) => `${JSON.stringify(key)}:${stableStringify(item)}`).join(',')}}`;
  }
  return JSON.stringify(value);
}

function parse(raw: string | null): StoredKey | null {
  if (!raw) return null;
  try {
    const value = JSON.parse(raw) as Partial<StoredKey>;
    return typeof value.key === 'string' && typeof value.hash === 'string' && typeof value.createdAt === 'number' ? (value as StoredKey) : null;
  } catch {
    return null;
  }
}

export async function idempotencyKeyFor(scope: string, payload: unknown, now: number): Promise<string> {
  const hash = await Crypto.digestStringAsync(Crypto.CryptoDigestAlgorithm.SHA256, stableStringify(payload));
  const stored = parse(await AsyncStorage.getItem(PREFIX + scope));
  if (stored && stored.hash === hash && now - stored.createdAt < IDEMPOTENCY_TTL_MS) return stored.key;
  const key = Crypto.randomUUID();
  await AsyncStorage.setItem(PREFIX + scope, JSON.stringify({ key, hash, createdAt: now } satisfies StoredKey));
  return key;
}

/** Called after the API answered: the next action in this scope is a new operation. */
export async function releaseIdempotencyKey(scope: string): Promise<void> {
  await AsyncStorage.removeItem(PREFIX + scope);
}
