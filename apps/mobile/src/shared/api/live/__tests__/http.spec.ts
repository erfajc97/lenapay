import { describe, expect, it, vi } from 'vitest';

import { ApiError } from '../../errors';
import { createHttp, type HttpDeps } from '../http';

// Why: with rotating refresh tokens, two parallel 401s that each refresh kill the session;
// a server message shown raw leaks internals; a missing version header breaks the gate (M13).
function json(status: number, body: unknown): Response {
  return new Response(body === undefined ? null : JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

function deps(over: Partial<HttpDeps>): HttpDeps {
  return {
    baseUrl: 'https://api.test',
    appVersion: '0.1.0',
    timeoutMs: 1_000,
    getAccessToken: () => 'old',
    refreshSession: async () => 'new',
    onSessionExpired: vi.fn(),
    ...over,
  };
}

describe('createHttp', () => {
  it('sends bearer, app version and money headers, and drops empty query params', async () => {
    const fetchImpl = vi.fn(async () => json(200, { ok: true }));
    const http = createHttp(deps({ fetchImpl }));
    await http('POST', '/orgs/o/payouts', { body: { amount: 1 }, query: { a: 'x', b: undefined, c: '' }, headers: { 'Idempotency-Key': 'k', 'X-Step-Up-Token': 's' } });
    const [url, init] = fetchImpl.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe('https://api.test/orgs/o/payouts?a=x');
    expect(init.headers).toMatchObject({ Authorization: 'Bearer old', 'X-App-Version': '0.1.0', 'Idempotency-Key': 'k', 'X-Step-Up-Token': 's' });
  });

  it('refreshes once for parallel 401s and retries with the new token', async () => {
    const refreshSession = vi.fn(async () => 'new');
    const fetchImpl = vi.fn(async (_url: string, init?: RequestInit) =>
      (init?.headers as Record<string, string>).Authorization === 'Bearer new' ? json(200, { ok: true }) : json(401, undefined),
    );
    const http = createHttp(deps({ fetchImpl: fetchImpl as unknown as typeof fetch, refreshSession }));
    await Promise.all([http('GET', '/me'), http('GET', '/orgs')]);
    expect(refreshSession).toHaveBeenCalledTimes(1);
  });

  it('expires the session when the refresh fails', async () => {
    const onSessionExpired = vi.fn();
    const http = createHttp(deps({ fetchImpl: vi.fn(async () => json(401, undefined)), refreshSession: async () => null, onSessionExpired }));
    await expect(http('GET', '/me')).rejects.toMatchObject({ code: 'auth.session_expired' });
    expect(onSessionExpired).toHaveBeenCalledTimes(1);
  });

  it('maps the contract error body to its code and never retries unauthenticated calls', async () => {
    const refreshSession = vi.fn(async () => 'new');
    const http = createHttp(deps({ fetchImpl: vi.fn(async () => json(401, { error: { code: 'auth.invalid_credentials', message: 'bad' } })), refreshSession }));
    const error = await http('POST', '/auth/login', { auth: false }).catch((caught: unknown) => caught);
    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({ code: 'auth.invalid_credentials', status: 401 });
    expect(refreshSession).not.toHaveBeenCalled();
  });

  it('turns a network failure into network.offline', async () => {
    const http = createHttp(deps({ fetchImpl: vi.fn(async () => Promise.reject(new TypeError('Network request failed'))) }));
    await expect(http('GET', '/me')).rejects.toMatchObject({ code: 'network.offline' });
  });
});
