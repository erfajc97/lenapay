import { useLocalSearchParams } from 'expo-router';
import { useState } from 'react';

import type { Currency, Installment, Payout } from '@/shared/api/types';
import { verifiedPersonalOrgId } from '@/shared/helpers/orgs';
import { useMyPaymentCompanies, useMyPaymentInstallments, useMyPaymentPayins } from '@/shared/queries/myPayments';
import { useMe } from '@/shared/queries/session';

import { installmentsView, type PayDraft, payableCurrencies, payDraftFor, payErrors, payRequest } from '../helpers/myPaymentsRules';
import { usePayCompany } from '../mutations/useMyPaymentsMutations';

export type CompanyTab = 'payins' | 'installments';

/** Pay sheet: closed, the form (prefilled from an installment or empty) or the result. */
type PaySheet = { step: 'form'; draft: PayDraft; installment: Installment | null } | { step: 'result'; payout: Payout } | null;

/** Company detail of "Mis pagos" (R-MP-2..4): payment history, installments and the pay sheet. */
export function useMyPaymentsCompany() {
  const params = useLocalSearchParams<{ linkId?: string }>();
  const linkId = typeof params.linkId === 'string' && params.linkId !== '' ? params.linkId : undefined;
  const me = useMe();
  const personalOrgId = verifiedPersonalOrgId(me.data?.memberships);
  // The list with hidden companies too: a hidden company still opens from its own link.
  const companies = useMyPaymentCompanies(personalOrgId !== null && linkId !== undefined, true);
  const company = companies.data?.find((item) => item.linkId === linkId) ?? null;
  const [tab, setTab] = useState<CompanyTab>('installments');
  const payins = useMyPaymentPayins(linkId);
  const installments = useMyPaymentInstallments(linkId);
  const [sheet, setSheet] = useState<PaySheet>(null);
  const [showErrors, setShowErrors] = useState(false);
  const pay = usePayCompany({ linkId, personalOrgId, onPaid: (payout) => setSheet({ step: 'result', payout }) });

  const currencies = company ? payableCurrencies(company) : [];
  const draft = sheet?.step === 'form' ? sheet.draft : null;
  const errors = draft ? payErrors(draft) : null;
  const updateDraft = (patch: Partial<PayDraft>) => setSheet((current) => (current?.step === 'form' ? { ...current, draft: { ...current.draft, ...patch } } : current));

  return {
    linkId,
    companies,
    company,
    notFound: companies.isSuccess && company === null,
    tab,
    setTab,
    payins,
    payinItems: payins.data?.pages.flatMap((page) => page.items) ?? [],
    loadMorePayins: () => {
      if (payins.hasNextPage && !payins.isFetchingNextPage) void payins.fetchNextPage();
    },
    installments,
    installmentsView: installmentsView(installments.data?.items ?? []),
    currencies,
    sheet,
    draft,
    errors: showErrors ? errors : null,
    openPay: (installment: Installment | null) => {
      setShowErrors(false);
      setSheet({ step: 'form', draft: payDraftFor({ installment, currencies }), installment });
    },
    closePay: () => setSheet(null),
    setCurrency: (currency: Currency) => updateDraft({ currency }),
    setAmountText: (amountText: string) => updateDraft({ amountText }),
    setConcept: (concept: string) => updateDraft({ concept }),
    confirm: () => {
      setShowErrors(true);
      const body = draft ? payRequest(draft) : null;
      if (body) pay.mutate(body);
    },
    paying: pay.isPending,
    refresh: () => Promise.all([companies.refetch(), payins.refetch(), installments.refetch()]),
  };
}
