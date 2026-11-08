import { describe, expect, it, vi } from 'vitest';

import { ApiError } from '../../errors';
import { HEALTH_CHECK_TIMEOUT_MS, checkApiHealth } from '../health';

// Why: "Probar conexión" is how a developer finds out why the phone cannot reach the PC. A
// check that hangs for 15 s or answers "OK" for a server that is not the Lenapay API sends
// them debugging the wrong thing; every outcome must come back fast and typed.
function json(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

describe('checkApiHealth', () => {
  it('returns the /healthz status and the round-trip time on a 200 {status:"ok"}', async () => {
    const fetchImpl = vi.fn(async (_url: string) => json(200, { status: 'ok' }));
    const clock = [1_000, 1_042];
    const result = await checkApiHealth('http://192.168.68.53:18080', { fetchImpl: fetchImpl as unknown as typeof fetch, now: () => clock.shift() ?? 0 });
    expect(result).toEqual({ status: 'ok', latencyMs: 42 });
    expect(fetchImpl.mock.calls[0]?.[0]).toBe('http://192.168.68.53:18080/healthz');
  });

  it('gives up after 5 s with network.timeout instead of hanging', async () => {
    vi.useFakeTimers();
    try {
      const fetchImpl = vi.fn(
        (_url: string, init?: RequestInit) =>
          new Promise<Response>((_resolve, reject) => {
            init?.signal?.addEventListener('abort', () => reject(new Error('Aborted')));
          }),
      );
      const pending = checkApiHealth('http://192.168.68.53:18080', { fetchImpl: fetchImpl as unknown as typeof fetch });
      const assertion = expect(pending).rejects.toMatchObject({ code: 'network.timeout' });
      await vi.advanceTimersByTimeAsync(HEALTH_CHECK_TIMEOUT_MS - 1);
      expect(fetchImpl.mock.calls[0]?.[1]?.signal?.aborted).toBe(false);
      await vi.advanceTimersByTimeAsync(1);
      await assertion;
    } finally {
      vi.useRealTimers();
    }
  });

  it('keeps the timeout at 5 s', () => {
    expect(HEALTH_CHECK_TIMEOUT_MS).toBe(5_000);
  });

  it('reports an unreachable host as network.offline', async () => {
    const fetchImpl = vi.fn(async () => {
      throw new TypeError('Network request failed');
    });
    await expect(checkApiHealth('http://10.0.0.9:18080', { fetchImpl })).rejects.toMatchObject({ code: 'network.offline', status: 0 });
  });

  it('rejects an invalid URL without touching the network', async () => {
    const fetchImpl = vi.fn(async () => json(200, { status: 'ok' }));
    await expect(checkApiHealth('localhost:18080', { fetchImpl })).rejects.toMatchObject({ code: 'health.invalid_url', status: 0 });
    expect(fetchImpl).not.toHaveBeenCalled();
  });

  it('carries the HTTP status of a non-200 answer', async () => {
    const fetchImpl = vi.fn(async () => json(503, { error: { code: 'provider.unavailable', message: 'db down' } }));
    await expect(checkApiHealth('http://192.168.68.53:18080', { fetchImpl })).rejects.toMatchObject({ code: 'provider.unavailable', status: 503 });
  });

  it('flags a 200 that is not the Lenapay health answer', async () => {
    const fetchImpl = vi.fn(async () => json(200, { hello: 'world' }));
    const error = await checkApiHealth('http://192.168.68.53:8081', { fetchImpl }).catch((caught: unknown) => caught);
    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({ code: 'health.unexpected_body', status: 200 });
  });

  it('flags a 200 HTML page (Metro, a router) as not the Lenapay API', async () => {
    const fetchImpl = vi.fn(async () => new Response('<html>Metro</html>', { status: 200 }));
    await expect(checkApiHealth('http://192.168.68.53:8081', { fetchImpl })).rejects.toMatchObject({ code: 'health.unexpected_body', status: 200 });
  });
});
