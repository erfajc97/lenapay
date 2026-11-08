import type { components } from '@lenapay/api-types';

import { accountDisplayName } from '@/shared/helpers/accounts';
import { i18n } from '@/shared/i18n';

import type {
  Account,
  Balance,
  ErpLink,
  Installment,
  InstallmentList,
  InternalTransfer,
  LiveEvent,
  Me,
  Member,
  Movement,
  MyPaymentCompany,
  MyPaymentPayin,
  Payer,
  Payin,
  OwnershipTransfer,
  Payout,
  PayoutQuote,
  ReconciliationItem,
  ReconciliationRun,
} from '../types';

/**
 * Contract DTO → view model, one pure function per shape (the live client's only translation
 * layer). Money `{minor, currency}` becomes minor units next to a currency; names the
 * contract does not carry are joined from the org directory (accounts and members).
 */
type Schemas = components['schemas'];

export interface OrgDirectory {
  accountName(accountId: string): string;
  /** Name (else email) of a member, or null when the user is not (or no longer) a member. */
  memberLabel(userId: string): string | null;
}

export function toBalance(dto: Schemas['CurrencyBalance']): Balance {
  return { currency: dto.currency, posted: dto.posted.minor, held: dto.held.minor, available: dto.available.minor };
}

export function toAccount(dto: Schemas['Account']): Account {
  return {
    id: dto.id,
    kind: dto.kind,
    name: accountDisplayName(dto, (key) => i18n.t(key)),
    archived: dto.archived,
    erpProjectRef: dto.erpProjectRef,
    balances: dto.balances.map(toBalance),
    payersCount: null,
    cvus: dto.cvus,
    createdAt: dto.createdAt,
  };
}

const NO_PREFERENCES: Schemas['UserPreferences'] = { defaultOrgId: null, defaultAccountId: null, locale: null, theme: null, updatedAt: null };

export function toMe(dto: Schemas['Me']): Me {
  return {
    id: dto.user.id,
    email: dto.user.email,
    fullName: dto.user.fullName ?? null,
    mfaEnabled: dto.user.mfaEnabled,
    personalModeEnabled: dto.memberships.some((membership) => membership.orgKind === 'personal'),
    memberships: dto.memberships,
    // Always present per the contract; an older API without it reads as "nothing set".
    preferences: dto.preferences ?? NO_PREFERENCES,
  };
}

export function toMember(dto: Schemas['Member']): Member {
  return { userId: dto.userId, email: dto.email, fullName: dto.fullName ?? null, role: dto.role, status: 'active' };
}

export function toOwnershipTransfer(dto: Schemas['OwnershipTransfer']): OwnershipTransfer {
  return { previousOwner: toMember(dto.previousOwner), owner: toMember(dto.owner) };
}

export function toPayer(dto: Schemas['Payer'], directory: OrgDirectory): Payer {
  return { ...dto, accountName: directory.accountName(dto.accountId) };
}

export function toPayin(dto: Schemas['Payin'], directory: OrgDirectory): Payin {
  return {
    id: dto.id,
    accountId: dto.accountId,
    accountName: directory.accountName(dto.accountId),
    payerId: dto.payerId,
    cvuId: dto.cvuId,
    currency: dto.gross.currency,
    amount: dto.gross.minor,
    fee: dto.fee.minor,
    net: dto.net.minor,
    originName: dto.origin.name,
    originCuit: dto.origin.cuit,
    createdAt: dto.createdAt,
  };
}

export function toQuote(dto: Schemas['PayoutQuote']): PayoutQuote {
  return { currency: dto.amount.currency, amount: dto.amount.minor, fee: dto.fee.minor, total: dto.total.minor, availableAfter: dto.availableAfter.minor, rail: dto.rail };
}

function person(userId: string, directory: OrgDirectory) {
  return { userId, label: directory.memberLabel(userId) };
}

export function toPayout(dto: Schemas['Payout'], directory: OrgDirectory): Payout {
  return {
    id: dto.id,
    accountId: dto.accountId,
    accountName: directory.accountName(dto.accountId),
    currency: dto.amount.currency,
    amount: dto.amount.minor,
    fee: dto.fee.minor,
    total: dto.total.minor,
    destination: dto.destination,
    concept: dto.concept,
    status: dto.status,
    rail: dto.rail,
    createdBy: person(dto.createdBy, directory),
    approvedBy: dto.approvedBy ? person(dto.approvedBy, directory) : null,
    rejectReason: dto.rejectReason,
    failureCode: dto.failureCode,
    attachments: dto.attachments,
    events: dto.events.map((event) => ({ status: event.toStatus, at: event.createdAt })),
    createdAt: dto.createdAt,
    settledAt: dto.settledAt,
  };
}

export function toInternalTransfer(dto: Schemas['InternalTransfer']): InternalTransfer {
  return {
    id: dto.id,
    fromAccountId: dto.fromAccountId,
    toAccountId: dto.toAccountId,
    currency: dto.amount.currency,
    amount: dto.amount.minor,
    concept: dto.concept,
    createdAt: dto.createdAt,
  };
}

export function toMovement(dto: Schemas['Movement'], directory: OrgDirectory): Movement {
  return {
    id: dto.id,
    type: dto.type,
    direction: dto.direction,
    status: dto.status,
    currency: dto.amount.currency,
    amount: dto.amount.minor,
    fee: dto.fee ? dto.fee.minor : null,
    accountId: dto.accountId,
    accountName: directory.accountName(dto.accountId),
    counterparty: dto.counterparty.name,
    counterpartyDoc: dto.counterparty.cuit,
    concept: dto.concept,
    refId: dto.refId,
    rail: dto.rail,
    createdAt: dto.createdAt,
  };
}

export function toMyPaymentCompany(dto: Schemas['MyPaymentCompany']): MyPaymentCompany {
  return {
    linkId: dto.linkId,
    companyName: dto.companyName,
    projectName: dto.projectName,
    cvus: dto.cvus,
    totalPaid: dto.totalPaid.map((total) => ({ currency: total.currency, amount: total.minor })),
    lastPaymentAt: dto.lastPaymentAt,
    erpLinked: dto.erpLinked,
    hidden: dto.hidden,
  };
}

export function toMyPaymentPayin(dto: Schemas['MyPaymentPayin']): MyPaymentPayin {
  return { id: dto.id, cvu: dto.cvu, currency: dto.amount.currency, amount: dto.amount.minor, status: dto.status, createdAt: dto.createdAt, creditedAt: dto.creditedAt };
}

function toInstallment(dto: Schemas['MyPaymentInstallment']): Installment {
  return { number: dto.number, dueDate: dto.dueDate, currency: dto.amount.currency, amount: dto.amount.minor, status: dto.status, adjustmentIndex: dto.adjustmentIndex };
}

export function toInstallmentList(dto: Schemas['MyPaymentInstallments']): InstallmentList {
  return { erpLinked: dto.erpLinked, items: dto.items.map(toInstallment) };
}

export function toReconciliationRun(dto: Schemas['ReconciliationRun']): ReconciliationRun {
  return {
    id: dto.id,
    currency: dto.currency,
    day: dto.day,
    status: dto.status,
    matched: dto.matchedCount,
    missingInLedger: dto.missingInLedgerCount,
    missingAtPsp: dto.missingAtPspCount,
    pspTotal: dto.pspTotal.minor,
    ledgerTotal: dto.ledgerTotal.minor,
    difference: dto.difference.minor,
    createdAt: dto.createdAt,
    finishedAt: dto.finishedAt,
  };
}

export function toReconciliationItem(dto: Schemas['ReconciliationItem']): ReconciliationItem {
  return {
    id: dto.id,
    status: dto.status,
    providerTransferId: dto.providerTransferId,
    direction: dto.direction,
    pspAmount: dto.pspAmount ? dto.pspAmount.minor : null,
    ledgerAmount: dto.ledgerAmount ? dto.ledgerAmount.minor : null,
    createdAt: dto.createdAt,
  };
}

const IMPORT_TO_LINK: Record<Schemas['ErpImportStatus'], ErpLink['status']> = {
  pending: 'importing',
  running: 'importing',
  completed: 'linked',
  failed: 'failed',
};

/** `null` is the contract's 404 `erp_link.not_found`: the org is not linked yet. */
export function toErpLink(dto: Schemas['ErpLink'] | null): ErpLink {
  if (!dto) return { status: 'not_linked', displayName: null, linkedAt: null, importedAt: null, importSummary: null };
  return {
    status: IMPORT_TO_LINK[dto.importStatus],
    displayName: dto.displayName,
    linkedAt: dto.linkedAt,
    importedAt: dto.importedAt,
    importSummary: dto.importSummary ?? null,
  };
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

/**
 * One SSE `data:` line → a live event, or null for anything the app does not handle
 * (unknown type, malformed JSON, missing payload). The envelope is `{type, orgId,
 * occurredAt, data}`; the payload is read from `data` (also copied to the top level).
 */
export function parseRealtimeFrame(raw: string | null | undefined, directory: OrgDirectory): LiveEvent | null {
  if (!raw) return null;
  let frame: unknown;
  try {
    frame = JSON.parse(raw);
  } catch {
    return null;
  }
  if (!isRecord(frame)) return null;
  const data = isRecord(frame.data) ? frame.data : frame;
  switch (frame.type) {
    case 'payin.credited':
      return isRecord(data.payin) ? { type: 'payin.credited', payin: toPayin(data.payin as Schemas['Payin'], directory) } : null;
    case 'payout.updated':
    case 'approval.requested':
      return isRecord(data.payout) ? { type: frame.type, payout: toPayout(data.payout as Schemas['Payout'], directory) } : null;
    case 'balance.changed':
      return { type: 'balance.changed', accountId: typeof data.accountId === 'string' ? data.accountId : null };
    default:
      return null;
  }
}
