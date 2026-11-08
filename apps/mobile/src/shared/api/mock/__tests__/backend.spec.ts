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
    // On-us transfers cost nothing by default (R-MP-5); the bank rail keeps its fee.
    expect(payoutFee(1_000_000_00, 'ARS', 'internal')).toBe(0);
    expect(payoutFee(1_000_000_00, 'ARS', 'bank')).toBe(150_00 + 1_000_00);
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
      expect.objectContaining({ code: 'ledger.insufficient_funds' }),
    );
    expect(b.createPayout(ORG, { accountId: 'acc_main', currency: 'USD', amount: free - fee, to: 'estudio.bianchi', concept: 'x' }, creds('k3')).status).toBe('pending_approval');
  });

  it('replays the same key + payload and rejects the same key with another payload', () => {
    const body = { accountId: 'acc_main', currency: 'ARS' as const, amount: 10_000_00, to: 'hormigon.plata', concept: 'x' };
    const first = b.createPayout(ORG, body, creds('k4'));
    const before = available(b, 'acc_main', 'ARS');
    expect(b.createPayout(ORG, body, creds('k4')).id).toBe(first.id);
    expect(available(b, 'acc_main', 'ARS')).toBe(before);
    expect(() => b.createPayout(ORG, { ...body, amount: 10_000_01 }, creds('k4'))).toThrow(expect.objectContaining({ code: 'idempotency.key_reused' }));
  });

  it('never lets the creator approve and settles after another approver approves', () => {
    expect(() => b.approvePayout(ORG, 'po_6', creds('a1'))).toThrow(expect.objectContaining({ code: 'payout.self_approval_forbidden' }));
    const approved = b.approvePayout(ORG, 'po_5', creds('a2'));
    expect(approved.status).toBe('approved');
    const heldBefore = b.listAccounts(ORG).find((item) => item.id === 'acc_main')?.balances.find((item) => item.currency === 'ARS');
    for (const run of pending.splice(0)) run();
    const heldAfter = b.listAccounts(ORG).find((item) => item.id === 'acc_main')?.balances.find((item) => item.currency === 'ARS');
    expect(b.getPayout(ORG, 'po_5').status).toBe('settled');
    expect(heldAfter?.posted).toBe((heldBefore?.posted ?? 0) - approved.total);
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
    expect(() => b.updateAccount(ORG, 'acc_main', { archived: true })).toThrow(expect.objectContaining({ code: 'account.not_archivable' }));
    const created = b.createAccount(ORG, { name: 'Oficinas Rosario' });
    expect(b.updateAccount(ORG, created.id, { archived: true }).archived).toBe(true);
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
    expect(events).toHaveLength(2);
    expect(events[1]).toEqual({ type: 'balance.changed', accountId: 'acc_torre' });
    expect(events[0]).toMatchObject({
      type: 'payin.credited',
      payin: { originName: 'Juan Pérez', accountName: 'Torre Palermo', accountId: 'acc_torre', currency: 'ARS', amount: 150_000_00, fee: 900_00, net: 149_100_00 },
    });
    expect(available(b, 'acc_torre', 'ARS')).toBe(before + 149_100_00);
  });
});

// Why: the contract's `from`/`to` are inclusive business dates; comparing them to timestamps
// as strings dropped every movement of the last day ("2026-11-07T15:00Z" > "2026-11-07").
describe('movement date filters', () => {
  it('includes both ends of the range and nothing outside it', () => {
    const b = backend();
    b.transfer(ORG, { fromAccountId: 'acc_main', toAccountId: 'acc_torre', currency: 'ARS', amount: 1_00, concept: 'Fecha' }, creds('k-date'));
    const ids = (from: string, to: string) => b.listMovements(ORG, { type: 'internal_transfer', from, to }, null).items.filter((item) => item.concept === 'Fecha').length;
    expect(ids('2026-11-07', '2026-11-07')).toBe(2);
    expect(ids('2026-11-01', '2026-11-06')).toBe(0);
    expect(ids('2026-11-08', '2026-11-30')).toBe(0);
  });
});

// Why: the mock is what the owner demos; team, KYC, attachment and reconciliation rules must
// answer with the same codes as the API so the screens are built against real behaviour.
describe('team', () => {
  it('changes a role but never demotes or removes the last admin', () => {
    const b = backend();
    expect(b.updateMember(ORG, 'u_martina', { role: 'approver' }).role).toBe('approver');
    expect(() => b.updateMember(ORG, 'u_erick', { role: 'operator' })).toThrow(expect.objectContaining({ code: 'org.last_admin' }));
    expect(() => b.removeMember(ORG, 'u_erick')).toThrow(expect.objectContaining({ code: 'org.last_admin' }));
    b.removeMember(ORG, 'u_sofia');
    expect(b.members(ORG).some((member) => member.userId === 'u_sofia')).toBe(false);
    expect(() => b.removeMember(ORG, 'u_sofia')).toThrow(expect.objectContaining({ code: 'member.not_found' }));
  });
});

describe('payout attachments', () => {
  it('accepts PDF, JPEG and PNG up to ten per payout', () => {
    const b = backend();
    const file = (mimeType: string) => ({ fileName: 'f', fileUri: 'mock://f', mimeType });
    expect(() => b.uploadAttachment(ORG, 'po_6', file('text/plain'))).toThrow(expect.objectContaining({ code: 'payout.attachment_invalid' }));
    for (let index = 0; index < 10; index += 1) b.uploadAttachment(ORG, 'po_6', file(index % 2 ? 'image/png' : 'application/pdf'));
    expect(b.getPayout(ORG, 'po_6').attachments).toHaveLength(10);
    expect(() => b.uploadAttachment(ORG, 'po_6', file('image/jpeg'))).toThrow(expect.objectContaining({ code: 'payout.attachment_invalid' }));
  });
});

describe('KYC', () => {
  it('requires DNI front, back and selfie per representative before submitting', () => {
    const b = backend();
    const org = b.createOrg({ kind: 'company', legalName: 'Nueva SA' });
    const address = { street: 'Calle', number: '1', city: 'CABA', province: 'CABA', postalCode: 'C1000', country: 'AR' };
    b.updateKyc(org.id, { company: { cuit: '30712345671', legalName: 'Nueva SA', legalAddress: address, representatives: [{ fullName: 'Ana', dni: '30111222', position: 'Presidenta' }] } });
    const upload = (kind: 'bylaws' | 'afip_constancy' | 'dni_front' | 'dni_back' | 'selfie', representativeDni?: string) =>
      b.uploadKycDocument(org.id, { kind, representativeDni, fileName: 'f.pdf', fileUri: 'mock://f', mimeType: 'application/pdf' });
    expect(() => upload('dni_front')).toThrow(expect.objectContaining({ code: 'validation.failed' }));
    for (const kind of ['bylaws', 'afip_constancy'] as const) upload(kind);
    upload('dni_front', '30111222');
    upload('selfie', '30111222');
    expect(() => b.submitKyc(org.id)).toThrow(expect.objectContaining({ code: 'kyc.incomplete' }));
    upload('dni_back', '30111222');
    expect(b.submitKyc(org.id).status).toBe('submitted');
    expect(() => b.updateKyc(org.id, { company: { cuit: '30712345671', legalName: 'x', legalAddress: address, representatives: [] } })).toThrow(expect.objectContaining({ code: 'kyc.invalid_state' }));
  });
});

describe('movements and reconciliation', () => {
  it('reads one movement by id and answers movement.not_found otherwise', () => {
    const b = backend();
    const first = b.listMovements(ORG, {}, null).items[0];
    expect(first && b.getMovement(ORG, first.id)).toEqual(first);
    expect(() => b.getMovement(ORG, 'mv_nope')).toThrow(expect.objectContaining({ code: 'movement.not_found' }));
  });

  it('lists a run, its items, and queues an on-demand run for admins', () => {
    const b = backend();
    const run = b.reconciliationRuns(ORG).find((item) => item.missingInLedger > 0);
    expect(run).toBeDefined();
    const items = b.reconciliationItems(ORG, run?.id ?? '');
    expect(items.filter((item) => item.status === 'missing_in_ledger')).toHaveLength(1);
    expect(run?.difference).toBe((run?.pspTotal ?? 0) - (run?.ledgerTotal ?? 0));
    const created = b.runReconciliation(ORG, { currency: 'ARS', day: '2026-11-07' });
    expect(created).toMatchObject({ status: 'completed', day: '2026-11-07', currency: 'ARS' });
    expect(b.reconciliationRuns(ORG).some((item) => item.id === created.id)).toBe(true);
    expect(() => b.reconciliationItems(ORG, 'rec_nope')).toThrow(expect.objectContaining({ code: 'reconciliation.run_not_found' }));
  });
});

// Why: "Mis pagos" and on-us payments move a person's own money; the mock must answer with the
// API's rules (verified DNI only, own links only, installment checked, instant settlement)
// or the screens are demoed against behaviour production will refuse (R-MP-1..5, R-CVU-4).
describe('Mis pagos', () => {
  const PERSONAL = 'org_personal';
  const personalAvailable = (b: MockBackend, currency: 'ARS' | 'USD') =>
    b.listAccounts(PERSONAL).find((item) => item.kind === 'personal')?.balances.find((item) => item.currency === currency)?.available ?? NaN;

  it('links the companies whose payer carries the verified DNI (DNI or CUIT middle digits)', () => {
    const b = backend();
    const companies = b.myPaymentCompanies(false);
    expect(companies.map((item) => item.companyName).sort()).toEqual(['Constructora Andina SRL', 'Desarrollos Del Sur SA']);
    const andina = companies.find((item) => item.companyName === 'Constructora Andina SRL');
    expect(andina).toMatchObject({ projectName: 'Edificio Belgrano R', erpLinked: true, hidden: false, totalPaid: [{ currency: 'ARS', amount: 1_737_500_00 }] });
    expect(andina?.cvus.map((cvu) => cvu.currency)).toEqual(['ARS', 'USD']);
    // Reading again never duplicates the links.
    expect(b.myPaymentCompanies(false)).toHaveLength(2);
  });

  it('hides and shows a company; hidden ones only come with includeHidden', () => {
    const b = backend();
    const [first] = b.myPaymentCompanies(false);
    b.setMyPaymentHidden(first?.linkId ?? '', true);
    expect(b.myPaymentCompanies(false)).toHaveLength(1);
    expect(b.myPaymentCompanies(true).find((item) => item.linkId === first?.linkId)?.hidden).toBe(true);
    b.setMyPaymentHidden(first?.linkId ?? '', false);
    expect(b.myPaymentCompanies(false)).toHaveLength(2);
  });

  it('answers installments only for an ERP-linked company', () => {
    const b = backend();
    const companies = b.myPaymentCompanies(false);
    const andina = companies.find((item) => item.erpLinked);
    const dds = companies.find((item) => !item.erpLinked);
    const plan = b.myPaymentInstallments(andina?.linkId ?? '');
    expect(plan.erpLinked).toBe(true);
    expect(plan.items.map((item) => item.status).slice(0, 5)).toEqual(['paid', 'paid', 'paid', 'overdue', 'pending']);
    expect(b.myPaymentInstallments(dds?.linkId ?? '')).toEqual({ erpLinked: false, items: [] });
  });

  it('pays an installment from the personal wallet instantly (on-us) and credits the company', () => {
    const b = backend();
    const andina = b.myPaymentCompanies(false).find((item) => item.erpLinked);
    const before = personalAvailable(b, 'ARS');
    const payout = b.payMyPaymentCompany(andina?.linkId ?? '', { currency: 'ARS', amount: 1_000_00, installmentNumber: 4 }, creds('mp1'));
    expect(payout).toMatchObject({ status: 'settled', rail: 'internal', concept: 'Cuota 4 - Constructora Andina SRL', amount: 1_000_00, fee: 0 });
    expect(personalAvailable(b, 'ARS')).toBe(before - 1_000_00);
    expect(b.myPaymentCompanies(false).find((item) => item.erpLinked)?.totalPaid).toEqual([{ currency: 'ARS', amount: 1_738_500_00 }]);
    // Same key, same body: the same payout, the money moves once.
    expect(b.payMyPaymentCompany(andina?.linkId ?? '', { currency: 'ARS', amount: 1_000_00, installmentNumber: 4 }, creds('mp1')).id).toBe(payout.id);
    expect(personalAvailable(b, 'ARS')).toBe(before - 1_000_00);
  });

  it('refuses an unknown installment, a currency without CVU and a foreign link', () => {
    const b = backend();
    const companies = b.myPaymentCompanies(false);
    const andina = companies.find((item) => item.erpLinked);
    const dds = companies.find((item) => !item.erpLinked);
    expect(() => b.payMyPaymentCompany(andina?.linkId ?? '', { currency: 'ARS', amount: 1_00, installmentNumber: 999 }, creds('mp2'))).toThrow(
      expect.objectContaining({ code: 'installment.not_found' }),
    );
    expect(() => b.payMyPaymentCompany(dds?.linkId ?? '', { currency: 'USD', amount: 1_00 }, creds('mp3'))).toThrow(expect.objectContaining({ code: 'money.currency_mismatch' }));
    expect(() => b.myPaymentInstallments('lnk_someone_else')).toThrow(expect.objectContaining({ code: 'payer_link.not_found' }));
  });

  it('is closed to a person without a verified personal wallet', () => {
    const b = backend();
    b.login({ email: 'martina@desarrollosdelsur.com.ar', password: 'secreto1' });
    expect(() => b.myPaymentCompanies(false)).toThrow(expect.objectContaining({ code: 'kyc.not_approved' }));
  });
});

describe('personal wallet and on-us rail', () => {
  it('signup opens a personal wallet in KYC draft and a second one is a conflict', () => {
    const b = backend();
    b.signup({ email: 'nueva@persona.com', password: 'secreto1234' });
    b.verifyEmail({ email: 'nueva@persona.com', code: '123456' });
    b.login({ email: 'nueva@persona.com', password: 'secreto1234' });
    const me = b.me();
    expect(me.personalModeEnabled).toBe(true);
    expect(me.memberships).toEqual([expect.objectContaining({ orgKind: 'personal', kycStatus: 'draft', orgName: 'nueva@persona.com' })]);
    const personal = me.memberships[0]?.orgId ?? '';
    expect(b.listAccounts(personal)).toEqual([expect.objectContaining({ kind: 'personal', cvus: [] })]);
    expect(() => b.createOrg({ kind: 'personal', legalName: 'Otra' })).toThrow(expect.objectContaining({ code: 'resource.conflict' }));
  });

  it('edits the alias of the wallet CVU and refuses one already taken', () => {
    const b = backend();
    expect(b.updateAccountCvu('org_personal', 'acc_personal', 'acvu_personal_ars', 'Erick.Nuevo').alias).toBe('erick.nuevo');
    expect(() => b.updateAccountCvu('org_personal', 'acc_personal', 'acvu_personal_usd', 'juan.perez.torre')).toThrow(expect.objectContaining({ code: 'cvu.alias_taken' }));
    expect(() => b.updateAccountCvu('org_personal', 'acc_personal', 'cvu_pay_juan_0', 'erick.otro')).toThrow(expect.objectContaining({ code: 'cvu.not_found' }));
  });

  it('quotes a Lenapay alias as internal and a bank alias as bank', () => {
    const b = backend();
    expect(b.quote(ORG, { accountId: 'acc_main', currency: 'ARS', amount: 1_000_00, to: 'erick.jimenez.lnp' }).rail).toBe('internal');
    expect(b.quote(ORG, { accountId: 'acc_main', currency: 'ARS', amount: 1_000_00, to: 'hormigon.plata' }).rail).toBe('bank');
  });

  it('settles a company on-us payout at approval, crediting the receiver once', () => {
    pending.length = 0;
    const b = backend();
    b.login({ email: 'martina@desarrollosdelsur.com.ar', password: 'secreto1' });
    const payout = b.createPayout(ORG, { accountId: 'acc_main', currency: 'ARS', amount: 5_000_00, to: 'erick.jimenez.lnp', concept: 'Reintegro' }, creds('ou1'));
    expect(payout).toMatchObject({ status: 'pending_approval', rail: 'internal' });
    b.login({ email: 'erick@desarrollosdelsur.com.ar', password: 'secreto1' });
    const personalBefore = b.listAccounts('org_personal')[0]?.balances.find((item) => item.currency === 'ARS')?.posted ?? NaN;
    expect(b.approvePayout(ORG, payout.id, creds('ou2')).status).toBe('settled');
    expect(pending).toHaveLength(0);
    expect(b.listAccounts('org_personal')[0]?.balances.find((item) => item.currency === 'ARS')?.posted).toBe(personalBefore + 5_000_00);
    const movement = b.listMovements('org_personal', { type: 'payin' }, null).items[0];
    expect(movement).toMatchObject({ rail: 'internal', amount: 5_000_00, counterparty: 'Desarrollos Del Sur SA' });
  });
});
