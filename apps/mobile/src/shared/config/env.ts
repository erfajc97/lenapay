import Constants from 'expo-constants';

/**
 * The ONE module that reads environment config (M5, ME3). Everything here ships in the
 * bundle, so it is public: only EXPO_PUBLIC_* names, read with literal dot access because
 * Metro inlines only that form.
 */
type ApiMode = 'mock' | 'live';

const LIVE_API_PORT = 8080;

export function resolveApiMode(raw: string | undefined): ApiMode {
  return raw === 'live' ? 'live' : 'mock';
}

/**
 * In development a phone cannot reach `localhost` (that is the phone), so without an explicit
 * URL the API host is derived from the Metro host the app was loaded from.
 */
export function resolveApiUrl(explicit: string | undefined, metroHostUri: string | undefined): string {
  if (explicit && explicit.trim() !== '') return explicit.replace(/\/+$/, '');
  const host = metroHostUri?.split(':')[0];
  return `http://${host && host !== '' ? host : 'localhost'}:${LIVE_API_PORT}`;
}

export const config = {
  apiMode: resolveApiMode(process.env.EXPO_PUBLIC_API_MODE),
  apiUrl: resolveApiUrl(process.env.EXPO_PUBLIC_API_URL, Constants.expoConfig?.hostUri),
  appVersion: Constants.expoConfig?.version ?? '0.0.0',
  /** EAS project id for Expo push tokens; absent until the EAS project exists. */
  easProjectId: (Constants.expoConfig?.extra as { eas?: { projectId?: string } } | undefined)?.eas?.projectId ?? null,
  requestTimeoutMs: 15_000,
} as const;
