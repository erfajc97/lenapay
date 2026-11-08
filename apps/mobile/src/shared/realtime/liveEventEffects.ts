import type { LiveEvent, Payin } from '@/shared/api/types';
import { ACCOUNTS_KEY, BALANCES_KEY } from '@/shared/queries/accounts';
import { MOVEMENTS_KEY } from '@/shared/queries/movements';
import { PAYERS_KEY } from '@/shared/queries/payers';
import { APPROVALS_KEY, PAYOUT_KEY } from '@/shared/queries/payouts';

export interface LiveEventEffects {
  /** Query keys whose data the event made stale. */
  invalidate: (readonly unknown[])[];
  /** A payin to announce in the banner (R-IN-3). */
  banner: Payin | null;
}

/**
 * What one realtime event changes on screen (R-IN-3, R-OUT-4): which queries refetch and
 * whether the "cobro recibido" banner shows. Every event that moves money refreshes balances
 * and movements; approvals refresh when a payout appears or changes state.
 */
export function liveEventEffects(orgId: string, event: LiveEvent): LiveEventEffects {
  const money = [BALANCES_KEY(orgId), ACCOUNTS_KEY(orgId), MOVEMENTS_KEY(orgId)];
  switch (event.type) {
    case 'payin.credited':
      return { invalidate: [...money, PAYERS_KEY(orgId)], banner: event.payin };
    case 'payout.updated':
    case 'approval.requested':
      return { invalidate: [...money, APPROVALS_KEY(orgId), PAYOUT_KEY(orgId, event.payout.id)], banner: null };
    case 'balance.changed':
      return { invalidate: money, banner: null };
    case 'resync':
      return { invalidate: [...money, APPROVALS_KEY(orgId), PAYERS_KEY(orgId)], banner: null };
  }
}
