import { type QueryClient, useQueryClient } from '@tanstack/react-query';
import { useEffect } from 'react';

import { api, useApiTarget } from '@/shared/api';
import type { LiveEvent } from '@/shared/api/types';
import { confirmHaptic } from '@/shared/feedback/haptics';
import { useLivePayinStore } from '@/shared/stores/useLivePayinStore';

import { liveEventEffects } from './liveEventEffects';

/** Applies one event: refetch what it made stale, announce a payin (R-IN-3). */
function applyLiveEvent(queryClient: QueryClient, orgId: string, event: LiveEvent): void {
  const effects = liveEventEffects(orgId, event);
  for (const queryKey of effects.invalidate) void queryClient.invalidateQueries({ queryKey });
  if (effects.banner) {
    useLivePayinStore.getState().show(effects.banner);
    confirmHaptic();
  }
}

/**
 * One subscription to the org's event stream (mock timer or SSE) while the app is signed in;
 * a switch of API target in Settings → Desarrollador reopens it against the new API.
 * A subscription is the one legitimate effect: it has a cleanup and makes no request itself.
 */
export function useLiveEvents(orgId: string | undefined): void {
  const queryClient = useQueryClient();
  const target = useApiTarget();
  useEffect(() => {
    if (!orgId) return undefined;
    return api.events.subscribe(orgId, (event) => applyLiveEvent(queryClient, orgId, event));
  }, [orgId, queryClient, target.mode, target.url]);
}
