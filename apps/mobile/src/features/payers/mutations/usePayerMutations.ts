import { useMutation, useQueryClient } from '@tanstack/react-query';
import { router } from 'expo-router';

import { api } from '@/shared/api';
import type { CreatePayerRequest, Currency } from '@/shared/api/types';
import { confirmHaptic } from '@/shared/feedback/haptics';
import { notifyError, notifySuccess } from '@/shared/feedback/mutationFeedback';
import { ACCOUNTS_KEY } from '@/shared/queries/accounts';
import { PAYERS_KEY } from '@/shared/queries/payers';
import { runMoneyAction } from '@/shared/stepup/moneyAction';
import { requestStepUp } from '@/shared/stores/useStepUpStore';

/**
 * Create the payer, then issue one CVU per chosen currency (R-CVU-1) behind ONE step-up.
 * If the CVU step is cancelled the payer stays, and its detail screen offers the CVU again.
 */
export function useCreatePayer(orgId: string | undefined) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (input: { body: CreatePayerRequest; currencies: Currency[] }) => {
      const stepUpToken = await requestStepUp();
      const payer = await api.payers.create(orgId ?? '', input.body);
      for (const currency of input.currencies) {
        await runMoneyAction({
          scope: `cvu:${payer.id}:${currency}`,
          payload: { currency },
          stepUp: async () => stepUpToken,
          call: (options) => api.payers.issueCvu(orgId ?? '', payer.id, { currency }, options),
        });
      }
      return payer;
    },
    onSuccess: (payer) => {
      void queryClient.invalidateQueries({ queryKey: PAYERS_KEY(orgId ?? '') });
      void queryClient.invalidateQueries({ queryKey: ACCOUNTS_KEY(orgId ?? '') });
      confirmHaptic();
      notifySuccess('payers.created', { name: payer.name });
      router.replace({ pathname: '/payers/[id]', params: { id: payer.id } });
    },
    onError: notifyError,
  });
}

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
