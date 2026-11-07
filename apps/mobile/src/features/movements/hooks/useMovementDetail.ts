import { type InfiniteData, useQueryClient } from '@tanstack/react-query';
import { useLocalSearchParams } from 'expo-router';

import { api } from '@/shared/api';
import type { Movement, Page } from '@/shared/api/types';
import { useShareFile } from '@/shared/mutations/useDeviceActions';
import { MOVEMENTS_KEY } from '@/shared/queries/movements';
import { useActiveOrg } from '@/shared/queries/orgs';
import { usePayin, usePayout } from '@/shared/queries/payouts';

import { findCachedMovement } from '../helpers/findCachedMovement';

export function useMovementDetail() {
  const params = useLocalSearchParams<{ id?: string }>();
  const id = typeof params.id === 'string' ? params.id : '';
  const { org } = useActiveOrg();
  const queryClient = useQueryClient();
  const cached = queryClient.getQueriesData<InfiniteData<Page<Movement>>>({ queryKey: MOVEMENTS_KEY(org?.id ?? '') }).map(([, data]) => data);
  const movement = findCachedMovement(cached, id);
  const payout = usePayout(org?.id, movement?.type === 'payout' ? movement.refId : undefined);
  const payin = usePayin(org?.id, movement?.type === 'payin' ? movement.refId : undefined);
  const shareFile = useShareFile();

  return {
    movement,
    payout,
    payin,
    canShareProof: payout.data?.status === 'settled',
    shareProof: () => {
      if (org && payout.data) {
        const payoutId = payout.data.id;
        shareFile.mutate(() => api.payouts.proof(org.id, payoutId));
      }
    },
    sharing: shareFile.isPending,
  };
}
