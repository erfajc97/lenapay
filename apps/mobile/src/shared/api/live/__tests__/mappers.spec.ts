import type { components } from '@lenapay/api-types';
import { describe, expect, it } from 'vitest';

import {
  type OrgDirectory,
  parseRealtimeFrame,
  toAccount,
  toErpLink,
  toInstallmentList,
  toMe,
  toMember,
  toMovement,
  toMyPaymentCompany,
  toMyPaymentPayin,
  toOwnershipTransfer,
  toPayin,
  toPayout,
  toQuote,
  toReconciliationItem,
  toReconciliationRun,
} from '../mappers';

type Schemas = components['schemas'];

/**
 * Fixtures are typed against the contract field by field but may omit members: the contract
 * grows members the mappers do not read, and these specs pin only what the mappers translate.
 */
const asContract = <T,>(value: Partial<T>): T => value as T;

// Why: the live client is the only place where contract money `{minor, currency}` becomes the
// app's minor units; a swapped field (fee for amount, gross for net) shows the wrong money on
// every screen, and a nullable member read as a string crashes the screen in live mode only.
const ars = (minor: number) => ({ minor, currency: 'ARS' as const });
const directory: OrgDirectory = {
  accountName: (id) => ({ 'acc-1': 'Torre Palermo' })[id] ?? '',
  memberLabel: (id) => ({ 'u-creator': 'approver@lenapay.app' })[id] ?? null,
};

const payoutDto = asContract<Schemas['Payout']>({
  id: 'po-1',
  accountId: 'acc-1',
  amount: ars(18_500_000),
  fee: ars(52_000),
  total: ars(18_552_000),
  destination: { type: 'alias', value: 'ascensores.norte', accountNumber: '0070999007153818841095', holderName: 'Lucía Fernández', holderCuit: '20714469518', bankName: null },
  concept: 'Anticipo ascensores',
  status: 'pending_approval',
  createdBy: 'u-creator',
  approvedBy: 'u-gone',
  rejectReason: null,
  failureCode: null,
  events: [{ fromStatus: null, toStatus: 'pending_approval', actorId: 'u-creator', reason: null, createdAt: '2026-11-07T23:50:24Z' }],
  attachments: [],
  createdAt: '2026-11-07T23:50:24Z',
  updatedAt: '2026-11-07T23:50:24Z',
  settledAt: null,
});

const payinDto = asContract<Schemas['Payin']>({
  id: 'pin-1',
  accountId: 'acc-1',
  payerId: null,
  cvuId: 'cvu-1',
  gross: ars(15_000_000),
  fee: ars(90_000),
  net: ars(14_910_000),
  origin: { name: 'Juan Pérez', cuit: '20284567898', cbu: null },
  providerTransferId: 'BIND-1',
  status: 'credited',
  createdAt: '2026-11-07T12:00:00Z',
  creditedAt: '2026-11-07T12:00:01Z',
});

describe('contract → view model', () => {
  it('maps a payout: money, names joined from the directory, events and nulls', () => {
    const payout = toPayout(payoutDto, directory);
    expect(payout).toMatchObject({ currency: 'ARS', amount: 18_500_000, fee: 52_000, total: 18_552_000, accountName: 'Torre Palermo', rejectReason: null });
    expect(payout.createdBy).toEqual({ userId: 'u-creator', label: 'approver@lenapay.app' });
    expect(payout.approvedBy).toEqual({ userId: 'u-gone', label: null });
    expect(payout.events).toEqual([{ status: 'pending_approval', at: '2026-11-07T23:50:24Z' }]);
    expect(payout.destination.bankName).toBeNull();
    expect(toPayout({ ...payoutDto, approvedBy: null }, directory).approvedBy).toBeNull();
  });

  it('maps a payin: gross as the amount, fee and net apart, the origin as the sender', () => {
    expect(toPayin(payinDto, directory)).toEqual({
      id: 'pin-1', accountId: 'acc-1', accountName: 'Torre Palermo', payerId: null, cvuId: 'cvu-1', currency: 'ARS',
      amount: 15_000_000, fee: 90_000, net: 14_910_000, originName: 'Juan Pérez', originCuit: '20284567898', createdAt: '2026-11-07T12:00:00Z',
    });
  });

  it('maps a movement with a null fee and a nameless counterparty, and one with both', () => {
    const transfer = asContract<Schemas['Movement']>({
      id: 'mv-1', type: 'internal_transfer', status: 'completed', direction: 'in', accountId: 'acc-unknown', amount: ars(25_000_000), fee: null,
      counterparty: { name: null, cuit: null, cbu: null }, concept: null, refId: 'tr-1', createdAt: '2026-11-07T23:50:25Z',
    });
    expect(toMovement(transfer, directory)).toMatchObject({ amount: 25_000_000, currency: 'ARS', fee: null, counterparty: null, counterpartyDoc: null, accountName: '' });
    const payout = asContract<Schemas['Movement']>({ ...transfer, type: 'payout', direction: 'out', accountId: 'acc-1', fee: ars(34_700), counterparty: { name: 'Martín Díaz', cuit: '20752817953', cbu: '0070' } });
    expect(toMovement(payout, directory)).toMatchObject({ fee: 34_700, counterparty: 'Martín Díaz', counterpartyDoc: '20752817953', accountName: 'Torre Palermo' });
  });

  it('maps balances and the account flags', () => {
    const account = toAccount(asContract<Schemas['Account']>({
      id: 'acc-1', kind: 'project', name: 'Torre Palermo', archived: true, erpProjectRef: null, createdAt: '2026-11-01T00:00:00Z',
      balances: [{ currency: 'ARS', posted: ars(567_090_350), held: ars(60_651_000), available: ars(506_439_350) }],
    }));
    expect(account.name).toBe('Torre Palermo');
    expect(account).toMatchObject({ archived: true, payersCount: null, balances: [{ currency: 'ARS', posted: 567_090_350, held: 60_651_000, available: 506_439_350 }] });
  });

  // Why: the API seeds "Main account" in English; the es-AR app must show "Cuenta principal".
  it('labels the seeded main account by kind in the active language', () => {
    const account = toAccount(asContract<Schemas['Account']>({ id: 'acc-0', kind: 'main', name: 'Main account', archived: false, erpProjectRef: null, createdAt: '2026-11-01T00:00:00Z', balances: [] }));
    expect(account.name).toBe('Cuenta principal');
  });

  it('maps the quote (available after the hold, negative when it does not fit)', () => {
    expect(toQuote(asContract<Schemas['PayoutQuote']>({ amount: ars(100), fee: ars(15_010), total: ars(15_110), destination: payoutDto.destination, availableAfter: ars(-5) }))).toEqual({ currency: 'ARS', amount: 100, fee: 15_010, total: 15_110, availableAfter: -5 });
  });

  it('derives personal mode from a personal membership (the contract has no flag)', () => {
    const user = { id: 'u', email: 'persona@lenapay.app', fullName: null, emailVerified: true, mfaEnabled: false, createdAt: '2026-11-01T00:00:00Z' };
    const personal = { orgId: 'o', orgName: 'Lucía', orgKind: 'personal' as const, role: 'owner' as const, kycStatus: 'approved' as const };
    expect(toMe({ user, memberships: [personal] })).toMatchObject({ id: 'u', fullName: null, personalModeEnabled: true });
    expect(toMe({ user, memberships: [{ ...personal, orgKind: 'company' }] }).personalModeEnabled).toBe(false);
    expect(toMe({ user, memberships: [] }).personalModeEnabled).toBe(false);
  });

  // Why: names replace emails on Home, Team and approvals (R-ID-5), and the default org/account
  // decides where the app opens; a dropped field shows the email or the wrong org.
  it('carries the full name and the synced preferences, with nothing set when absent', () => {
    const user = { id: 'u', email: 'demo@lenapay.app', fullName: 'Ricardo Benítez', emailVerified: true, mfaEnabled: false, createdAt: 'c' };
    const preferences = { defaultOrgId: 'o', defaultAccountId: 'a', locale: 'en-US' as const, theme: 'dark' as const, updatedAt: 'u' };
    expect(toMe({ user, memberships: [], preferences })).toMatchObject({ fullName: 'Ricardo Benítez', preferences });
    expect(toMe({ user, memberships: [] }).preferences).toEqual({ defaultOrgId: null, defaultAccountId: null, locale: null, theme: null, updatedAt: null });
  });

  it('maps members with their name and the owner role, and both sides of a transfer', () => {
    const member = { userId: 'u', email: 'demo@lenapay.app', fullName: 'Ricardo Benítez', role: 'owner' as const, joinedAt: 'j' };
    expect(toMember(member)).toEqual({ userId: 'u', email: 'demo@lenapay.app', fullName: 'Ricardo Benítez', role: 'owner', status: 'active' });
    expect(toMember({ ...member, fullName: null }).fullName).toBeNull();
    const transfer = toOwnershipTransfer({ previousOwner: { ...member, role: 'admin' }, owner: { ...member, userId: 'v', role: 'owner' } });
    expect(transfer).toMatchObject({ previousOwner: { userId: 'u', role: 'admin' }, owner: { userId: 'v', role: 'owner' } });
  });

  it('maps reconciliation runs and items, with an absent side as null', () => {
    const run = toReconciliationRun({
      id: 'r', currency: 'ARS', day: '2026-11-07', status: 'completed', matchedCount: 13, missingInLedgerCount: 1, missingAtPspCount: 2,
      pspTotal: ars(1_000), ledgerTotal: ars(900), difference: ars(100), createdAt: 'c', finishedAt: null,
    });
    expect(run).toMatchObject({ matched: 13, missingInLedger: 1, missingAtPsp: 2, pspTotal: 1_000, ledgerTotal: 900, difference: 100, finishedAt: null });
    expect(toReconciliationItem({ id: 'i', status: 'missing_in_ledger', providerTransferId: 'B', direction: 'in', pspAmount: ars(85), ledgerAmount: null, createdAt: 'c' })).toMatchObject({ pspAmount: 85, ledgerAmount: null });
  });

  it('reads every ERP import state, and "not linked" from the 404', () => {
    const link = { id: 'l', erpOrgId: 'erp-1', displayName: 'DDS', linkedBy: 'u', linkedAt: 'a', importedAt: null };
    expect(toErpLink(null).status).toBe('not_linked');
    expect(toErpLink({ ...link, importStatus: 'pending' }).status).toBe('importing');
    expect(toErpLink({ ...link, importStatus: 'running' }).status).toBe('importing');
    expect(toErpLink({ ...link, importStatus: 'completed' }).status).toBe('linked');
    expect(toErpLink({ ...link, importStatus: 'failed' })).toEqual({ status: 'failed', displayName: 'DDS', linkedAt: 'a', importedAt: null, importSummary: null });
  });

  // Why: the ERP screen lists who was invited and with which role (R-ERP-4); losing the summary
  // hides that people were invited on the owner's behalf.
  it('keeps the import summary once the import ran', () => {
    const importSummary = {
      projects: 2, buyers: 5, skippedBuyers: 0, usersInvited: 1, usersAlreadyMembers: 0, usersWithoutPaymentsAccess: 1,
      invitedUsers: [{ email: 'tesoreria@constructoraandina.com.ar', role: 'operator' as const }],
    };
    const link = { id: 'l', erpOrgId: 'erp-2', displayName: 'Andina', linkedBy: 'u', linkedAt: 'a', importedAt: 'i', importStatus: 'completed' as const };
    expect(toErpLink({ ...link, importSummary }).importSummary).toEqual(importSummary);
    expect(toErpLink({ ...link, importSummary: null }).importSummary).toBeNull();
  });
});

// Why: the SSE frames are unnamed `message` events whose JSON is the RealtimeEvent envelope;
// a parser that expects the old `{type, payin}` shape drops every live payin and approval.
describe('parseRealtimeFrame', () => {
  const frame = (type: string, data: Record<string, unknown>) => JSON.stringify({ type, orgId: 'o', occurredAt: '2026-11-07T12:00:00Z', data, ...data });

  it('parses payin.credited into a payin with its account name', () => {
    expect(parseRealtimeFrame(frame('payin.credited', { payinId: 'pin-1', accountId: 'acc-1', payin: payinDto }), directory)).toMatchObject({ type: 'payin.credited', payin: { id: 'pin-1', accountName: 'Torre Palermo', net: 14_910_000 } });
  });

  it('parses payout.updated and approval.requested into the payout', () => {
    expect(parseRealtimeFrame(frame('payout.updated', { payoutId: 'po-1', status: 'settled', payout: { ...payoutDto, status: 'settled' } }), directory)).toMatchObject({ type: 'payout.updated', payout: { id: 'po-1', status: 'settled' } });
    expect(parseRealtimeFrame(frame('approval.requested', { payoutId: 'po-1', payout: payoutDto }), directory)).toMatchObject({ type: 'approval.requested', payout: { id: 'po-1' } });
  });

  it('parses balance.changed with or without an account', () => {
    expect(parseRealtimeFrame(frame('balance.changed', { accountId: 'acc-1' }), directory)).toEqual({ type: 'balance.changed', accountId: 'acc-1' });
    expect(parseRealtimeFrame(JSON.stringify({ type: 'balance.changed', orgId: 'o', occurredAt: 'x', data: {} }), directory)).toEqual({ type: 'balance.changed', accountId: null });
  });

  it('reads the payload from the top level when `data` is missing', () => {
    expect(parseRealtimeFrame(JSON.stringify({ type: 'payin.credited', payin: payinDto }), directory)).toMatchObject({ type: 'payin.credited' });
  });

  it('ignores unknown types, missing payloads, malformed JSON and heartbeats', () => {
    expect(parseRealtimeFrame(frame('payer.created', {}), directory)).toBeNull();
    expect(parseRealtimeFrame(frame('payout.updated', { payoutId: 'po-1' }), directory)).toBeNull();
    expect(parseRealtimeFrame(frame('payin.credited', { payin: 'nope' }), directory)).toBeNull();
    expect(parseRealtimeFrame('{not json', directory)).toBeNull();
    expect(parseRealtimeFrame('[1,2]', directory)).toBeNull();
    expect(parseRealtimeFrame('', directory)).toBeNull();
    expect(parseRealtimeFrame(null, directory)).toBeNull();
  });
});

// Why: the rail decides whether the app promises "instantánea · sin costo"; "Mis pagos" money
// arrives as `{minor, currency}` lists and nullable dates. A dropped rail or a swapped total
// tells a person the wrong thing about their own money (R-MP-2, R-MP-3, R-MP-5, R-CVU-4).
describe('rails, personal CVUs and Mis pagos', () => {
  it('carries the rail of payouts, quotes and movements as the API sends it', () => {
    expect(toPayout({ ...payoutDto, rail: 'internal' }, directory).rail).toBe('internal');
    expect(toPayout({ ...payoutDto, rail: 'bank' }, directory).rail).toBe('bank');
    const quote = asContract<Schemas['PayoutQuote']>({ amount: ars(100), fee: ars(0), total: ars(100), destination: payoutDto.destination, availableAfter: ars(5), rail: 'internal' });
    expect(toQuote(quote).rail).toBe('internal');
    const movement = asContract<Schemas['Movement']>({
      id: 'mv-2', type: 'payin', status: 'completed', direction: 'in', accountId: 'acc-1', amount: ars(1), fee: null,
      counterparty: { name: null, cuit: null, cbu: null }, concept: null, refId: 'pin-2', rail: 'internal', createdAt: '2026-11-07T23:50:25Z',
    });
    expect(toMovement(movement, directory).rail).toBe('internal');
  });

  it("keeps the account's own CVUs, and [] for a company account", () => {
    const cvu = { id: 'acvu-1', currency: 'USD' as const, cvu: '0000322100000010000157', alias: null, status: 'active' as const, share: { link: 'https://x', qrPayload: 'q' } };
    const base = { id: 'acc-p', kind: 'personal' as const, name: 'Personal account', archived: false, erpProjectRef: null, createdAt: '2026-11-01T00:00:00Z', balances: [] };
    expect(toAccount(asContract<Schemas['Account']>({ ...base, cvus: [cvu] })).cvus).toEqual([cvu]);
    expect(toAccount(asContract<Schemas['Account']>({ ...base, cvus: [] })).cvus).toEqual([]);
  });

  it('maps a company of Mis pagos: totals per currency in minor units, nullable project and date', () => {
    const company = toMyPaymentCompany({
      linkId: 'lnk-1', companyName: 'Constructora Andina SRL', projectName: null, erpLinked: true, hidden: false, lastPaymentAt: null,
      cvus: [{ currency: 'ARS', cvu: '0000322100000010000133', alias: 'rio.cerro.0013' }],
      totalPaid: [ars(170_000_000), { minor: 1_250_00, currency: 'USD' }],
    });
    expect(company).toEqual({
      linkId: 'lnk-1', companyName: 'Constructora Andina SRL', projectName: null, erpLinked: true, hidden: false, lastPaymentAt: null,
      cvus: [{ currency: 'ARS', cvu: '0000322100000010000133', alias: 'rio.cerro.0013' }],
      totalPaid: [{ currency: 'ARS', amount: 170_000_000 }, { currency: 'USD', amount: 1_250_00 }],
    });
  });

  it('maps my payins and installments with their money and nullable fields', () => {
    expect(toMyPaymentPayin({ id: 'p1', cvu: '0000322100000010000133', amount: ars(85_000_000), status: 'received', createdAt: '2026-11-07T01:00:00Z', creditedAt: null })).toEqual({
      id: 'p1', cvu: '0000322100000010000133', currency: 'ARS', amount: 85_000_000, status: 'received', createdAt: '2026-11-07T01:00:00Z', creditedAt: null,
    });
    const plan = toInstallmentList({
      erpLinked: true,
      items: [
        { number: 4, dueDate: '2026-10-10', amount: ars(88_750_000), status: 'overdue', adjustmentIndex: 'CAC' },
        { number: 5, dueDate: '2026-11-10', amount: ars(90_000_000), status: 'pending', adjustmentIndex: null },
      ],
    });
    expect(plan).toEqual({
      erpLinked: true,
      items: [
        { number: 4, dueDate: '2026-10-10', currency: 'ARS', amount: 88_750_000, status: 'overdue', adjustmentIndex: 'CAC' },
        { number: 5, dueDate: '2026-11-10', currency: 'ARS', amount: 90_000_000, status: 'pending', adjustmentIndex: null },
      ],
    });
    expect(toInstallmentList({ erpLinked: false, items: [] })).toEqual({ erpLinked: false, items: [] });
  });
});
