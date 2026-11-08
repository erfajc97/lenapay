import { randomUUID } from 'node:crypto';

import { beforeAll, describe, expect, it, vi } from 'vitest';

import { saveTokens } from '@/shared/session/session';
import { verifyStepUp } from '@/shared/stepup/stepUpFlow';

import type { LenapayApi } from '../../client';
import type { Account, MyPaymentCompany, Org } from '../../types';
import { createLiveApi } from '../liveApi';

/**
 * "Mis pagos", the personal wallet and on-us rails against the running stack (`pnpm test:live`):
 * the seeded persona (verified DNI, payer of two companies, one ERP-linked) lists her companies,
 * reads installments and payins, hides/shows one, edits her CVU alias (restored), quotes an
 * on-us transfer and pays an installment with a password step-up. Demo password only.
 */
const BASE_URL = process.env.LIVE_API_URL ?? 'http://localhost:18080';
const PASSWORD = 'Lenapay2026!';
const PERSONA = 'persona@lenapay.app';
const DEMO = 'demo@lenapay.app';

// No realtime here: the stream is covered by liveApi.live.ts.
vi.mock('react-native-sse', () => ({ default: class {} }));
vi.mock('expo-file-system', () => ({ File: class {}, Paths: { cache: '' } }));

async function signIn(api: LenapayApi, email: string): Promise<void> {
  await saveTokens(await api.auth.login({ email, password: PASSWORD }));
}

function stepUp(api: LenapayApi): Promise<string> {
  return verifyStepUp({ method: 'password', password: PASSWORD }, { auth: api.auth, readDeviceKey: async () => null }).then((token) => token.stepUpToken);
}

describe(`Mis pagos and the personal wallet against ${BASE_URL}`, () => {
  const api = createLiveApi({ baseUrl: BASE_URL });
  let personal: Org;
  let account: Account;
  let erpCompany: MyPaymentCompany;

  beforeAll(async () => {
    const health = await fetch(`${BASE_URL}/healthz`).catch(() => null);
    if (!health?.ok) throw new Error(`API not reachable at ${BASE_URL}: start the stack (docker compose up -d --wait) and seed it`);
  });

  it('finds the verified personal wallet with its own ARS and USD CVUs', async () => {
    await signIn(api, PERSONA);
    const me = await api.auth.me();
    expect(me.memberships).toContainEqual(expect.objectContaining({ orgKind: 'personal', kycStatus: 'approved' }));
    personal = (await api.orgs.list()).find((org) => org.kind === 'personal') as Org;
    expect(personal).toBeDefined();
    account = (await api.accounts.list(personal.id)).find((item) => item.kind === 'personal') as Account;
    expect(account.cvus.map((cvu) => cvu.currency).sort()).toEqual(['ARS', 'USD']);
    for (const cvu of account.cvus) expect(cvu.cvu).toMatch(/^\d{22}$/);
  });

  it('edits the alias of the wallet CVU through the account endpoint, then restores it', async () => {
    const cvu = account.cvus[0];
    if (!cvu) throw new Error('no CVU');
    const original = cvu.alias;
    const next = `lnp.test.${randomUUID().slice(0, 6)}`;
    expect((await api.accounts.updateCvu(personal.id, account.id, cvu.id, { alias: next })).alias).toBe(next);
    if (original) expect((await api.accounts.updateCvu(personal.id, account.id, cvu.id, { alias: original })).alias).toBe(original);
    await expect(api.accounts.updateCvu(personal.id, account.id, randomUUID(), { alias: 'otro.alias.x' })).rejects.toMatchObject({ code: 'cvu.not_found' });
  });

  it('lists the companies with my CVUs, totals and the ERP flag', async () => {
    const companies = await api.myPayments.companies({ includeHidden: false });
    expect(companies.length).toBeGreaterThanOrEqual(2);
    for (const company of companies) {
      expect(company.cvus.length).toBeGreaterThan(0);
      for (const total of company.totalPaid) expect(Number.isSafeInteger(total.amount)).toBe(true);
    }
    erpCompany = companies.find((company) => company.erpLinked) as MyPaymentCompany;
    expect(erpCompany).toBeDefined();
    expect(companies.some((company) => !company.erpLinked)).toBe(true);
  });

  it('reads installments (and none for a company without ERP) and the payin history', async () => {
    const plan = await api.myPayments.installments(erpCompany.linkId);
    expect(plan.erpLinked).toBe(true);
    expect(plan.items.length).toBeGreaterThan(0);
    expect(plan.items.some((item) => item.status === 'overdue')).toBe(true);
    const plain = (await api.myPayments.companies({ includeHidden: false })).find((company) => !company.erpLinked);
    expect(await api.myPayments.installments(plain?.linkId ?? '')).toEqual({ erpLinked: false, items: [] });
    const payins = await api.myPayments.payins(erpCompany.linkId, null);
    expect(payins.items.length).toBeGreaterThan(0);
    expect(payins.items[0]?.cvu).toMatch(/^\d{22}$/);
  });

  it('hides and shows a company', async () => {
    expect((await api.myPayments.setHidden(erpCompany.linkId, true)).hidden).toBe(true);
    expect((await api.myPayments.companies({ includeHidden: false })).some((company) => company.linkId === erpCompany.linkId)).toBe(false);
    expect((await api.myPayments.companies({ includeHidden: true })).some((company) => company.linkId === erpCompany.linkId)).toBe(true);
    expect((await api.myPayments.setHidden(erpCompany.linkId, false)).hidden).toBe(false);
  });

  it('quotes a payment to my CVU at the company as an on-us (internal) transfer', async () => {
    const to = erpCompany.cvus.find((cvu) => cvu.currency === 'ARS')?.cvu ?? '';
    const quote = await api.payouts.quote(personal.id, { accountId: account.id, currency: 'ARS', amount: 100_00, to });
    expect(quote.rail).toBe('internal');
    expect(quote.total).toBe(quote.amount + quote.fee);
  });

  it('pays an installment with a step-up: settled on-us, replayed by the same key', async () => {
    const plan = await api.myPayments.installments(erpCompany.linkId);
    const due = plan.items.find((item) => item.status !== 'paid');
    if (!due) throw new Error('no installment to pay');
    const body = { currency: due.currency, amount: 100_00, installmentNumber: due.number };
    const idempotencyKey = randomUUID();
    const payout = await api.myPayments.pay(erpCompany.linkId, body, { idempotencyKey, stepUpToken: await stepUp(api) });
    expect(payout).toMatchObject({ rail: 'internal', status: 'settled', amount: 100_00, currency: due.currency });
    expect(payout.concept.startsWith(`Cuota ${due.number} - `)).toBe(true);
    expect((await api.myPayments.pay(erpCompany.linkId, body, { idempotencyKey, stepUpToken: await stepUp(api) })).id).toBe(payout.id);
    const movements = await api.movements.list(personal.id, { type: 'payout' }, null);
    expect(movements.items.find((item) => item.refId === payout.id)?.rail).toBe('internal');
  });

  it('answers installment.not_found and payer_link.not_found with their codes', async () => {
    await expect(api.myPayments.pay(erpCompany.linkId, { currency: 'ARS', amount: 1_00, installmentNumber: 999 }, { idempotencyKey: randomUUID(), stepUpToken: await stepUp(api) })).rejects.toMatchObject({
      code: 'installment.not_found',
    });
    await expect(api.myPayments.installments(randomUUID())).rejects.toMatchObject({ code: 'payer_link.not_found' });
  });

  it('keeps a company member without a verified personal wallet out of Mis pagos', async () => {
    await signIn(api, DEMO);
    const me = await api.auth.me();
    const verified = me.memberships.some((item) => item.orgKind === 'personal' && item.kycStatus === 'approved');
    if (verified) return;
    await expect(api.myPayments.companies({ includeHidden: false })).rejects.toMatchObject({ code: 'kyc.not_approved' });
  });
});
