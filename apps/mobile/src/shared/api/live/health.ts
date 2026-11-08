import { config, isValidApiUrl } from '@/shared/config/env';

import { endpoints } from '../endpoints';
import { ApiError } from '../errors';
import { createHttp } from './http';

/** "Probar conexión" answers within this time, whatever the network does (never hangs). */
export const HEALTH_CHECK_TIMEOUT_MS = 5_000;

export interface HealthCheckResult {
  /** The `status` of GET /healthz (the contract's only value is `ok`). */
  status: 'ok';
  latencyMs: number;
}

/**
 * "Probar conexión" in Settings → Desarrollador: GET /healthz on a candidate base URL, through
 * the same client (version header, error codes) the live API will use, with a short timeout.
 * Failures are ApiErrors: `health.invalid_url`, `network.timeout`, `network.offline`, the
 * API's own code or `http.<status>` for a non-200, and `health.unexpected_body` for a 200 that
 * is not the Lenapay API (another server on that port).
 */
export async function checkApiHealth(baseUrl: string, options: { fetchImpl?: typeof fetch; now?: () => number } = {}): Promise<HealthCheckResult> {
  if (!isValidApiUrl(baseUrl)) throw new ApiError('health.invalid_url', 0);
  const now = options.now ?? Date.now;
  const http = createHttp({
    baseUrl,
    appVersion: config.appVersion,
    timeoutMs: HEALTH_CHECK_TIMEOUT_MS,
    getAccessToken: () => null,
    refreshSession: async () => null,
    onSessionExpired: () => undefined,
    fetchImpl: options.fetchImpl,
  });
  const startedAt = now();
  // A 200 whose body is not JSON (Metro, a router page) makes the client's parse throw.
  const body = await http<unknown>('GET', endpoints.health, { auth: false }).catch((error: unknown) => {
    if (error instanceof ApiError) throw error;
    throw new ApiError('health.unexpected_body', 200);
  });
  const latencyMs = now() - startedAt;
  if (typeof body !== 'object' || body === null || (body as { status?: unknown }).status !== 'ok') throw new ApiError('health.unexpected_body', 200);
  return { status: 'ok', latencyMs };
}
