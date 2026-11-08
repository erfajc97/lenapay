import Constants from 'expo-constants';

/**
 * The ONE module that reads environment config (M5, ME3). Everything here ships in the
 * bundle, so it is public: only EXPO_PUBLIC_* names, read with literal dot access because
 * Metro inlines only that form.
 */
export type ApiMode = 'mock' | 'live';

/** Host port of the API in the local stack (docker compose maps 18080 → 8080). */
const LIVE_API_PORT = 18080;

export function resolveApiMode(raw: string | undefined): ApiMode {
  return raw === 'live' ? 'live' : 'mock';
}

/**
 * In development a phone cannot reach `localhost` (that is the phone), so without an explicit
 * URL the API host is derived from the Metro host the app was loaded from.
 */
export function resolveApiUrl(explicit: string | undefined, metroHostUri: string | undefined): string {
  if (explicit && explicit.trim() !== '') return explicit.trim().replace(/\/+$/, '');
  const host = metroHostUri?.split(':')[0];
  return `http://${host && host !== '' ? host : 'localhost'}:${LIVE_API_PORT}`;
}

export interface ApiTarget {
  mode: ApiMode;
  url: string;
}

/**
 * The API the app talks to: the build's env, unless a development build saved an override in
 * Settings → Desarrollador. A release build ignores any saved override, so a store build can
 * never be pointed at another server from the device.
 */
export function resolveApiTarget(params: { env: ApiTarget; override: ApiTarget | null; devTools: boolean }): ApiTarget {
  if (!params.devTools || !params.override) return params.env;
  const url = params.override.url.trim();
  return { mode: params.override.mode, url: url === '' ? params.env.url : resolveApiUrl(url, undefined) };
}

/** A URL the developer screen accepts: http(s) with a host, nothing else. */
export function isValidApiUrl(text: string): boolean {
  return /^https?:\/\/[^\s/?#]+(:\d+)?(\/[^\s?#]*)?$/.test(text.trim());
}

export const config = {
  env: {
    mode: resolveApiMode(process.env.EXPO_PUBLIC_API_MODE),
    url: resolveApiUrl(process.env.EXPO_PUBLIC_API_URL, Constants.expoConfig?.hostUri),
  } satisfies ApiTarget,
  /** Settings → Desarrollador exists only in development builds. */
  devTools: typeof __DEV__ !== 'undefined' && __DEV__,
  appVersion: Constants.expoConfig?.version ?? '0.0.0',
  /** EAS project id for Expo push tokens; absent until the EAS project exists. */
  easProjectId: (Constants.expoConfig?.extra as { eas?: { projectId?: string } } | undefined)?.eas?.projectId ?? null,
  requestTimeoutMs: 15_000,
} as const;
