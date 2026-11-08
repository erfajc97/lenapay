import { config } from '@/shared/config/env';

import { endpoints } from '../endpoints';
import { createHttp } from './http';

/**
 * "Probar conexión" in Settings → Desarrollador: GET /healthz on a candidate base URL, through
 * the same client (timeout, version header, error codes) the live API will use.
 */
export async function checkApiHealth(baseUrl: string, fetchImpl?: typeof fetch): Promise<void> {
  const http = createHttp({
    baseUrl,
    appVersion: config.appVersion,
    timeoutMs: config.requestTimeoutMs,
    getAccessToken: () => null,
    refreshSession: async () => null,
    onSessionExpired: () => undefined,
    fetchImpl,
  });
  await http('GET', endpoints.health, { auth: false });
}
