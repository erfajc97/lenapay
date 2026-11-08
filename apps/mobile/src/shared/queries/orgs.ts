import { useQuery } from '@tanstack/react-query';

import { api } from '@/shared/api';
import { pickActiveOrg, rememberedOrgId, visibleOrgs } from '@/shared/helpers/orgs';
import { usePreferencesStore } from '@/shared/stores/usePreferencesStore';
import { useSessionStore } from '@/shared/stores/useSessionStore';

import { useMe } from './session';

export const ORGS_KEY = ['orgs'] as const;
export const MEMBERS_KEY = (orgId: string) => ['members', orgId] as const;
export const KYC_KEY = (orgId: string) => ['kyc', orgId] as const;
export const APPROVAL_POLICY_KEY = (orgId: string) => ['approvalPolicy', orgId] as const;

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

/** The org on screen: this session's pick, else the synced default org (R-ID-5), else the last one used here. */
export function useActiveOrg() {
  const { query, orgs } = useVisibleOrgs();
  const me = useMe();
  const lastOrgId = usePreferencesStore((state) => state.activeOrgId);
  const sessionOrgId = usePreferencesStore((state) => state.sessionOrgId);
  const rememberedId = rememberedOrgId({ sessionOrgId, defaultOrgId: me.data?.preferences.defaultOrgId, lastOrgId });
  return { query, org: orgs ? pickActiveOrg(orgs, rememberedId) : null };
}

/** The org's approval policy (R-OUT-7): every member reads it; personal wallets never need approval. */
export function useApprovalPolicy(orgId: string | undefined) {
  return useQuery({ queryKey: APPROVAL_POLICY_KEY(orgId ?? ''), queryFn: () => api.orgs.approvalPolicy(orgId ?? ''), enabled: Boolean(orgId) });
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
