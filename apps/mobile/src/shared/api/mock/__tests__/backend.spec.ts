import { beforeEach, describe, expect, it } from 'vitest';

import { MockBackend } from '../backend';
import { applyFee, payoutFee } from '../feePlan';

// Why: the mock is what the owner sees and demos; its balances, fees and 4-eyes rules must
// behave like the PRD or the UI is validated against wrong money (R-OUT-2, R-OUT-3, R-OUT-4).
const pending: (() => void)[] = [];
function backend(): MockBackend {
  const instance = new MockBackend({ now: () => Date.UTC(2026, 10, 7, 15), setTimeout: (callback) => pending.push(callback) });
  instance.login({ email: 'erick@desarrollosdelsur.com.ar', password: 'secreto1' });
  return instance;
}
const ORG = 'org_dds';
const creds = (key: string) => ({ idempotencyKey: key, stepUpToken: 'su' });
const available = (b: MockBackend, accountId: string, currency: 'ARS' | 'USD') =>
  b.listAccounts(ORG).find((item) => item.id === accountId)?.balances.find((item) => item.currency === currency)?.available ?? NaN;

describe('fee plan', () => {
  it('rounds the percentage half-up in integer arithmetic', () => {
    expect(applyFee(4_999, { fixedMinor: 0, bps: 1 })).toBe(0);
    expect(applyFee(5_000, { fixedMinor: 0, bps: 1 })).toBe(1);
    expect(payoutFee(1_000_000_00, 'ARS')).toBe(150_00 + 1_000_00);
    expect(payoutFee(1_000_00, 'USD')).toBe(1_00);
  });
});

describe('payouts', () => {
  let b: MockBackend;
  beforeEach(() => {
    pending.length = 0;
    b = backend();
  });

  it('holds amount + fee on creation and waits for approval', () => {
    const before = available(b, 'acc_main', 'USD');
    const payout = b.createPayout(ORG, { accountId: 'acc_main', currency: 'USD', amount: 100_00, to: 'estudio.bianchi', concept: 'Planos' }, creds('k1'));
    expect(payout.status).toBe('pending_approval');
    expect(available(b, 'acc_main', 'USD')).toBe(before - 100_00 - 1_00);
  });

  it('accepts the last affordable amount and rejects one minor unit more', () => {
    const free = available(b, 'acc_main', 'USD');
    const fee = 1_00;
    expect(() => b.createPayout(ORG, { accountId: 'acc_main', currency: 'USD', amount: free - fee + 1, to: 'estudio.bianchi', concept: 'x' }, creds('k2'))).toThrow(
      expect.objectContaining({ code: 'payout.insufficient_funds' }),
    );
    expect(b.createPayout(ORG, { accountId: 'acc_main', currency: 'USD', amount: free - fee, to: 'estudio.bianchi', concept: 'x' }, creds('k3')).status).toBe('pending_approval');
  });

  it('replays the same key + payload and rejects the same key with another payload', () => {
    const body = { accountId: 'acc_main', currency: 'ARS' as const, amount: 10_000_00, to: 'hormigon.plata', concept: 'x' };
    const first = b.createPayout(ORG, body, creds('k4'));
    const before = available(b, 'acc_main', 'ARS');
    expect(b.createPayout(ORG, body, creds('k4')).id).toBe(first.id);
    expect(available(b, 'acc_main', 'ARS')).toBe(before);
    expect(() => b.createPayout(ORG, { ...body, amount: 10_000_01 }, creds('k4'))).toThrow(expect.objectContaining({ code: 'idempotency.conflict' }));
  });

  it('never lets the creator approve and settles after another approver approves', () => {
    expect(() => b.approvePayout(ORG, 'po_6', creds('a1'))).toThrow(expect.objectContaining({ code: 'payout.creator_cannot_approve' }));
    const approved = b.approvePayout(ORG, 'po_5', creds('a2'));
    expect(approved.status).toBe('approved');
    const heldBefore = b.listAccounts(ORG).find((item) => item.id === 'acc_main')?.balances.find((item) => item.currency === 'ARS');
    for (const run of pending.splice(0)) run();
    const heldAfter = b.listAccounts(ORG).find((item) => item.id === 'acc_main')?.balances.find((item) => item.currency === 'ARS');
    expect(b.getPayout(ORG, 'po_5').status).toBe('settled');
    expect(heldAfter?.posted).toBe((heldBefore?.posted ?? 0) - approved.amount - approved.fee);
    expect(heldAfter?.available).toBe(heldBefore?.available);
  });

  it('releases the hold on reject', () => {
    const before = available(b, 'acc_main', 'ARS');
    const rejected = b.rejectPayout(ORG, 'po_5', 'Factura vencida', creds('r1'));
    expect(available(b, 'acc_main', 'ARS')).toBe(before + rejected.amount + rejected.fee);
  });
});

describe('accounts and transfers', () => {
  let b: MockBackend;
  beforeEach(() => {
    b = backend();
  });

  it('archives only a project account with zero balance', () => {
    expect(() => b.updateAccount(ORG, 'acc_torre', { archived: true })).toThrow(expect.objectContaining({ code: 'account.not_empty' }));
    expect(() => b.updateAccount(ORG, 'acc_main', { archived: true })).toThrow(expect.objectContaining({ code: 'account.main_locked' }));
    const created = b.createAccount(ORG, { name: 'Oficinas Rosario' });
    expect(b.updateAccount(ORG, created.id, { archived: true }).archivedAt).not.toBeNull();
  });

  it('moves money between own accounts immediately and refuses the same account', () => {
    const from = available(b, 'acc_torre', 'ARS');
    const to = available(b, 'acc_nordelta', 'ARS');
    b.transfer(ORG, { fromAccountId: 'acc_torre', toAccountId: 'acc_nordelta', currency: 'ARS', amount: 1_000_00, concept: 'x' }, creds('t1'));
    expect(available(b, 'acc_torre', 'ARS')).toBe(from - 1_000_00);
    expect(available(b, 'acc_nordelta', 'ARS')).toBe(to + 1_000_00);
    expect(() => b.transfer(ORG, { fromAccountId: 'acc_torre', toAccountId: 'acc_torre', currency: 'ARS', amount: 1, concept: 'x' }, creds('t2'))).toThrow(
      expect.objectContaining({ code: 'transfer.same_account' }),
    );
  });

  it('credits a live payin net of fee to the payer project account', () => {
    const before = available(b, 'acc_torre', 'ARS');
    const events: unknown[] = [];
    b.subscribe(ORG, (event) => events.push(event));
    b.simulateIncomingPayin();
    expect(events).toHaveLength(1);
    expect(events[0]).toMatchObject({
      type: 'payin.credited',
      payin: { payerName: 'Juan Pérez', accountId: 'acc_torre', currency: 'ARS', amount: 150_000_00, fee: 900_00, net: 149_100_00 },
    });
    expect(available(b, 'acc_torre', 'ARS')).toBe(before + 149_100_00);
  });
});
