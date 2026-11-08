import { buildCbu, buildCvu } from '@/shared/helpers/cbu';

import type { AccountCvu, Currency, Cvu, Destination, DocType, Installment, KycCase, KycDocument, PayoutStatus, Rail, ReconciliationItem } from '../types';
import { payinFee, payoutFee } from './feePlan';
import type { MockAccount, MockPayer, MockPayin, MockPayout, MockReconRun, MockState, MockTransfer } from './state';

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
/** The DNI Erick's personal KYC verified: he is a payer of both demo companies ("Mis pagos"). */
const PERSONAL_DNI = '30111222';

const users = {
  erick: { userId: 'u_erick', label: 'Erick Jiménez' },
  martina: { userId: 'u_martina', label: 'Martina López' },
  pablo: { userId: 'u_pablo', label: 'Pablo Sánchez' },
  diego: { userId: 'u_diego', label: 'Diego Ferrari' },
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
  return { type: 'alias', value: alias, accountNumber: buildCbu(entityBranch, account), holderName, holderCuit, bankName };
}

/** A CVU as the contract returns it; the share link uses a reserved `.test` host (fake data). */
export function buildMockCvu(params: { id: string; payerId: string; accountId: string; currency: Currency; cvu: string; alias: string; createdAt: string }): Cvu {
  const link = `https://pay.lenapay.test/${params.cvu}`;
  return {
    id: params.id,
    payerId: params.payerId,
    accountId: params.accountId,
    currency: params.currency,
    cvu: params.cvu,
    alias: params.alias,
    status: 'active',
    share: { link, qrPayload: `${link}?currency=${params.currency}` },
    createdAt: params.createdAt,
  };
}

/** A personal account's own CVU (R-CVU-4), shaped like the contract's AccountCvu. */
export function buildAccountCvu(params: { id: string; currency: Currency; cvu: string; alias: string }): AccountCvu {
  const link = `https://pay.lenapay.test/${params.cvu}`;
  return { id: params.id, currency: params.currency, cvu: params.cvu, alias: params.alias, status: 'active', share: { link, qrPayload: `${link}?currency=${params.currency}` } };
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
  docType: DocType;
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
    cvus: params.cvus.map((cvu, index) =>
      buildMockCvu({ id: `cvu_${params.id}_${index}`, payerId: params.id, accountId: params.accountId, currency: cvu.currency, cvu: buildCvu(PSP_PREFIX, cvu.account), alias: cvu.alias, createdAt: params.createdAt }),
    ),
  };
}

function account(params: Omit<MockAccount, 'cvus'> & { cvus?: AccountCvu[] }): MockAccount {
  return { ...params, cvus: params.cvus ?? [] };
}

function seedAccounts(now: number): MockAccount[] {
  return [
    account({ id: 'acc_main', orgId: DEMO_ORG_ID, kind: 'main', name: 'Cuenta principal', archived: false, createdAt: at(now, 200, 10), ledger: ledger(48_250_000_00, 85_000_00) }),
    account({ id: 'acc_torre', orgId: DEMO_ORG_ID, kind: 'project', name: 'Torre Palermo', archived: false, createdAt: at(now, 200, 10), ledger: ledger(125_430_500_00, 312_000_00) }),
    account({ id: 'acc_nordelta', orgId: DEMO_ORG_ID, kind: 'project', name: 'Barrio Nordelta Lote 12', archived: false, createdAt: at(now, 200, 10), ledger: ledger(36_780_000_00, 54_500_00) }),
    account({ id: 'acc_caballito', orgId: DEMO_ORG_ID, kind: 'project', name: 'Edificio Caballito', archived: true, createdAt: at(now, 200, 10), ledger: ledger(0, 0) }),
    account({ id: 'acc_andina_main', orgId: ANDINA_ORG_ID, kind: 'main', name: 'Cuenta principal', archived: false, createdAt: at(now, 200, 10), ledger: ledger(12_400_000_00, 9_800_00) }),
    account({ id: 'acc_belgrano', orgId: ANDINA_ORG_ID, kind: 'project', name: 'Edificio Belgrano R', archived: false, createdAt: at(now, 200, 10), ledger: ledger(22_150_000_00, 0) }),
    account({
      id: 'acc_personal', orgId: PERSONAL_ORG_ID, kind: 'personal', name: 'Mi cuenta', archived: false, createdAt: at(now, 200, 10), ledger: ledger(2_985_000_00, 1_250_00),
      cvus: [
        buildAccountCvu({ id: 'acvu_personal_ars', currency: 'ARS', cvu: buildCvu(PSP_PREFIX, '1000000000901'), alias: 'erick.jimenez.lnp' }),
        buildAccountCvu({ id: 'acvu_personal_usd', currency: 'USD', cvu: buildCvu(PSP_PREFIX, '1000000000902'), alias: 'erick.jimenez.usd' }),
      ],
    }),
  ];
}

function seedPayers(now: number): MockPayer[] {
  const created = at(now, 45, 11);
  return [
    payer({ id: 'pay_juan', orgId: DEMO_ORG_ID, accountId: 'acc_torre', name: 'Juan Pérez', docType: 'dni', docNumber: '28456789', email: 'juan.perez@gmail.com', createdAt: created,
      cvus: [{ currency: 'ARS', alias: 'juan.perez.torre', account: '1000000000101' }, { currency: 'USD', alias: 'juan.perez.torre.usd', account: '1000000000102' }] }),
    payer({ id: 'pay_maria', orgId: DEMO_ORG_ID, accountId: 'acc_torre', name: 'María González', docType: 'cuit', docNumber: '27334567899', email: 'maria.gonzalez@hotmail.com', createdAt: created,
      cvus: [{ currency: 'ARS', alias: 'maria.gonzalez.tp', account: '1000000000201' }] }),
    payer({ id: 'pay_lago', orgId: DEMO_ORG_ID, accountId: 'acc_torre', name: 'Inversiones Lago SRL', docType: 'cuit', docNumber: '30709876542', email: 'admin@inversioneslago.com.ar', createdAt: created,
      cvus: [{ currency: 'ARS', alias: 'inv.lago.torre', account: '1000000000301' }, { currency: 'USD', alias: 'inv.lago.torre.usd', account: '1000000000302' }] }),
    payer({ id: 'pay_carlos', orgId: DEMO_ORG_ID, accountId: 'acc_nordelta', name: 'Carlos Rodríguez', docType: 'cuit', docNumber: '20319876546', email: 'crodriguez@outlook.com', createdAt: created,
      cvus: [{ currency: 'ARS', alias: 'carlos.rod.nordelta', account: '1000000000401' }] }),
    payer({ id: 'pay_lucia', orgId: DEMO_ORG_ID, accountId: 'acc_nordelta', name: 'Lucía Fernández', docType: 'cuit', docNumber: '27351112226', email: null, createdAt: created,
      cvus: [{ currency: 'ARS', alias: 'lucia.fer.nordelta', account: '1000000000501' }, { currency: 'USD', alias: 'lucia.fer.ndt.usd', account: '1000000000502' }] }),
    payer({ id: 'pay_erick_dds', orgId: DEMO_ORG_ID, accountId: 'acc_torre', name: 'Erick Jiménez', docType: 'dni', docNumber: PERSONAL_DNI, email: null, createdAt: created,
      cvus: [{ currency: 'ARS', alias: 'erick.jim.torre', account: '1000000000701' }] }),
    payer({ id: 'pay_erick_andina', orgId: ANDINA_ORG_ID, accountId: 'acc_belgrano', name: 'Erick Jiménez', docType: 'cuit', docNumber: `20${PERSONAL_DNI}3`, email: null, createdAt: created,
      cvus: [{ currency: 'ARS', alias: 'erick.jim.belgrano', account: '1000000000801' }, { currency: 'USD', alias: 'erick.jim.belg.usd', account: '1000000000802' }] }),
    payer({ id: 'pay_roberto', orgId: ANDINA_ORG_ID, accountId: 'acc_belgrano', name: 'Roberto Silva', docType: 'cuit', docNumber: '20389991113', email: 'rsilva@gmail.com', createdAt: created,
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
  { payerId: 'pay_erick_andina', currency: 'ARS', major: 862_500, daysAgo: 58, hour: 10 },
  { payerId: 'pay_erick_andina', currency: 'ARS', major: 875_000, daysAgo: 27, hour: 11 },
  { payerId: 'pay_erick_dds', currency: 'ARS', major: 450_000, daysAgo: 9, hour: 17 },
];

function originCuitOf(payerRecord: Pick<MockPayer, 'docType' | 'docNumber'>): string {
  return payerRecord.docType === 'dni' ? `20${payerRecord.docNumber}8` : payerRecord.docNumber;
}

export function buildPayin(params: { id: string; payer: MockPayer; accountName: string; currency: Currency; amount: number; createdAt: string; rail?: Rail }): MockPayin {
  const cvu = params.payer.cvus.find((item) => item.currency === params.currency) ?? params.payer.cvus[0];
  const fee = payinFee(params.amount, params.currency);
  return {
    id: params.id,
    orgId: params.payer.orgId,
    cvuId: cvu?.id ?? '',
    payerId: params.payer.id,
    accountId: params.payer.accountId,
    accountName: params.accountName,
    currency: params.currency,
    amount: params.amount,
    fee,
    net: params.amount - fee,
    originName: params.payer.name,
    originCuit: originCuitOf(params.payer),
    createdAt: params.createdAt,
    rail: params.rail ?? 'bank',
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
    const fee = payoutFee(amount, plan.currency);
    const destination = DIRECTORY[plan.supplier];
    if (!destination) throw new Error(`fixture supplier ${plan.supplier}`);
    return {
      id: plan.id,
      orgId: plan.orgId,
      accountId: plan.accountId,
      accountName: accounts.find((item) => item.id === plan.accountId)?.name ?? '',
      currency: plan.currency,
      amount,
      fee,
      total: amount + fee,
      destination,
      concept: plan.concept,
      attachments: plan.id === 'po_5' ? [invoice(plan.id, users[plan.createdBy].userId, new Date(created).toISOString())] : [],
      status: plan.status,
      rail: 'bank',
      createdBy: users[plan.createdBy],
      approvedBy: plan.decidedBy && plan.status !== 'rejected' ? users[plan.decidedBy] : null,
      rejectReason: plan.rejectionReason ?? null,
      failureCode: plan.status === 'failed' ? 'provider.rejected' : null,
      createdAt: new Date(created).toISOString(),
      settledAt: plan.status === 'settled' ? new Date(created + 0.3 * HOUR).toISOString() : null,
      events: STATUS_PATH[plan.status].map((status, index) => ({ status, at: new Date(created + index * 0.1 * HOUR).toISOString() })),
    };
  });
}

function invoice(payoutId: string, uploadedBy: string, createdAt: string) {
  return { id: `att_${payoutId}`, fileName: 'factura-electro-norte-0012.pdf', contentType: 'application/pdf' as const, sizeBytes: 48_213, uploadedBy, createdAt };
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

const ADDRESS = { street: 'Av. del Libertador', number: '5800', floor: '9', city: 'CABA', province: 'CABA', postalCode: 'C1428', country: 'AR' };

function kycDocuments(orgId: string, now: number, representativeDni: string): KycDocument[] {
  const document = (index: number, kind: KycDocument['kind'], fileName: string, dni: string | null): KycDocument => ({
    id: `doc_${orgId}_${index}`,
    kind,
    representativeDni: dni,
    fileName,
    contentType: fileName.endsWith('.pdf') ? 'application/pdf' : 'image/jpeg',
    sizeBytes: 120_000,
    createdAt: at(now, 40, 10, index),
  });
  return [
    document(1, 'bylaws', 'estatuto-social.pdf', null),
    document(2, 'afip_constancy', 'constancia-afip.pdf', null),
    document(3, 'dni_front', 'dni-frente.jpg', representativeDni),
    document(4, 'dni_back', 'dni-dorso.jpg', representativeDni),
    document(5, 'selfie', 'selfie.jpg', representativeDni),
  ];
}

function kycCase(params: { orgId: string; kind: 'company' | 'person'; status: KycCase['status']; now: number; company?: { cuit: string; legalName: string } }): KycCase {
  const approved = params.status === 'approved';
  const representative = { fullName: 'Erick Jiménez', dni: '30111222', position: 'Presidente' };
  return {
    id: `kyc_${params.orgId}`,
    orgId: params.orgId,
    kind: params.kind,
    status: params.status,
    ...(params.company ? { company: { ...params.company, legalAddress: ADDRESS, representatives: approved ? [representative] : [] } } : {}),
    documents: approved ? kycDocuments(params.orgId, params.now, representative.dni) : [],
    decisionReason: null,
    submittedAt: approved ? at(params.now, 40, 10, 15) : null,
    decidedAt: approved ? at(params.now, 39, 16) : null,
    updatedAt: approved ? at(params.now, 39, 16) : at(params.now, 1, 9),
  };
}

/** Erick's personal KYC, approved with his DNI and the three identity documents. */
function personalKyc(now: number): KycCase {
  const document = (index: number, kind: KycDocument['kind'], fileName: string): KycDocument => ({
    id: `doc_${PERSONAL_ORG_ID}_${index}`, kind, representativeDni: null, fileName, contentType: 'image/jpeg', sizeBytes: 120_000, createdAt: at(now, 59, 10, index),
  });
  return {
    id: `kyc_${PERSONAL_ORG_ID}`,
    orgId: PERSONAL_ORG_ID,
    kind: 'person',
    status: 'approved',
    person: { fullName: 'Erick Jiménez', dni: PERSONAL_DNI, birthDate: '1990-05-14', address: ADDRESS },
    documents: [document(1, 'dni_front', 'dni-frente.jpg'), document(2, 'dni_back', 'dni-dorso.jpg'), document(3, 'selfie', 'selfie.jpg')],
    decisionReason: null,
    submittedAt: at(now, 59, 10, 15),
    decidedAt: at(now, 59, 16),
    updatedAt: at(now, 59, 16),
  };
}

/** The 10th of each month: date-only (YYYY-MM-DD), months relative to `now`. */
function dueDate(now: number, monthOffset: number): string {
  const date = new Date(now);
  return new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth() + monthOffset, 10)).toISOString().slice(0, 10);
}

/** 12 CAC-adjusted installments: 3 paid, the 4th overdue, the rest pending (like the ERP fixture). */
function seedInstallments(now: number): Installment[] {
  return Array.from({ length: 12 }, (_, index) => {
    const number = index + 1;
    const status: Installment['status'] = number <= 3 ? 'paid' : number === 4 ? 'overdue' : 'pending';
    return { number, dueDate: dueDate(now, number - 5), currency: 'ARS', amount: (850_000_00 + index * 12_500_00), status, adjustmentIndex: 'CAC' };
  });
}

function reconItems(prefix: string, payins: MockPayin[], payouts: MockPayout[]): ReconciliationItem[] {
  return [
    ...payins.map((item) => ({ id: `${prefix}_${item.id}`, status: 'matched' as const, providerTransferId: `BIND-${item.id.toUpperCase()}`, direction: 'in' as const, pspAmount: item.amount, ledgerAmount: item.amount, createdAt: item.createdAt })),
    ...payouts.map((item) => ({ id: `${prefix}_${item.id}`, status: 'matched' as const, providerTransferId: `BIND-${item.id.toUpperCase()}`, direction: 'out' as const, pspAmount: item.total, ledgerAmount: item.total, createdAt: item.createdAt })),
  ];
}

/** Signed sum of one side (credits minus debits); a null amount is absent on that side. */
function sideTotal(items: ReconciliationItem[], side: 'pspAmount' | 'ledgerAmount'): number {
  return items.reduce((total, item) => total + (item[side] ?? 0) * (item.direction === 'in' ? 1 : -1), 0);
}

export function buildReconRun(params: { id: string; currency: Currency; day: string; items: ReconciliationItem[]; createdAt: string }): MockReconRun {
  const { items } = params;
  const pspTotal = sideTotal(items, 'pspAmount');
  const ledgerTotal = sideTotal(items, 'ledgerAmount');
  return {
    run: {
      id: params.id,
      currency: params.currency,
      day: params.day,
      status: 'completed',
      matched: items.filter((item) => item.status === 'matched').length,
      missingInLedger: items.filter((item) => item.status === 'missing_in_ledger').length,
      missingAtPsp: items.filter((item) => item.status === 'missing_at_psp').length,
      pspTotal,
      ledgerTotal,
      difference: pspTotal - ledgerTotal,
      createdAt: params.createdAt,
      finishedAt: params.createdAt,
    },
    items,
  };
}

/** Seven days x two currencies; ARS two days ago and USD five days ago carry a difference. */
function seedRecon(now: number, payins: MockPayin[], payouts: MockPayout[]): MockReconRun[] {
  const runs: MockReconRun[] = [];
  for (let daysAgo = 1; daysAgo <= 7; daysAgo += 1) {
    const day = at(now, daysAgo, 12).slice(0, 10);
    for (const currency of ['ARS', 'USD'] as const) {
      const sameDay = (iso: string) => iso.slice(0, 10) === day;
      const dayPayins = payins.filter((item) => item.orgId === DEMO_ORG_ID && item.currency === currency && sameDay(item.createdAt));
      const dayPayouts = payouts.filter((item) => item.orgId === DEMO_ORG_ID && item.currency === currency && item.status === 'settled' && sameDay(item.createdAt));
      const id = `rec_${currency}_${day}`;
      const createdAt = at(now, daysAgo - 1, 3);
      const items = reconItems(id, dayPayins, dayPayouts);
      if (currency === 'ARS' && daysAgo === 2) {
        items.push({ id: `${id}_x1`, status: 'missing_in_ledger', providerTransferId: 'BIND-77810342', direction: 'in', pspAmount: 85_000_00, ledgerAmount: null, createdAt });
      }
      if (currency === 'USD' && daysAgo === 5) {
        items.push({ id: `${id}_x2`, status: 'missing_at_psp', providerTransferId: 'BIND-77790015', direction: 'out', pspAmount: null, ledgerAmount: 200_00, createdAt });
      }
      runs.push(buildReconRun({ id, currency, day, items, createdAt }));
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
    devices: [],
    users: [
      { id: 'u_erick', email: 'erick@desarrollosdelsur.com.ar', fullName: 'Erick Jiménez', mfaEnabled: false },
      { id: 'u_martina', email: 'martina@desarrollosdelsur.com.ar', fullName: 'Martina López', mfaEnabled: true },
      { id: 'u_pablo', email: 'pablo@desarrollosdelsur.com.ar', fullName: 'Pablo Sánchez', mfaEnabled: true },
      { id: 'u_sofia', email: 'sofia@desarrollosdelsur.com.ar', fullName: 'Sofía Díaz', mfaEnabled: false },
      { id: 'u_diego', email: 'diego@constructoraandina.com.ar', fullName: 'Diego Ferrari', mfaEnabled: false },
    ],
    orgs: [
      { id: DEMO_ORG_ID, kind: 'company', legalName: 'Desarrollos Del Sur SA', cuit: '30712345671', kycStatus: 'approved', createdAt: at(now, 41, 9) },
      { id: ANDINA_ORG_ID, kind: 'company', legalName: 'Constructora Andina SRL', cuit: '30713456787', kycStatus: 'approved', createdAt: at(now, 41, 9) },
      { id: PERSONAL_ORG_ID, kind: 'personal', legalName: 'Erick Jiménez', cuit: null, kycStatus: 'approved', createdAt: at(now, 60, 9) },
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
    invites: [{ id: 'inv_1', orgId: DEMO_ORG_ID, email: 'contador@estudiogarcia.com.ar', role: 'viewer', expiresAt: new Date(now + 5 * DAY).toISOString(), createdAt: at(now, 2, 10) }],
    kyc: {
      [DEMO_ORG_ID]: kycCase({ orgId: DEMO_ORG_ID, kind: 'company', status: 'approved', now, company: { cuit: '30712345671', legalName: 'Desarrollos Del Sur SA' } }),
      [ANDINA_ORG_ID]: kycCase({ orgId: ANDINA_ORG_ID, kind: 'company', status: 'approved', now, company: { cuit: '30713456787', legalName: 'Constructora Andina SRL' } }),
      [PERSONAL_ORG_ID]: personalKyc(now),
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
        candidate: { found: true, erpOrgId: 'erp-org-001', displayName: 'Desarrollos Del Sur S.A.' },
        projects: ['Torre Palermo', 'Barrio Nordelta Lote 12', 'Complejo Pilar Norte'],
        payers: [
          { name: 'Federico Acosta', docType: 'cuit', docNumber: '20284567898', project: 'Complejo Pilar Norte' },
          { name: 'Valeria Méndez', docType: 'dni', docNumber: '33456789', project: 'Complejo Pilar Norte' },
          { name: 'Grupo Inversor Norte SA', docType: 'cuit', docNumber: '30705556667', project: 'Complejo Pilar Norte' },
          { name: 'Martín Romero', docType: 'dni', docNumber: '35111222', project: 'Torre Palermo' },
        ],
        linkStartedAt: null,
        linkedAt: null,
      },
      // Already linked to its ERP: the source of Erick's installments in "Mis pagos".
      [ANDINA_ORG_ID]: {
        candidate: { found: true, erpOrgId: 'erp-org-002', displayName: 'Constructora Andina S.R.L.' },
        projects: [],
        payers: [],
        linkStartedAt: now - 90 * DAY,
        linkedAt: at(now, 90, 10),
      },
    },
    verifiedDni: { [PERSONAL_ORG_ID]: PERSONAL_DNI },
    payerLinks: [],
    installments: { pay_erick_andina: seedInstallments(now) },
    idempotency: {},
  };
}
