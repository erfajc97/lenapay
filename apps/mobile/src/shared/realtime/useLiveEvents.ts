import { type QueryClient, useQueryClient } from '@tanstack/react-query';
import { useEffect } from 'react';

import { api } from '@/shared/api';
import type { LiveEvent } from '@/shared/api/types';
import { confirmHaptic } from '@/shared/feedback/haptics';
import { ACCOUNTS_KEY, BALANCES_KEY } from '@/shared/queries/accounts';
import { MOVEMENTS_KEY } from '@/shared/queries/movements';
import { PAYERS_KEY } from '@/shared/queries/payers';
import { APPROVALS_KEY, PAYOUT_KEY } from '@/shared/queries/payouts';
import { useLivePayinStore } from '@/shared/stores/useLivePayinStore';

/** What a live event changes on screen (R-IN-3): balances, movements, approvals, the banner. */
function applyLiveEvent(queryClient: QueryClient, orgId: string, event: LiveEvent): void {
  const refresh = (queryKey: readonly unknown[]) => void queryClient.invalidateQueries({ queryKey });
  refresh(BALANCES_KEY(orgId));
  refresh(ACCOUNTS_KEY(orgId));
  refresh(MOVEMENTS_KEY(orgId));
  if (event.type === 'payin.credited') {
    refresh(PAYERS_KEY(orgId));
    useLivePayinStore.getState().show(event.payin);
    confirmHaptic();
    return;
  }
  refresh(APPROVALS_KEY(orgId));
  refresh(PAYOUT_KEY(orgId, event.payout.id));
}

/**
 * One subscription to the org's event stream (mock timer or SSE) while the app is signed in.
 * A subscription is the one legitimate effect: it has a cleanup and makes no request itself.
 */
export function useLiveEvents(orgId: string | undefined): void {
  const queryClient = useQueryClient();
  useEffect(() => {
    if (!orgId) return undefined;
    return api.events.subscribe(orgId, (event) => applyLiveEvent(queryClient, orgId, event));
  }, [orgId, queryClient]);
}
