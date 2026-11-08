import { router } from 'expo-router';
import { useState } from 'react';

import type { OrgKind } from '@/shared/api/types';
import { personalOrg } from '@/shared/helpers/orgs';
import { useActiveOrg, useOrgs } from '@/shared/queries/orgs';
import { useMe } from '@/shared/queries/session';
import { usePreferencesStore } from '@/shared/stores/usePreferencesStore';

import { onboardingTarget } from '../helpers/onboardingTarget';
import { useCreateOrg } from '../mutations/useKycMutations';

function legalNameError(name: string): 'validation.legalName' | null {
  return name.trim().length >= 3 ? null : 'validation.legalName';
}

/** Step 1 of onboarding: company or person, and its name (R-KYC-1, R-KYC-2). */
export function useOnboarding() {
  const { org } = useActiveOrg();
  const me = useMe();
  const preview = usePreferencesStore((state) => state.personalModePreview);
  const setActiveOrgId = usePreferencesStore((state) => state.setActiveOrgId);
  const existingPersonal = personalOrg(useOrgs().data);
  const [kind, setKind] = useState<OrgKind>('company');
  const [legalName, setLegalName] = useState('');
  const [touched, setTouched] = useState(false);
  const create = useCreateOrg();
  const error = legalNameError(legalName);
  const pending = org && (org.kycStatus === 'draft' || org.kycStatus === 'rejected') ? org : null;

  return {
    pendingOrg: pending,
    continuePending: () => {
      if (pending) router.push(pending.kind === 'company' ? '/onboarding/company' : '/onboarding/person');
    },
    // Personal wallets are B2C behind a flag (PRD §1); the Settings preview turns it on.
    personalEnabled: preview || me.data?.personalModeEnabled === true,
    kind,
    setKind,
    legalName,
    setLegalName: (value: string) => {
      setLegalName(value);
      setTouched(true);
    },
    error: touched ? error : null,
    submit: () => {
      setTouched(true);
      const target = onboardingTarget({ kind, existingPersonal });
      if (target.action === 'open') {
        setActiveOrgId(target.orgId);
        router.push('/onboarding/person');
        return;
      }
      if (!error) create.mutate({ kind: kind === 'company' ? 'company' : 'personal', legalName: legalName.trim() });
    },
    creating: create.isPending,
  };
}
