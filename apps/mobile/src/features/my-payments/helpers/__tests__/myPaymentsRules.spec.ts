import { describe, expect, it } from 'vitest';

import type { Installment, MyPaymentCompany } from '@/shared/api/types';

import {
  INSTALLMENT_TONE,
  MAX_PAY_CONCEPT,
  PAYIN_TONE,
  canPayInstallment,
  installmentsView,
  payableCurrencies,
  payDraftFor,
  payErrors,
  payRequest,
} from '../myPaymentsRules';

// Why: "Mis pagos" moves the person's own money to a company. A wrong prefill pays the wrong
// installment amount, a paid installment offered again pays twice, and a currency without a CVU
// at the company is a guaranteed 422 at the confirm step (R-MP-3, R-MP-4).
const installment = (number: number, status: Installment['status'], overrides: Partial<Installment> = {}): Installment => ({
  number,
  dueDate: `2026-${String(number).padStart(2, '0')}-10`,
  currency: 'ARS',
  amount: 850_000_00 + number,
  status,
  adjustmentIndex: 'CAC',
  ...overrides,
});

const company = (cvus: MyPaymentCompany['cvus']): Pick<MyPaymentCompany, 'cvus'> => ({ cvus });

describe('installments', () => {
  it('maps every contract status to a chip tone, overdue as danger', () => {
    expect(Object.keys(INSTALLMENT_TONE).sort()).toEqual(['overdue', 'paid', 'pending']);
    expect(INSTALLMENT_TONE.overdue).toBe('danger');
    expect(INSTALLMENT_TONE.paid).toBe('success');
    expect(Object.keys(PAYIN_TONE).sort()).toEqual(['credited', 'failed', 'received']);
  });

  it('offers to pay pending and overdue installments, never a paid one', () => {
    expect(canPayInstallment(installment(1, 'pending'))).toBe(true);
    expect(canPayInstallment(installment(1, 'overdue'))).toBe(true);
    expect(canPayInstallment(installment(1, 'paid'))).toBe(false);
  });

  it('counts the overdue ones and picks the next to pay (the oldest unpaid)', () => {
    const view = installmentsView([installment(3, 'pending'), installment(1, 'paid'), installment(2, 'overdue')]);
    expect(view.items.map((item) => item.number)).toEqual([1, 2, 3]);
    expect(view.overdueCount).toBe(1);
    expect(view.next?.number).toBe(2);
  });

  it('has no next installment when everything is paid, nor with an empty list', () => {
    expect(installmentsView([installment(1, 'paid')]).next).toBeNull();
    expect(installmentsView([])).toEqual({ items: [], overdueCount: 0, next: null });
  });
});

describe('pay form', () => {
  it('pays only in the currencies the person has a CVU for at the company, ARS first', () => {
    const cvu = (currency: 'ARS' | 'USD') => ({ currency, cvu: '0'.repeat(22), alias: null });
    expect(payableCurrencies(company([cvu('USD'), cvu('ARS'), cvu('ARS')]))).toEqual(['ARS', 'USD']);
    expect(payableCurrencies(company([cvu('USD')]))).toEqual(['USD']);
    expect(payableCurrencies(company([]))).toEqual([]);
  });

  it('prefills amount and currency from the installment, untouched in minor units', () => {
    const draft = payDraftFor({ installment: installment(4, 'overdue', { amount: 887_500_55, currency: 'USD' }), currencies: ['ARS', 'USD'] });
    expect(draft).toEqual({ installmentNumber: 4, currency: 'USD', amountText: '887500,55', concept: '' });
  });

  it('starts a free payment empty, in the first payable currency', () => {
    expect(payDraftFor({ installment: null, currencies: ['USD'] })).toEqual({ installmentNumber: null, currency: 'USD', amountText: '', concept: '' });
  });

  it('requires a positive amount and keeps the concept within the API cap', () => {
    const base = { installmentNumber: null, currency: 'ARS' as const, amountText: '1000', concept: '' };
    expect(payErrors(base)).toEqual({ amount: null, concept: null });
    expect(payErrors({ ...base, amountText: '' }).amount).toBe('validation.amountRequired');
    expect(payErrors({ ...base, amountText: '0' }).amount).toBe('validation.amountZero');
    expect(payErrors({ ...base, amountText: '12a' }).amount).toBe('validation.amountInvalid');
    expect(payErrors({ ...base, concept: 'ñ'.repeat(MAX_PAY_CONCEPT) }).concept).toBeNull();
    expect(payErrors({ ...base, concept: 'ñ'.repeat(MAX_PAY_CONCEPT + 1) }).concept).toBe('validation.payConcept');
  });

  it('builds the request: installment number when paying one, typed concept only when given', () => {
    expect(payRequest({ installmentNumber: 4, currency: 'ARS', amountText: '887.500,50', concept: '' })).toEqual({ currency: 'ARS', amount: 887_500_50, installmentNumber: 4 });
    expect(payRequest({ installmentNumber: null, currency: 'USD', amountText: '10', concept: '  Seña  ' })).toEqual({ currency: 'USD', amount: 10_00, concept: 'Seña' });
    expect(payRequest({ installmentNumber: null, currency: 'ARS', amountText: '', concept: '' })).toBeNull();
  });
});
