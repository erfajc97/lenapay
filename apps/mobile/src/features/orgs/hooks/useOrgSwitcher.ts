import { router } from 'expo-router';

import type { Org } from '@/shared/api/types';
import { useActiveOrg, useVisibleOrgs } from '@/shared/queries/orgs';
import { usePreferencesStore } from '@/shared/stores/usePreferencesStore';

/** Switch between organizations (R-ORG-1). Every query is keyed by org, so nothing to clear. */
export function useOrgSwitcher() {
  const { query, orgs } = useVisibleOrgs();
  const { org: active } = useActiveOrg();
  const setActiveOrgId = usePreferencesStore((state) => state.setActiveOrgId);
  return {
    query,
    orgs: orgs ?? [],
    activeId: active?.id ?? null,
    select: (org: Org) => {
      setActiveOrgId(org.id);
      router.back();
    },
    create: () => router.push('/onboarding'),
  };
}
