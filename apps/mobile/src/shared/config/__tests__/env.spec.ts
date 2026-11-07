import { describe, expect, it } from 'vitest';

import { ApiError } from '@/shared/api/errors';
import { shouldRetry } from '@/shared/queries/queryClient';

import { resolveApiMode, resolveApiUrl } from '../env';

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
    expect(resolveApiUrl('', '192.168.0.10:8081')).toBe('http://192.168.0.10:8080');
    expect(resolveApiUrl(undefined, undefined)).toBe('http://localhost:8080');
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
