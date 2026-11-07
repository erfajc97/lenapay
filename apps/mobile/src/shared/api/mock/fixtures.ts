import { buildCbu, buildCvu } from '@/shared/helpers/cbu';

import type { Currency, Destination, KycCase, PayoutStatus, ReconciliationItem, ReconciliationRun } from '../types';
import { payinFee, payoutFee } from './feePlan';
import type { MockAccount, MockPayer, MockPayin, MockPayout, MockState, MockTransfer } from './state';

/**
 * Realistic demo data for `EXPO_PUBLIC_API_MODE=mock`: Desarrollos Del Sur SA (two live
 * projects, payers with CVUs, payins, payouts waiting for approval, a reconciliation day with
 * a difference and an ERP link candidate) plus a smaller second org and a personal wallet.
 * Dates are relative to `now` so the demo always looks fresh.
 */
const PSP_PREFIX = '0031';
const HOUR = 3_600_000;
const DAY = 24 * HOUR;

export const DEMO_USER_ID = 'u_erick';
const DEMO_ORG_ID = 'org_dds';
const ANDINA_ORG_ID = 'org_andina';
const PERSONAL_ORG_ID = 'org_personal';

const users = {
  erick: { userId: 'u_erick', fullName: 'Erick Jiménez' },
  martina: { userId: 'u_martina', fullName: 'Martina López' },
  pablo: { userId: 'u_pablo', fullName: 'Pablo Sánchez' },
  diego: { userId: 'u_diego', fullName: 'Diego Ferrari' },
};

/** Supplier accounts the payout flow can resolve (bank CBUs and aliases). */
export const DIRECTORY: Destination[] = [
  supplier('0070999', '4000123456789', 'hormigon.plata', 'Hormigonera del Plata SA', '30715554441', 'Banco Galicia'),
  supplier('0720123', '0000987654321', 'estudio.bianchi', 'Estudio Arq. Bianchi', '20264445559', 'Banco Santander'),
  supplier('0110456', '0000555444333', 'corralon.sanmartin', 'Corralón San Martín SA', '30698765433', 'Banco Nación'),
  supplier('0170200', '0000777888999', 'aceros.rosario', 'Aceros Rosario SA', '30714443336', 'BBVA'),
  supplier('2850590', '0000112233445', 'electro.norte', 'Electro Norte SA', '30705556667', 'Banco Macro'),
];

function supplier(entityBranch: string, account: string, alias: string, holderName: string, holderCuit: string, bankName: string): Destination {
  return { type: 'cbu', cbu: buildCbu(entityBranch, account), alias, holderName, holderCuit, bankName };
}

function at(now: number, daysAgo: number, hour: number, minute = 0): string {
  const date = new Date(now - daysAgo * DAY);
  date.setHours(hour, minute, 0, 0);
  return date.toISOString();
}

function ledger(ars: number, usd: number): MockAccount['ledger'] {
  return { ARS: { posted: ars, held: 0 }, USD: { posted: usd, held: 0 } };
}

function payer(params: {
  id: string;
  orgId: string;
  accountId: string;
  name: string;
  docType: 'CUIT' | 'DNI';
  docNumber: string;
  email: string | null;
  cvus: { currency: Currency; alias: string; account: string }[];
  createdAt: string;
}): MockPayer {
  return {
    id: params.id,
    orgId: params.orgId,
    accountId: params.accountId,
    name: params.name,
    docType: params.docType,
    docNumber: params.docNumber,
    email: params.email,
    erpRef: null,
    createdAt: params.createdAt,
    cvus: params.cvus.map((cvu, index) => ({
      id: `cvu_${params.id}_${index}`,
      payerId: params.id,
      currency: cvu.currency,
      cvu: buildCvu(PSP_PREFIX, cvu.account),
      alias: cvu.alias,
      status: 'active',
    })),
  };
}

function seedAccounts(now: number): MockAccount[] {
  return [
    { id: 'acc_main', orgId: DEMO_ORG_ID, kind: 'main', name: 'Cuenta principal', archivedAt: null, ledger: ledger(48_250_000_00, 85_000_00) },
    { id: 'acc_torre', orgId: DEMO_ORG_ID, kind: 'project', name: 'Torre Palermo', archivedAt: null, ledger: ledger(125_430_500_00, 312_000_00) },
    { id: 'acc_nordelta', orgId: DEMO_ORG_ID, kind: 'project', name: 'Barrio Nordelta Lote 12', archivedAt: null, ledger: ledger(36_780_000_00, 54_500_00) },
    { id: 'acc_caballito', orgId: DEMO_ORG_ID, kind: 'project', name: 'Edificio Caballito', archivedAt: at(now, 60, 10), ledger: ledger(0, 0) },
    { id: 'acc_andina_main', orgId: ANDINA_ORG_ID, kind: 'main', name: 'Cuenta principal', archivedAt: null, ledger: ledger(12_400_000_00, 9_800_00) },
    { id: 'acc_belgrano', orgId: ANDINA_ORG_ID, kind: 'project', name: 'Edificio Belgrano R', archivedAt: null, ledger: ledger(22_150_000_00, 0) },
    { id: 'acc_personal', orgId: PERSONAL_ORG_ID, kind: 'personal', name: 'Mi cuenta', archivedAt: null, ledger: ledger(0, 0) },
  ];
}

function seedPayers(now: number): MockPayer[] {
  const created = at(now, 45, 11);
  return [
    payer({ id: 'pay_juan', orgId: DEMO_ORG_ID, accountId: 'acc_torre', name: 'Juan Pérez', docType: 'DNI', docNumber: '28456789', email: 'juan.perez@gmail.com', createdAt: created,
      cvus: [{ currency: 'ARS', alias: 'juan.perez.torre', account: '1000000000101' }, { currency: 'USD', alias: 'juan.perez.torre.usd', account: '1000000000102' }] }),
    payer({ id: 'pay_maria', orgId: DEMO_ORG_ID, accountId: 'acc_torre', name: 'María González', docType: 'CUIT', docNumber: '27334567899', email: 'maria.gonzalez@hotmail.com', createdAt: created,
      cvus: [{ currency: 'ARS', alias: 'maria.gonzalez.tp', account: '1000000000201' }] }),
    payer({ id: 'pay_lago', orgId: DEMO_ORG_ID, accountId: 'acc_torre', name: 'Inversiones Lago SRL', docType: 'CUIT', docNumber: '30709876542', email: 'admin@inversioneslago.com.ar', createdAt: created,
      cvus: [{ currency: 'ARS', alias: 'inv.lago.torre', account: '1000000000301' }, { currency: 'USD', alias: 'inv.lago.torre.usd', account: '1000000000302' }] }),
    payer({ id: 'pay_carlos', orgId: DEMO_ORG_ID, accountId: 'acc_nordelta', name: 'Carlos Rodríguez', docType: 'CUIT', docNumber: '20319876546', email: 'crodriguez@outlook.com', createdAt: created,
      cvus: [{ currency: 'ARS', alias: 'carlos.rod.nordelta', account: '1000000000401' }] }),
    payer({ id: 'pay_lucia', orgId: DEMO_ORG_ID, accountId: 'acc_nordelta', name: 'Lucía Fernández', docType: 'CUIT', docNumber: '27351112226', email: null, createdAt: created,
      cvus: [{ currency: 'ARS', alias: 'lucia.fer.nordelta', account: '1000000000501' }, { currency: 'USD', alias: 'lucia.fer.ndt.usd', account: '1000000000502' }] }),
    payer({ id: 'pay_roberto', orgId: ANDINA_ORG_ID, accountId: 'acc_belgrano', name: 'Roberto Silva', docType: 'CUIT', docNumber: '20389991113', email: 'rsilva@gmail.com', createdAt: created,
      cvus: [{ currency: 'ARS', alias: 'roberto.silva.br', account: '1000000000601' }] }),
  ];
}

const PAYIN_PLAN: { payerId: string; currency: Currency; major: number; daysAgo: number; hour: number }[] = [
  { payerId: 'pay_maria', currency: 'ARS', major: 850_000, daysAgo: 1, hour: 10 },
  { payerId: 'pay_lago', currency: 'USD', major: 12_500, daysAgo: 2, hour: 15 },
  { payerId: 'pay_carlos', currency: 'ARS', major: 1_200_000, daysAgo: 3, hour: 9 },
  { payerId: 'pay_juan', currency: 'ARS', major: 150_000, daysAgo: 4, hour: 18 },
  { payerId: 'pay_lucia', currency: 'USD', major: 3_200, daysAgo: 6, hour: 12 },
  { payerId: 'pay_lucia', currency: 'ARS', major: 640_000, daysAgo: 8, hour: 11 },
  { payerId: 'pay_juan', currency: 'USD', major: 1_500, daysAgo: 10, hour: 16 },
  { payerId: 'pay_maria', currency: 'ARS', major: 850_000, daysAgo: 12, hour: 10 },
  { payerId: 'pay_carlos', currency: 'ARS', major: 1_200_000, daysAgo: 15, hour: 9 },
  { payerId: 'pay_lago', currency: 'ARS', major: 4_500_000, daysAgo: 18, hour: 14 },
  { payerId: 'pay_juan', currency: 'ARS', major: 150_000, daysAgo: 22, hour: 19 },
  { payerId: 'pay_maria', currency: 'ARS', major: 850_000, daysAgo: 26, hour: 10 },
  { payerId: 'pay_lucia', currency: 'ARS', major: 640_000, daysAgo: 29, hour: 11 },
  { payerId: 'pay_roberto', currency: 'ARS', major: 2_300_000, daysAgo: 2, hour: 13 },
  { payerId: 'pay_roberto', currency: 'ARS', major: 2_300_000, daysAgo: 32, hour: 13 },
];

function originCuitOf(payerRecord: Pick<MockPayer, 'docType' | 'docNumber'>): string {
  return payerRecord.docType === 'CUIT' ? payerRecord.docNumber : `20${payerRecord.docNumber}8`;
}

export function buildPayin(params: { id: string; payer: MockPayer; accountName: string; currency: Currency; amount: number; createdAt: string }): MockPayin {
  const cvu = params.payer.cvus.find((item) => item.currency === params.currency) ?? params.payer.cvus[0];
  const fee = payinFee(params.amount, params.currency);
  return {
    id: params.id,
    orgId: params.payer.orgId,
    cvuId: cvu?.id ?? '',
    payerId: params.payer.id,
    payerName: params.payer.name,
    accountId: params.payer.accountId,
    accountName: params.accountName,
    currency: params.currency,
    amount: params.amount,
    fee,
    net: params.amount - fee,
    originName: params.payer.name,
    originCuit: originCuitOf(params.payer),
    createdAt: params.createdAt,
  };
}

function seedPayins(now: number, payers: MockPayer[], accounts: MockAccount[]): MockPayin[] {
  return PAYIN_PLAN.map((plan, index) => {
    const payerRecord = payers.find((item) => item.id === plan.payerId);
    if (!payerRecord) throw new Error(`fixture payer ${plan.payerId}`);
    const accountName = accounts.find((item) => item.id === payerRecord.accountId)?.name ?? '';
    return buildPayin({ id: `pin_${index + 1}`, payer: payerRecord, accountName, currency: plan.currency, amount: plan.major * 100, createdAt: at(now, plan.daysAgo, plan.hour, 7 * index) });
  });
}

const PAYOUT_PLAN: {
  id: string;
  orgId: string;
  accountId: string;
  supplier: number;
  currency: Currency;
  major: number;
  concept: string;
  status: PayoutStatus;
  createdBy: keyof typeof users;
  decidedBy?: keyof typeof users;
  hoursAgo: number;
  rejectionReason?: string;
}[] = [
  { id: 'po_1', orgId: DEMO_ORG_ID, accountId: 'acc_main', supplier: 0, currency: 'ARS', major: 3_250_000, concept: 'Hormigón H30 – Torre Palermo', status: 'settled', createdBy: 'martina', decidedBy: 'erick', hoursAgo: 50 },
  { id: 'po_2', orgId: DEMO_ORG_ID, accountId: 'acc_torre', supplier: 1, currency: 'ARS', major: 980_000, concept: 'Honorarios dirección de obra octubre', status: 'settled', createdBy: 'erick', decidedBy: 'pablo', hoursAgo: 122 },
  { id: 'po_3', orgId: DEMO_ORG_ID, accountId: 'acc_nordelta', supplier: 2, currency: 'ARS', major: 1_540_000, concept: 'Materiales lote 12', status: 'settled', createdBy: 'martina', decidedBy: 'pablo', hoursAgo: 218 },
  { id: 'po_4', orgId: DEMO_ORG_ID, accountId: 'acc_torre', supplier: 3, currency: 'USD', major: 8_400, concept: 'Hierro Ø12 – anticipo', status: 'settled', createdBy: 'martina', decidedBy: 'erick', hoursAgo: 338 },
  { id: 'po_5', orgId: DEMO_ORG_ID, accountId: 'acc_main', supplier: 4, currency: 'ARS', major: 1_200_000, concept: 'Tableros eléctricos – piso 4', status: 'pending_approval', createdBy: 'martina', hoursAgo: 3 },
  { id: 'po_6', orgId: DEMO_ORG_ID, accountId: 'acc_torre', supplier: 0, currency: 'ARS', major: 2_100_000, concept: 'Segunda entrega de hormigón', status: 'pending_approval', createdBy: 'erick', hoursAgo: 1 },
  { id: 'po_7', orgId: DEMO_ORG_ID, accountId: 'acc_nordelta', supplier: 1, currency: 'USD', major: 1_200, concept: 'Planos de instalación sanitaria', status: 'pending_approval', createdBy: 'martina', hoursAgo: 26 },
  { id: 'po_8', orgId: DEMO_ORG_ID, accountId: 'acc_main', supplier: 2, currency: 'ARS', major: 430_000, concept: 'Cemento y cal – reposición', status: 'sending', createdBy: 'martina', decidedBy: 'pablo', hoursAgo: 0.3 },
  { id: 'po_9', orgId: DEMO_ORG_ID, accountId: 'acc_torre', supplier: 2, currency: 'ARS', major: 210_000, concept: 'Pintura látex exterior', status: 'failed', createdBy: 'erick', decidedBy: 'pablo', hoursAgo: 150 },
  { id: 'po_10', orgId: DEMO_ORG_ID, accountId: 'acc_main', supplier: 1, currency: 'ARS', major: 500_000, concept: 'Honorarios – ajuste', status: 'rejected', createdBy: 'martina', decidedBy: 'pablo', hoursAgo: 264, rejectionReason: 'Monto duplicado con la factura de octubre.' },
  { id: 'po_11', orgId: ANDINA_ORG_ID, accountId: 'acc_belgrano', supplier: 3, currency: 'ARS', major: 3_800_000, concept: 'Estructura metálica – cuota 2', status: 'pending_approval', createdBy: 'diego', hoursAgo: 5 },
];

const STATUS_PATH: Record<PayoutStatus, PayoutStatus[]> = {
  pending_approval: ['pending_approval'],
  approved: ['pending_approval', 'approved'],
  sending: ['pending_approval', 'approved', 'sending'],
  settled: ['pending_approval', 'approved', 'sending', 'settled'],
  failed: ['pending_approval', 'approved', 'sending', 'failed'],
  rejected: ['pending_approval', 'rejected'],
  cancelled: ['pending_approval', 'cancelled'],
};

function seedPayouts(now: number, accounts: MockAccount[]): MockPayout[] {
  return PAYOUT_PLAN.map((plan) => {
    const created = now - plan.hoursAgo * HOUR;
    const amount = plan.major * 100;
    const destination = DIRECTORY[plan.supplier];
    if (!destination) throw new Error(`fixture supplier ${plan.supplier}`);
    return {
      id: plan.id,
      orgId: plan.orgId,
      accountId: plan.accountId,
      accountName: accounts.find((item) => item.id === plan.accountId)?.name ?? '',
      currency: plan.currency,
      amount,
      fee: payoutFee(amount, plan.currency),
      destination,
      concept: plan.concept,
      attachmentName: plan.id === 'po_5' ? 'factura-electro-norte-0012.pdf' : null,
      status: plan.status,
      createdBy: users[plan.createdBy],
      decidedBy: plan.decidedBy ? users[plan.decidedBy] : null,
      rejectionReason: plan.rejectionReason ?? null,
      createdAt: new Date(created).toISOString(),
      events: STATUS_PATH[plan.status].map((status, index) => ({ status, at: new Date(created + index * 0.1 * HOUR).toISOString() })),
    };
  });
}

/** Payouts in flight hold amount + fee on their account, like the ledger does (R-OUT-2). */
function isHolding(status: PayoutStatus): boolean {
  return status === 'pending_approval' || status === 'approved' || status === 'sending';
}

function applySeedEffects(accounts: MockAccount[], payouts: MockPayout[]): void {
  for (const payout of payouts) {
    const account = accounts.find((item) => item.id === payout.accountId);
    if (account && isHolding(payout.status)) account.ledger[payout.currency].held += payout.amount + payout.fee;
  }
}

function seedTransfers(now: number): MockTransfer[] {
  return [
    { id: 'tr_1', orgId: DEMO_ORG_ID, fromAccountId: 'acc_torre', toAccountId: 'acc_main', currency: 'ARS', amount: 5_000_000_00, concept: 'Fondo operativo noviembre', createdAt: at(now, 7, 9, 30) },
  ];
}

function kycCase(params: { orgId: string; kind: 'company' | 'person'; status: KycCase['status']; now: number; company?: { cuit: string; legalName: string; legalAddress: string } }): KycCase {
  const approved = params.status === 'approved';
  return {
    id: `kyc_${params.orgId}`,
    orgId: params.orgId,
    kind: params.kind,
    status: params.status,
    cuit: params.company?.cuit ?? null,
    legalName: params.company?.legalName ?? null,
    legalAddress: params.company?.legalAddress ?? null,
    fullName: params.kind === 'person' ? 'Erick Jiménez' : null,
    dni: null,
    representatives: approved ? [{ id: `rep_${params.orgId}`, fullName: 'Erick Jiménez', dni: '30111222' }] : [],
    documents: approved
      ? [
          { id: `doc_${params.orgId}_1`, type: 'bylaws', fileName: 'estatuto-social.pdf', representativeId: null, uploadedAt: at(params.now, 40, 10) },
          { id: `doc_${params.orgId}_2`, type: 'afip_constancy', fileName: 'constancia-afip.pdf', representativeId: null, uploadedAt: at(params.now, 40, 10, 5) },
          { id: `doc_${params.orgId}_3`, type: 'dni_front', fileName: 'dni-frente.jpg', representativeId: `rep_${params.orgId}`, uploadedAt: at(params.now, 40, 10, 8) },
          { id: `doc_${params.orgId}_4`, type: 'selfie', fileName: 'selfie.jpg', representativeId: `rep_${params.orgId}`, uploadedAt: at(params.now, 40, 10, 9) },
        ]
      : [],
    timeline: approved
      ? [
          { status: 'draft', at: at(params.now, 41, 9) },
          { status: 'submitted', at: at(params.now, 40, 10, 15) },
          { status: 'approved', at: at(params.now, 39, 16) },
        ]
      : [{ status: 'draft', at: at(params.now, 1, 9) }],
    rejectionReason: null,
  };
}

function reconItems(prefix: string, payins: MockPayin[], payouts: MockPayout[]): ReconciliationItem[] {
  return [
    ...payins.map((item) => ({ id: `${prefix}_${item.id}`, status: 'matched' as const, providerTransferId: `BIND-${item.id.toUpperCase()}`, amount: item.amount, description: item.payerName })),
    ...payouts.map((item) => ({ id: `${prefix}_${item.id}`, status: 'matched' as const, providerTransferId: `BIND-${item.id.toUpperCase()}`, amount: -(item.amount + item.fee), description: item.destination.holderName })),
  ];
}

/** Seven days × two currencies; ARS two days ago and USD five days ago carry a difference. */
function seedRecon(now: number, payins: MockPayin[], payouts: MockPayout[]): ReconciliationRun[] {
  const runs: ReconciliationRun[] = [];
  for (let daysAgo = 1; daysAgo <= 7; daysAgo += 1) {
    const day = at(now, daysAgo, 12).slice(0, 10);
    for (const currency of ['ARS', 'USD'] as const) {
      const sameDay = (iso: string) => iso.slice(0, 10) === day;
      const dayPayins = payins.filter((item) => item.orgId === DEMO_ORG_ID && item.currency === currency && sameDay(item.createdAt));
      const dayPayouts = payouts.filter((item) => item.orgId === DEMO_ORG_ID && item.currency === currency && item.status === 'settled' && sameDay(item.createdAt));
      const id = `rec_${currency}_${day}`;
      const items = reconItems(id, dayPayins, dayPayouts);
      if (currency === 'ARS' && daysAgo === 2) {
        items.push({ id: `${id}_x1`, status: 'missing_in_ledger', providerTransferId: 'BIND-77810342', amount: 85_000_00, description: 'Transferencia entrante sin CVU asociado' });
      }
      if (currency === 'USD' && daysAgo === 5) {
        items.push({ id: `${id}_x2`, status: 'missing_at_psp', providerTransferId: 'BIND-77790015', amount: -200_00, description: 'Pago informado y no liquidado por el banco' });
      }
      const sum = (filter: (item: ReconciliationItem) => boolean) => items.filter(filter).reduce((total, item) => total + item.amount, 0);
      runs.push({
        id,
        currency,
        day,
        matched: items.filter((item) => item.status === 'matched').length,
        missingInLedger: items.filter((item) => item.status === 'missing_in_ledger').length,
        missingAtPsp: items.filter((item) => item.status === 'missing_at_psp').length,
        pspTotal: sum((item) => item.status !== 'missing_at_psp'),
        ledgerTotal: sum((item) => item.status !== 'missing_in_ledger'),
        items,
        createdAt: at(now, daysAgo - 1, 3),
      });
    }
  }
  return runs;
}

export function createSeed(now: number): MockState {
  const accounts = seedAccounts(now);
  const payers = seedPayers(now);
  const payins = seedPayins(now, payers, accounts);
  const payouts = seedPayouts(now, accounts);
  applySeedEffects(accounts, payouts);
  return {
    seq: 1000,
    currentUserId: null,
    pendingSignup: null,
    users: [
      { id: 'u_erick', email: 'erick@desarrollosdelsur.com.ar', fullName: 'Erick Jiménez', mfaEnabled: false, personalModeEnabled: false },
      { id: 'u_martina', email: 'martina@desarrollosdelsur.com.ar', fullName: 'Martina López', mfaEnabled: true, personalModeEnabled: false },
      { id: 'u_pablo', email: 'pablo@desarrollosdelsur.com.ar', fullName: 'Pablo Sánchez', mfaEnabled: true, personalModeEnabled: false },
      { id: 'u_sofia', email: 'sofia@desarrollosdelsur.com.ar', fullName: 'Sofía Díaz', mfaEnabled: false, personalModeEnabled: false },
      { id: 'u_diego', email: 'diego@constructoraandina.com.ar', fullName: 'Diego Ferrari', mfaEnabled: false, personalModeEnabled: false },
    ],
    orgs: [
      { id: DEMO_ORG_ID, kind: 'company', legalName: 'Desarrollos Del Sur SA', cuit: '30712345671', kycStatus: 'approved', erpLinked: false },
      { id: ANDINA_ORG_ID, kind: 'company', legalName: 'Constructora Andina SRL', cuit: '30713456787', kycStatus: 'approved', erpLinked: false },
      { id: PERSONAL_ORG_ID, kind: 'personal', legalName: 'Erick Jiménez', cuit: null, kycStatus: 'draft', erpLinked: false },
    ],
    memberships: [
      { orgId: DEMO_ORG_ID, userId: 'u_erick', role: 'admin' },
      { orgId: DEMO_ORG_ID, userId: 'u_martina', role: 'operator' },
      { orgId: DEMO_ORG_ID, userId: 'u_pablo', role: 'approver' },
      { orgId: DEMO_ORG_ID, userId: 'u_sofia', role: 'viewer' },
      { orgId: ANDINA_ORG_ID, userId: 'u_erick', role: 'approver' },
      { orgId: ANDINA_ORG_ID, userId: 'u_diego', role: 'admin' },
      { orgId: PERSONAL_ORG_ID, userId: 'u_erick', role: 'admin' },
    ],
    invites: [{ id: 'inv_1', orgId: DEMO_ORG_ID, email: 'contador@estudiogarcia.com.ar', role: 'viewer', expiresAt: new Date(now + 5 * DAY).toISOString() }],
    kyc: {
      [DEMO_ORG_ID]: kycCase({ orgId: DEMO_ORG_ID, kind: 'company', status: 'approved', now, company: { cuit: '30712345671', legalName: 'Desarrollos Del Sur SA', legalAddress: 'Av. del Libertador 5800, piso 9, CABA' } }),
      [ANDINA_ORG_ID]: kycCase({ orgId: ANDINA_ORG_ID, kind: 'company', status: 'approved', now, company: { cuit: '30713456787', legalName: 'Constructora Andina SRL', legalAddress: 'Av. Cabildo 2150, CABA' } }),
      [PERSONAL_ORG_ID]: kycCase({ orgId: PERSONAL_ORG_ID, kind: 'person', status: 'draft', now }),
    },
    kycSubmittedAt: {},
    accounts,
    payers,
    payins,
    payouts,
    transfers: seedTransfers(now),
    recon: { [DEMO_ORG_ID]: seedRecon(now, payins, payouts), [ANDINA_ORG_ID]: [] },
    erp: {
      [DEMO_ORG_ID]: {
        candidate: { found: true, erpOrgName: 'Desarrollos Del Sur S.A.', cuit: '30712345671', projectsCount: 3, buyersCount: 4 },
        projects: ['Torre Palermo', 'Barrio Nordelta Lote 12', 'Complejo Pilar Norte'],
        payers: [
          { name: 'Federico Acosta', docType: 'CUIT', docNumber: '20284567898', project: 'Complejo Pilar Norte' },
          { name: 'Valeria Méndez', docType: 'DNI', docNumber: '33456789', project: 'Complejo Pilar Norte' },
          { name: 'Grupo Inversor Norte SA', docType: 'CUIT', docNumber: '30705556667', project: 'Complejo Pilar Norte' },
          { name: 'Martín Romero', docType: 'DNI', docNumber: '35111222', project: 'Torre Palermo' },
        ],
        linkStartedAt: null,
        linkedAt: null,
      },
      [ANDINA_ORG_ID]: { candidate: { found: false, erpOrgName: null, cuit: null, projectsCount: 0, buyersCount: 0 }, projects: [], payers: [], linkStartedAt: null, linkedAt: null },
    },
    idempotency: {},
  };
}
