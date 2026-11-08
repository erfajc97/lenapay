import AsyncStorage from '@react-native-async-storage/async-storage';
import { createAsyncStoragePersister } from '@tanstack/query-async-storage-persister';
import { QueryClient } from '@tanstack/react-query';

import { ApiError } from '@/shared/api/errors';
import { config } from '@/shared/config/env';

/** A 4xx is an answer, not a glitch: retrying it only delays the error state. */
export function shouldRetry(failureCount: number, error: unknown): boolean {
  if (error instanceof ApiError && error.status >= 400 && error.status < 500) return false;
  return failureCount < 2;
}

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: { staleTime: 30_000, gcTime: 24 * 60 * 60 * 1000, retry: shouldRetry },
    mutations: { retry: false },
  },
});

/**
 * Cached server data survives a restart (no secrets in it). The build's API mode and the
 * view-model version bust it; a runtime switch in Settings → Desarrollador clears it.
 */
export const queryPersister = createAsyncStoragePersister({ storage: AsyncStorage, key: 'lenapay.queryCache' });
export const PERSIST_BUSTER = `${config.env.mode}-2`;
