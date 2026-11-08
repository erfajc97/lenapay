import { useQueryClient } from '@tanstack/react-query';
import { router } from 'expo-router';

import { ACCOUNTS_KEY, BALANCES_KEY } from '@/shared/queries/accounts';
import { ORGS_KEY, useActiveOrg, useKyc } from '@/shared/queries/orgs';

import { kycTimeline } from '../helpers/kycTimeline';

export function useKycStatus() {
  const queryClient = useQueryClient();
  const { org } = useActiveOrg();
  const kyc = useKyc(org?.id, { pollWhileSubmitted: true });
  return {
    org,
    kyc,
    timeline: kycTimeline(kyc.data),
    goHome: () => {
      void queryClient.invalidateQueries({ queryKey: ORGS_KEY });
      void queryClient.invalidateQueries({ queryKey: ACCOUNTS_KEY(org?.id ?? '') });
      void queryClient.invalidateQueries({ queryKey: BALANCES_KEY(org?.id ?? '') });
      router.replace('/');
    },
    fix: () => router.replace(org?.kind === 'personal' ? '/onboarding/person' : '/onboarding/company'),
  };
}
