import { useInfiniteQuery, useQuery } from '@tanstack/react-query';

import { api } from '@/shared/api';

/** Root of every "Mis pagos" query: a payment or a hide/show invalidates the whole tree. */
export const MY_PAYMENTS_KEY = ['myPayments'] as const;
export const MY_PAYMENT_COMPANIES_KEY = (includeHidden: boolean) => [...MY_PAYMENTS_KEY, 'companies', includeHidden] as const;
export const MY_PAYMENT_PAYINS_KEY = (linkId: string) => [...MY_PAYMENTS_KEY, 'payins', linkId] as const;
export const MY_PAYMENT_INSTALLMENTS_KEY = (linkId: string) => [...MY_PAYMENTS_KEY, 'installments', linkId] as const;

/** Runs only for a person with a verified personal wallet (`enabled`), never as a 403 probe. */
export function useMyPaymentCompanies(enabled: boolean, includeHidden: boolean) {
  return useQuery({
    queryKey: MY_PAYMENT_COMPANIES_KEY(includeHidden),
    queryFn: () => api.myPayments.companies({ includeHidden }),
    enabled,
  });
}

export function useMyPaymentPayins(linkId: string | undefined) {
  return useInfiniteQuery({
    queryKey: MY_PAYMENT_PAYINS_KEY(linkId ?? ''),
    queryFn: ({ pageParam }) => api.myPayments.payins(linkId ?? '', pageParam),
    initialPageParam: null as string | null,
    getNextPageParam: (lastPage) => lastPage.nextCursor,
    enabled: Boolean(linkId),
  });
}

export function useMyPaymentInstallments(linkId: string | undefined) {
  return useQuery({
    queryKey: MY_PAYMENT_INSTALLMENTS_KEY(linkId ?? ''),
    queryFn: () => api.myPayments.installments(linkId ?? ''),
    enabled: Boolean(linkId),
  });
}
