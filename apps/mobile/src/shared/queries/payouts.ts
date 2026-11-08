import { useQuery } from '@tanstack/react-query';

import { api } from '@/shared/api';
import type { PayoutQuoteRequest } from '@/shared/api/types';

export const APPROVALS_KEY = (orgId: string) => ['approvals', orgId] as const;
export const PAYOUT_KEY = (orgId: string, payoutId: string) => ['payout', orgId, payoutId] as const;
export const PAYIN_KEY = (orgId: string, payinId: string) => ['payin', orgId, payinId] as const;

export function useApprovals(orgId: string | undefined) {
  return useQuery({ queryKey: APPROVALS_KEY(orgId ?? ''), queryFn: () => api.payouts.approvals(orgId ?? ''), enabled: Boolean(orgId) });
}

export function usePayout(orgId: string | undefined, payoutId: string | undefined) {
  return useQuery({
    queryKey: PAYOUT_KEY(orgId ?? '', payoutId ?? ''),
    queryFn: () => api.payouts.get(orgId ?? '', payoutId ?? ''),
    enabled: Boolean(orgId && payoutId),
  });
}

export function usePayin(orgId: string | undefined, payinId: string | undefined) {
  return useQuery({
    queryKey: PAYIN_KEY(orgId ?? '', payinId ?? ''),
    queryFn: () => api.payins.get(orgId ?? '', payinId ?? ''),
    enabled: Boolean(orgId && payinId),
  });
}

export const DESTINATION_KEY = (orgId: string, to: string) => ['destination', orgId, to] as const;
export const QUOTE_KEY = (orgId: string, request: PayoutQuoteRequest) => ['quote', orgId, request] as const;

/** Resolve CBU/CVU/alias at the PSP before paying (R-OUT-1); runs only once a value is submitted. */
export function useResolveDestination(orgId: string | undefined, to: string | null) {
  return useQuery({
    queryKey: DESTINATION_KEY(orgId ?? '', to ?? ''),
    queryFn: () => api.payouts.resolveDestination(orgId ?? '', to ?? ''),
    enabled: Boolean(orgId && to),
    staleTime: 60_000,
  });
}

const NO_QUOTE: PayoutQuoteRequest = { accountId: '', currency: 'ARS', amount: 0, to: '' };

/** Fee and total before confirming (R-OUT-2). */
export function usePayoutQuote(orgId: string | undefined, request: PayoutQuoteRequest | null) {
  return useQuery({
    queryKey: QUOTE_KEY(orgId ?? '', request ?? NO_QUOTE),
    queryFn: () => api.payouts.quote(orgId ?? '', request ?? NO_QUOTE),
    enabled: Boolean(orgId && request),
    staleTime: 0,
  });
}
