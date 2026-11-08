import type { Currency, Installment, InstallmentStatus, MyPaymentCompany, PayCompanyRequest, PayinStatus } from '@/shared/api/types';
import { CURRENCIES } from '@/shared/api/types';
import { minorToInput } from '@/shared/money/format';
import { parseAmountInput } from '@/shared/money/parse';
import type { ChipTone } from '@/shared/ui/Chip';

/** The API's cap on the concept the person types (the installment prefix needs the rest). */
export const MAX_PAY_CONCEPT = 120;

/** Status → chip tone; the chip always carries the label too (M7). Overdue stands out. */
export const INSTALLMENT_TONE: Record<InstallmentStatus, ChipTone> = {
  pending: 'warning',
  paid: 'success',
  overdue: 'danger',
};

export const PAYIN_TONE: Record<PayinStatus, ChipTone> = {
  received: 'warning',
  credited: 'success',
  failed: 'danger',
};

export function canPayInstallment(installment: Pick<Installment, 'status'>): boolean {
  return installment.status !== 'paid';
}

/** Installments by number, how many are overdue and the next one to pay (the oldest unpaid). */
export function installmentsView(items: Installment[]): { items: Installment[]; overdueCount: number; next: Installment | null } {
  const sorted = [...items].sort((a, b) => a.number - b.number);
  return {
    items: sorted,
    overdueCount: sorted.filter((item) => item.status === 'overdue').length,
    next: sorted.find(canPayInstallment) ?? null,
  };
}

/** A payment goes to the person's CVU at the company in that currency: only those currencies. */
export function payableCurrencies(company: Pick<MyPaymentCompany, 'cvus'>): Currency[] {
  return CURRENCIES.filter((currency) => company.cvus.some((cvu) => cvu.currency === currency));
}

export interface PayDraft {
  installmentNumber: number | null;
  currency: Currency;
  amountText: string;
  concept: string;
}

/** "Pagar cuota" prefills the installment's amount and currency; "Pagar" starts empty. */
export function payDraftFor(params: { installment: Installment | null; currencies: Currency[] }): PayDraft {
  const { installment } = params;
  if (installment) return { installmentNumber: installment.number, currency: installment.currency, amountText: minorToInput(installment.amount), concept: '' };
  return { installmentNumber: null, currency: params.currencies[0] ?? 'ARS', amountText: '', concept: '' };
}

type AmountError = 'validation.amountRequired' | 'validation.amountInvalid' | 'validation.amountZero';

/** The amount (M16: empty is no value, invalid is never corrected) and the optional concept. */
export function payErrors(draft: PayDraft): { amount: AmountError | null; concept: 'validation.payConcept' | null } {
  const parsed = parseAmountInput(draft.amountText);
  const amount = parsed.kind === 'empty' ? 'validation.amountRequired' : parsed.kind === 'invalid' ? 'validation.amountInvalid' : parsed.minor <= 0 ? 'validation.amountZero' : null;
  return { amount, concept: [...draft.concept.trim()].length > MAX_PAY_CONCEPT ? 'validation.payConcept' : null };
}

/** The request body, or null while the draft is not valid. */
export function payRequest(draft: PayDraft): PayCompanyRequest | null {
  const errors = payErrors(draft);
  const parsed = parseAmountInput(draft.amountText);
  if (errors.amount || errors.concept || parsed.kind !== 'ok') return null;
  const concept = draft.concept.trim();
  return {
    currency: draft.currency,
    amount: parsed.minor,
    ...(draft.installmentNumber === null ? {} : { installmentNumber: draft.installmentNumber }),
    ...(concept === '' ? {} : { concept }),
  };
}
