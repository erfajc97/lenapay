import type { Movement, MovementFilters, MovementStatus, PayoutStatus } from '../types';
import type { MockState } from './state';

/** Movements read model of the mock (R-MOV-1): payins, payouts and both legs of transfers. */
const PAYOUT_TO_MOVEMENT: Record<PayoutStatus, MovementStatus> = {
  pending_approval: 'pending',
  approved: 'pending',
  sending: 'pending',
  settled: 'completed',
  failed: 'failed',
  rejected: 'cancelled',
  cancelled: 'cancelled',
};

function allMovements(state: MockState, orgId: string): Movement[] {
  const accountName = (id: string) => state.accounts.find((item) => item.id === id)?.name ?? '';
  const payins: Movement[] = state.payins
    .filter((item) => item.orgId === orgId)
    .map((item) => ({
      id: `mv_${item.id}`, type: 'payin', direction: 'in', status: 'completed', currency: item.currency, amount: item.net, fee: item.fee,
      accountId: item.accountId, accountName: item.accountName, counterparty: item.originName, counterpartyDoc: item.originCuit,
      concept: null, refId: item.id, rail: item.rail, createdAt: item.createdAt,
    }));
  const payouts: Movement[] = state.payouts
    .filter((item) => item.orgId === orgId)
    .map((item) => ({
      id: `mv_${item.id}`, type: 'payout', direction: 'out', status: PAYOUT_TO_MOVEMENT[item.status], currency: item.currency,
      amount: item.total, fee: item.fee, accountId: item.accountId, accountName: item.accountName,
      counterparty: item.destination.holderName, counterpartyDoc: item.destination.holderCuit, concept: item.concept, refId: item.id, rail: item.rail, createdAt: item.createdAt,
    }));
  const transfers: Movement[] = state.transfers
    .filter((item) => item.orgId === orgId)
    .flatMap((item) => [
      {
        id: `mv_${item.id}_out`, type: 'internal_transfer' as const, direction: 'out' as const, status: 'completed' as const, currency: item.currency, amount: item.amount, fee: null,
        accountId: item.fromAccountId, accountName: accountName(item.fromAccountId), counterparty: accountName(item.toAccountId), counterpartyDoc: null,
        concept: item.concept, refId: item.id, rail: 'internal' as const, createdAt: item.createdAt,
      },
      {
        id: `mv_${item.id}_in`, type: 'internal_transfer' as const, direction: 'in' as const, status: 'completed' as const, currency: item.currency, amount: item.amount, fee: null,
        accountId: item.toAccountId, accountName: accountName(item.toAccountId), counterparty: accountName(item.fromAccountId), counterpartyDoc: null,
        concept: item.concept, refId: item.id, rail: 'internal' as const, createdAt: item.createdAt,
      },
    ]);
  return [...payins, ...payouts, ...transfers];
}

export function buildMovements(state: MockState, orgId: string, filters: MovementFilters): Movement[] {
  return allMovements(state, orgId)
    .filter((item) => !filters.account || item.accountId === filters.account)
    .filter((item) => !filters.type || item.type === filters.type)
    .filter((item) => !filters.status || item.status === filters.status)
    .filter((item) => !filters.currency || item.currency === filters.currency)
    // `from`/`to` are inclusive business dates (YYYY-MM-DD); the mock's business day is UTC.
    .filter((item) => !filters.from || item.createdAt.slice(0, 10) >= filters.from)
    .filter((item) => !filters.to || item.createdAt.slice(0, 10) <= filters.to)
    .sort((a, b) => b.createdAt.localeCompare(a.createdAt));
}
