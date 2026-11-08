import { router } from 'expo-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import type { MyPaymentCompany, MyPaymentCvu } from '@/shared/api/types';
import { personalOrg, verifiedPersonalOrgId } from '@/shared/helpers/orgs';
import { useCopy, useShareText } from '@/shared/mutations/useDeviceActions';
import { useMyPaymentCompanies } from '@/shared/queries/myPayments';
import { useOrgs } from '@/shared/queries/orgs';
import { useMe } from '@/shared/queries/session';
import { usePreferencesStore } from '@/shared/stores/usePreferencesStore';

import { useSetCompanyHidden } from '../mutations/useSetCompanyHidden';

/** "Mis pagos" list (R-MP-2): the companies the person pays, their CVUs there and totals. */
export function useMyPayments() {
  const { t } = useTranslation();
  const me = useMe();
  const personalOrgId = verifiedPersonalOrgId(me.data?.memberships);
  const [showHidden, setShowHidden] = useState(false);
  const companies = useMyPaymentCompanies(personalOrgId !== null, showHidden);
  const copy = useCopy();
  const share = useShareText();
  const setHidden = useSetCompanyHidden();
  const wallet = personalOrg(useOrgs().data);
  const setActiveOrgId = usePreferencesStore((state) => state.setActiveOrgId);

  return {
    me,
    verified: personalOrgId !== null,
    companies,
    showHidden,
    toggleShowHidden: () => setShowHidden((value) => !value),
    open: (company: MyPaymentCompany) => router.push({ pathname: '/my-payments/[linkId]', params: { linkId: company.linkId } }),
    copy: copy.mutate,
    share: (company: MyPaymentCompany, cvu: MyPaymentCvu) =>
      share.mutate(t('myPayments.shareMessage', { company: company.companyName, currency: t(`currency.${cvu.currency}`), cvu: cvu.cvu, alias: cvu.alias ?? '-' })),
    toggleHidden: (company: MyPaymentCompany) => setHidden.mutate({ linkId: company.linkId, hidden: !company.hidden }),
    togglingId: setHidden.isPending ? (setHidden.variables?.linkId ?? null) : null,
    // The person KYC runs on the personal wallet: make it the active org first.
    canVerify: wallet !== null,
    openVerification: () => {
      if (!wallet) return;
      setActiveOrgId(wallet.id);
      router.push('/onboarding/person');
    },
  };
}
