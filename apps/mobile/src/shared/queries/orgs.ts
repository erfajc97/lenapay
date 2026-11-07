import { useQuery } from '@tanstack/react-query';

import { api } from '@/shared/api';
import { pickActiveOrg, visibleOrgs } from '@/shared/helpers/orgs';
import { usePreferencesStore } from '@/shared/stores/usePreferencesStore';
import { useSessionStore } from '@/shared/stores/useSessionStore';

import { useMe } from './session';

export const ORGS_KEY = ['orgs'] as const;
export const MEMBERS_KEY = (orgId: string) => ['members', orgId] as const;
export const KYC_KEY = (orgId: string) => ['kyc', orgId] as const;

export function useOrgs() {
  const active = useSessionStore((state) => state.status === 'active');
  return useQuery({ queryKey: ORGS_KEY, queryFn: () => api.orgs.list(), enabled: active });
}

export function useVisibleOrgs() {
  const orgsQuery = useOrgs();
  const meQuery = useMe();
  const preview = usePreferencesStore((state) => state.personalModePreview);
  const personalMode = preview || meQuery.data?.personalModeEnabled === true;
  return { query: orgsQuery, orgs: orgsQuery.data ? visibleOrgs(orgsQuery.data, personalMode) : undefined };
}

export function useActiveOrg() {
  const { query, orgs } = useVisibleOrgs();
  const rememberedId = usePreferencesStore((state) => state.activeOrgId);
  return { query, org: orgs ? pickActiveOrg(orgs, rememberedId) : null };
}

export function useMembers(orgId: string | undefined) {
  return useQuery({ queryKey: MEMBERS_KEY(orgId ?? ''), queryFn: () => api.orgs.members(orgId ?? ''), enabled: Boolean(orgId) });
}

export function useKyc(orgId: string | undefined, options: { pollWhileSubmitted?: boolean } = {}) {
  return useQuery({
    queryKey: KYC_KEY(orgId ?? ''),
    queryFn: () => api.kyc.get(orgId ?? ''),
    enabled: Boolean(orgId),
    // Polls only while the provider decides; stops on approved or rejected (M3).
    refetchInterval: (query) => (options.pollWhileSubmitted && query.state.data?.status === 'submitted' ? 3_000 : false),
  });
}
