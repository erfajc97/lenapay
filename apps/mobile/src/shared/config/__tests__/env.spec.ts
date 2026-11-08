import { describe, expect, it } from 'vitest';

import { ApiError } from '@/shared/api/errors';
import { shouldRetry } from '@/shared/queries/queryClient';

import { isValidApiUrl, resolveApiMode, resolveApiTarget, resolveApiUrl } from '../env';

// Why: a typo in EXPO_PUBLIC_API_MODE must never point a demo at a real API, and on a phone
// `localhost` is the phone itself (ME3).
describe('config', () => {
  it('uses live only when asked explicitly, mock otherwise', () => {
    expect(resolveApiMode('live')).toBe('live');
    expect(resolveApiMode('mock')).toBe('mock');
    expect(resolveApiMode(undefined)).toBe('mock');
    expect(resolveApiMode('LIVE')).toBe('mock');
  });

  it('prefers the explicit URL and otherwise derives it from the Metro host', () => {
    expect(resolveApiUrl('https://api.lenapay.app/', '192.168.0.10:8081')).toBe('https://api.lenapay.app');
    expect(resolveApiUrl('', '192.168.0.10:8081')).toBe('http://192.168.0.10:18080');
    expect(resolveApiUrl('  ', '192.168.68.53:8081')).toBe('http://192.168.68.53:18080');
    expect(resolveApiUrl(undefined, undefined)).toBe('http://localhost:18080');
  });
});

// Why: Settings → Desarrollador lets a phone point at another API; a store build must never
// honour a saved override, and a blank URL must keep the build's (Metro-derived) address.
describe('resolveApiTarget', () => {
  const env = { mode: 'mock' as const, url: 'http://192.168.68.53:18080' };
  const override = { mode: 'live' as const, url: 'http://10.0.0.5:18080/' };

  it('uses the build env without an override or outside development builds', () => {
    expect(resolveApiTarget({ env, override: null, devTools: true })).toEqual(env);
    expect(resolveApiTarget({ env, override, devTools: false })).toEqual(env);
  });

  it('applies a development override, trimming the trailing slash', () => {
    expect(resolveApiTarget({ env, override, devTools: true })).toEqual({ mode: 'live', url: 'http://10.0.0.5:18080' });
  });

  it('keeps the build URL when the override leaves it blank', () => {
    expect(resolveApiTarget({ env, override: { mode: 'live', url: ' ' }, devTools: true })).toEqual({ mode: 'live', url: env.url });
  });
});

describe('isValidApiUrl', () => {
  it('accepts http(s) URLs with a host and an optional port or path', () => {
    expect(isValidApiUrl('http://192.168.68.53:18080')).toBe(true);
    expect(isValidApiUrl('https://api.lenapay.app/v1')).toBe(true);
  });

  it('rejects other schemes, missing hosts and spaces', () => {
    expect(isValidApiUrl('192.168.68.53:18080')).toBe(false);
    expect(isValidApiUrl('ftp://host')).toBe(false);
    expect(isValidApiUrl('http://')).toBe(false);
    expect(isValidApiUrl('http://my host')).toBe(false);
  });
});

describe('shouldRetry', () => {
  it('never retries a 4xx answer and retries other failures twice', () => {
    expect(shouldRetry(0, new ApiError('payout.insufficient_funds', 422))).toBe(false);
    expect(shouldRetry(0, new ApiError('network.offline', 0))).toBe(true);
    expect(shouldRetry(1, new ApiError('http.503', 503))).toBe(true);
    expect(shouldRetry(2, new ApiError('http.503', 503))).toBe(false);
  });
});
