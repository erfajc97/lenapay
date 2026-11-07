import { useQuery } from '@tanstack/react-query';

import { api } from '@/shared/api';

export const ACCOUNTS_KEY = (orgId: string) => ['accounts', orgId] as const;
export const BALANCES_KEY = (orgId: string) => ['balances', orgId] as const;

export function useAccounts(orgId: string | undefined) {
  return useQuery({ queryKey: ACCOUNTS_KEY(orgId ?? ''), queryFn: () => api.accounts.list(orgId ?? ''), enabled: Boolean(orgId) });
}

export function useBalances(orgId: string | undefined) {
  return useQuery({ queryKey: BALANCES_KEY(orgId ?? ''), queryFn: () => api.accounts.balances(orgId ?? ''), enabled: Boolean(orgId) });
}
