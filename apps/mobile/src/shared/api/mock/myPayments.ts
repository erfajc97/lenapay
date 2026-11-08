import type { Currency, MyPaymentCompany, MyPaymentPayin, Payer } from '../types';
import { CURRENCIES } from '../types';
import type { MockPayerLink, MockState } from './state';

/**
 * "Mis pagos" read model of the mock (R-MP-1, R-MP-2), the API's `payerview` rules: a payer
 * belongs to the person when its document carries the verified DNI — a DNI equal to it
 * (7 digits left-padded) or an 11-digit CUIT/CUIL whose middle 8 digits are it.
 */
function documentCarriesDni(payer: Pick<Payer, 'docType' | 'docNumber'>, dni: string): boolean {
  const wanted = dni.padStart(8, '0');
  const digits = payer.docNumber.replace(/\D/g, '');
  if (payer.docType === 'dni') return digits.padStart(8, '0') === wanted;
  return digits.length === 11 && digits.slice(2, 10) === wanted;
}

/** Links every payer carrying the DNI that is not linked yet (lazy matching on read). */
export function linkMatchingPayers(state: MockState, personalOrgId: string, dni: string, nextId: () => string): void {
  for (const payer of state.payers) {
    if (!documentCarriesDni(payer, dni)) continue;
    if (state.payerLinks.some((link) => link.personalOrgId === personalOrgId && link.payerId === payer.id)) continue;
    state.payerLinks.push({ id: nextId(), personalOrgId, payerId: payer.id, hidden: false });
  }
}

function creditedPayinsOf(state: MockState, payerId: string) {
  return state.payins.filter((item) => item.payerId === payerId).sort((a, b) => b.createdAt.localeCompare(a.createdAt));
}

/** What the person may see of the company: its name, the project, their own CVUs and payins. */
export function companyView(state: MockState, link: MockPayerLink): MyPaymentCompany {
  const payer = state.payers.find((item) => item.id === link.payerId);
  if (!payer) throw new Error(`mock link ${link.id} without payer`);
  const company = state.orgs.find((item) => item.id === payer.orgId);
  const account = state.accounts.find((item) => item.id === payer.accountId);
  const payins = creditedPayinsOf(state, payer.id);
  const totalPaid = CURRENCIES.flatMap((currency: Currency) => {
    const sum = payins.filter((item) => item.currency === currency).reduce((total, item) => total + item.amount, 0);
    return sum > 0 ? [{ currency, amount: sum }] : [];
  });
  return {
    linkId: link.id,
    companyName: company?.legalName ?? '',
    projectName: account && account.kind === 'project' ? account.name : null,
    cvus: payer.cvus.filter((cvu) => cvu.status === 'active').map((cvu) => ({ currency: cvu.currency, cvu: cvu.cvu, alias: cvu.alias })),
    totalPaid,
    lastPaymentAt: payins[0]?.createdAt ?? null,
    erpLinked: Boolean(state.erp[payer.orgId]?.linkedAt),
    hidden: link.hidden,
  };
}

export function payinsOf(state: MockState, link: MockPayerLink): MyPaymentPayin[] {
  const cvuOf = (cvuId: string) => state.payers.flatMap((item) => item.cvus).find((cvu) => cvu.id === cvuId)?.cvu ?? '';
  return creditedPayinsOf(state, link.payerId).map((item) => ({
    id: item.id,
    cvu: cvuOf(item.cvuId),
    currency: item.currency,
    amount: item.amount,
    status: 'credited',
    createdAt: item.createdAt,
    creditedAt: item.createdAt,
  }));
}
