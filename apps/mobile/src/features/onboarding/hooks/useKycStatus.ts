import { useQueryClient } from '@tanstack/react-query';
import { router } from 'expo-router';

import type { KycStatus } from '@/shared/api/types';
import { ACCOUNTS_KEY, BALANCES_KEY } from '@/shared/queries/accounts';
import { ORGS_KEY, useActiveOrg, useKyc } from '@/shared/queries/orgs';

const STEPS: KycStatus[] = ['draft', 'submitted', 'approved'];

/** The timeline borrador → enviado → aprobado/rechazado; the last step shows the decision. */
function kycTimeline(status: KycStatus): { status: KycStatus; state: 'done' | 'current' | 'upcoming' }[] {
  const steps: KycStatus[] = status === 'rejected' ? ['draft', 'submitted', 'rejected'] : STEPS;
  const index = steps.indexOf(status);
  return steps.map((step, position) => ({
    status: step,
    state: position < index || status === 'approved' ? 'done' : position === index ? 'current' : 'upcoming',
  }));
}

export function useKycStatus() {
  const queryClient = useQueryClient();
  const { org } = useActiveOrg();
  const kyc = useKyc(org?.id, { pollWhileSubmitted: true });
  return {
    org,
    kyc,
    timeline: kycTimeline(kyc.data?.status ?? 'draft'),
    goHome: () => {
      void queryClient.invalidateQueries({ queryKey: ORGS_KEY });
      void queryClient.invalidateQueries({ queryKey: ACCOUNTS_KEY(org?.id ?? '') });
      void queryClient.invalidateQueries({ queryKey: BALANCES_KEY(org?.id ?? '') });
      router.replace('/');
    },
    fix: () => router.replace(org?.kind === 'personal' ? '/onboarding/person' : '/onboarding/company'),
  };
}
