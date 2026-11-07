import { ApiError, isApiErrorBody } from '../errors';

/**
 * Fetch wrapper of the live client: base URL, app version header (M13), bearer token,
 * single-flight refresh on 401, request timeout (M6) and contract error codes.
 */
export interface HttpDeps {
  baseUrl: string;
  appVersion: string;
  timeoutMs: number;
  getAccessToken(): string | null;
  /** Rotates the session and returns the new access token, or null when it cannot. */
  refreshSession(): Promise<string | null>;
  onSessionExpired(): void;
  fetchImpl?: typeof fetch;
}

interface RequestOptions {
  body?: unknown;
  query?: Record<string, string | undefined | null>;
  headers?: Record<string, string>;
  /** false for login/signup/refresh: no bearer, no refresh on 401. */
  auth?: boolean;
}

export type Http = <T>(method: string, path: string, options?: RequestOptions) => Promise<T>;

function buildUrl(baseUrl: string, path: string, query: RequestOptions['query']): string {
  const params = Object.entries(query ?? {}).filter((entry): entry is [string, string] => entry[1] !== undefined && entry[1] !== null && entry[1] !== '');
  const search = params.map(([key, value]) => `${encodeURIComponent(key)}=${encodeURIComponent(value)}`).join('&');
  return `${baseUrl}${path}${search ? `?${search}` : ''}`;
}

export function createHttp(deps: HttpDeps): Http {
  const fetchImpl = deps.fetchImpl ?? fetch;
  let refreshing: Promise<string | null> | null = null;

  // Concurrent 401s share ONE refresh: rotating refresh tokens die if used twice.
  function refreshOnce(): Promise<string | null> {
    refreshing ??= deps.refreshSession().finally(() => {
      refreshing = null;
    });
    return refreshing;
  }

  async function send(method: string, url: string, options: RequestOptions, token: string | null): Promise<Response> {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), deps.timeoutMs);
    const isForm = typeof FormData !== 'undefined' && options.body instanceof FormData;
    const headers: Record<string, string> = { Accept: 'application/json', 'X-App-Version': deps.appVersion, ...options.headers };
    if (options.body !== undefined && !isForm) headers['Content-Type'] = 'application/json';
    if (token) headers.Authorization = `Bearer ${token}`;
    try {
      return await fetchImpl(url, {
        method,
        headers,
        body: options.body === undefined ? undefined : isForm ? (options.body as FormData) : JSON.stringify(options.body),
        signal: controller.signal,
      });
    } catch (error) {
      throw new ApiError(controller.signal.aborted ? 'network.timeout' : 'network.offline', 0, error instanceof Error ? error.message : undefined);
    } finally {
      clearTimeout(timer);
    }
  }

  return async function request<T>(method: string, path: string, options: RequestOptions = {}): Promise<T> {
    const auth = options.auth !== false;
    const url = buildUrl(deps.baseUrl, path, options.query);
    let response = await send(method, url, options, auth ? deps.getAccessToken() : null);
    if (response.status === 401 && auth) {
      const fresh = await refreshOnce();
      if (!fresh) {
        deps.onSessionExpired();
        throw new ApiError('auth.session_expired', 401);
      }
      response = await send(method, url, options, fresh);
    }
    if (!response.ok) {
      const body: unknown = await response.json().catch(() => null);
      if (isApiErrorBody(body)) throw new ApiError(body.error.code, response.status, body.error.message);
      throw new ApiError(`http.${response.status}`, response.status);
    }
    if (response.status === 204) return undefined as T;
    return (await response.json()) as T;
  };
}
