import { useMutation, useQueryClient } from '@tanstack/react-query';

import { api } from '@/shared/api';
import type { Currency } from '@/shared/api/types';
import { confirmHaptic } from '@/shared/feedback/haptics';
import { notifyError, notifySuccess } from '@/shared/feedback/mutationFeedback';
import { PAYERS_KEY } from '@/shared/queries/payers';
import { runMoneyAction } from '@/shared/stepup/moneyAction';

export function useIssueCvu(orgId: string | undefined, payerId: string | undefined) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (currency: Currency) =>
      runMoneyAction({
        scope: `cvu:${payerId ?? ''}:${currency}`,
        payload: { currency },
        call: (options) => api.payers.issueCvu(orgId ?? '', payerId ?? '', { currency }, options),
      }),
    onSuccess: (cvu) => {
      void queryClient.invalidateQueries({ queryKey: PAYERS_KEY(orgId ?? '') });
      confirmHaptic();
      notifySuccess('payers.cvuIssued', { currency: cvu.currency });
    },
    onError: notifyError,
  });
}

export function useUpdateAlias(orgId: string | undefined, onDone: () => void) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: { cvuId: string; alias: string }) => api.payers.updateCvu(orgId ?? '', input.cvuId, { alias: input.alias.trim().toLowerCase() }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: PAYERS_KEY(orgId ?? '') });
      notifySuccess('payers.aliasUpdated');
      onDone();
    },
    onError: notifyError,
  });
}

export function useDisableCvu(orgId: string | undefined, onDone: () => void) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (cvuId: string) => api.payers.disableCvu(orgId ?? '', cvuId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: PAYERS_KEY(orgId ?? '') });
      notifySuccess('payers.cvuDisabled');
      onDone();
    },
    onError: notifyError,
  });
}
