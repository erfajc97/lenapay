/// <reference types="node" />
import { createHash, randomUUID, getRandomValues } from 'node:crypto';
import { vi } from 'vitest';

// Native modules mocked once, with their real contracts (ME10).

vi.mock('expo-crypto', () => ({
  randomUUID: () => randomUUID(),
  getRandomValues: <T extends ArrayBufferView>(array: T) => getRandomValues(array as never) as T,
  CryptoDigestAlgorithm: { SHA256: 'SHA-256' },
  digestStringAsync: async (_algorithm: string, data: string) => createHash('sha256').update(data).digest('hex'),
}));

const secureStore = new Map<string, string>();
vi.mock('expo-secure-store', () => ({
  setItemAsync: vi.fn(async (key: string, value: string) => {
    secureStore.set(key, value);
  }),
  getItemAsync: vi.fn(async (key: string) => secureStore.get(key) ?? null),
  deleteItemAsync: vi.fn(async (key: string) => {
    secureStore.delete(key);
  }),
}));

const asyncStorage = new Map<string, string>();
vi.mock('@react-native-async-storage/async-storage', () => ({
  default: {
    getItem: vi.fn(async (key: string) => asyncStorage.get(key) ?? null),
    setItem: vi.fn(async (key: string, value: string) => {
      asyncStorage.set(key, value);
    }),
    removeItem: vi.fn(async (key: string) => {
      asyncStorage.delete(key);
    }),
  },
}));

vi.mock('expo-constants', () => ({
  default: { expoConfig: { version: '0.1.0', hostUri: '192.168.0.10:8081' } },
}));
