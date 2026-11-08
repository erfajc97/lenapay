import { describe, expect, it, vi } from 'vitest';

import type { Payin, Payout } from '@/shared/api/types';
import { ACCOUNTS_KEY, BALANCES_KEY } from '@/shared/queries/accounts';
import { MOVEMENTS_KEY } from '@/shared/queries/movements';
import { PAYERS_KEY } from '@/shared/queries/payers';
import { APPROVALS_KEY, PAYOUT_KEY } from '@/shared/queries/payouts';

import { liveEventEffects } from '../liveEventEffects';

// The query modules export their keys next to hooks that import the API client (native SSE).
vi.mock('@/shared/api', () => ({ api: {} }));

// Why: a live event that does not refresh the right screen leaves a stale balance next to a
// banner that says the money arrived, or an approval that already happened still waiting.
const ORG = 'org-1';
const money = [BALANCES_KEY(ORG), ACCOUNTS_KEY(ORG), MOVEMENTS_KEY(ORG)];
const payin = { id: 'pin-1' } as Payin;
const payout = { id: 'po-1' } as Payout;

describe('liveEventEffects', () => {
  it('payin.credited refreshes money and payers and shows the banner', () => {
    expect(liveEventEffects(ORG, { type: 'payin.credited', payin })).toEqual({ invalidate: [...money, PAYERS_KEY(ORG)], banner: payin });
  });

  it('payout.updated and approval.requested refresh money, approvals and that payout', () => {
    for (const type of ['payout.updated', 'approval.requested'] as const) {
      expect(liveEventEffects(ORG, { type, payout })).toEqual({ invalidate: [...money, APPROVALS_KEY(ORG), PAYOUT_KEY(ORG, 'po-1')], banner: null });
    }
  });

  it('balance.changed refreshes only money', () => {
    expect(liveEventEffects(ORG, { type: 'balance.changed', accountId: 'acc-1' })).toEqual({ invalidate: money, banner: null });
  });

  it('a reconnect refetches everything a missed frame could have changed', () => {
    expect(liveEventEffects(ORG, { type: 'resync' })).toEqual({ invalidate: [...money, APPROVALS_KEY(ORG), PAYERS_KEY(ORG)], banner: null });
  });
});
