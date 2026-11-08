import { type ApiTarget, config, resolveApiTarget } from '@/shared/config/env';
import { useApiTargetStore } from '@/shared/stores/useApiTargetStore';

import type { LenapayApi } from './client';
import { createLiveApi } from './live/liveApi';
import { MOCK_EMAIL_CODE } from './mock/backend';
import { createMockApi } from './mock/mockApi';

/**
 * The app's API: `mock` (in-memory fixtures) or `live` (the Lenapay API), picked by the build's
 * EXPO_PUBLIC_API_MODE / EXPO_PUBLIC_API_URL or, in development builds, by Settings →
 * Desarrollador. Features import `api` and never know which one answers.
 */
function currentApiTarget(): ApiTarget {
  return resolveApiTarget({ env: config.env, override: useApiTargetStore.getState().override, devTools: config.devTools });
}

export function useApiTarget(): ApiTarget {
  const override = useApiTargetStore((state) => state.override);
  return resolveApiTarget({ env: config.env, override, devTools: config.devTools });
}

export function isMockApi(): boolean {
  return currentApiTarget().mode === 'mock';
}

export function useIsMockApi(): boolean {
  return useApiTarget().mode === 'mock';
}

/** The email code the mock backend accepts (shown as a demo hint); null in live mode. */
export function useDemoEmailCode(): string | null {
  return useIsMockApi() ? MOCK_EMAIL_CODE : null;
}

let mockApi: LenapayApi | null = null;
const liveApis = new Map<string, LenapayApi>();

function implementationFor(target: ApiTarget): LenapayApi {
  if (target.mode === 'mock') {
    mockApi ??= createMockApi();
    return mockApi;
  }
  let live = liveApis.get(target.url);
  if (!live) {
    live = createLiveApi({ baseUrl: target.url });
    liveApis.set(target.url, live);
  }
  return live;
}

type Namespaces = Record<string, Record<string, (...args: unknown[]) => unknown>>;

/** Every call is resolved at call time, so a switch in Settings applies to the next request. */
function forwardTo(pick: () => LenapayApi): LenapayApi {
  return new Proxy({} as LenapayApi, {
    get: (_target, namespace: string) =>
      new Proxy(
        {},
        {
          get: (_inner, method: string) =>
            (...args: unknown[]) => {
              const call = (pick() as unknown as Namespaces)[namespace]?.[method];
              if (!call) throw new TypeError(`api.${namespace}.${method} does not exist`);
              return call(...args);
            },
        },
      ),
  });
}

export const api: LenapayApi = forwardTo(() => implementationFor(currentApiTarget()));
