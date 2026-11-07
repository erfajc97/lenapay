/**
 * Lenapay API DTOs (architecture §5). This is the ONE file to swap for
 * `@lenapay/api-types` once the generated contract types exist: every service, mock and
 * screen imports its shapes from here.
 *
 * Money is always integer minor units + ISO currency (R-NF-2).
 */

export type Currency = 'ARS' | 'USD';
export const CURRENCIES: readonly Currency[] = ['ARS', 'USD'];

export type Role = 'viewer' | 'operator' | 'approver' | 'admin';
export const ROLES: readonly Role[] = ['viewer', 'operator', 'approver', 'admin'];
export type OrgKind = 'company' | 'personal';
export type KycStatus = 'draft' | 'submitted' | 'approved' | 'rejected';

export interface ApiErrorBody {
  error: { code: string; message: string; details?: Record<string, unknown> };
}

// ── Auth ────────────────────────────────────────────────────────────────────────────────
export interface AuthTokens {
  accessToken: string;
  refreshToken: string;
  expiresIn: number;
}
export interface SignupRequest {
  email: string;
  password: string;
  fullName: string;
}
export interface SignupResponse {
  userId: string;
  email: string;
}
export interface VerifyEmailRequest {
  email: string;
  code: string;
}
export interface LoginRequest {
  email: string;
  password: string;
  provider?: 'lebane';
}
export interface StepUpChallenge {
  challengeId: string;
  nonce: string;
  expiresAt: string;
}
export interface StepUpVerifyRequest {
  challengeId: string;
  method: 'device_key' | 'password';
  signature?: string;
  password?: string;
}
export interface StepUpToken {
  stepUpToken: string;
  expiresAt: string;
}
export interface Membership {
  orgId: string;
  role: Role;
}
export interface Me {
  id: string;
  email: string;
  fullName: string;
  mfaEnabled: boolean;
  personalModeEnabled: boolean;
  memberships: Membership[];
}

// ── Orgs and team ───────────────────────────────────────────────────────────────────────
export interface Org {
  id: string;
  kind: OrgKind;
  legalName: string;
  cuit: string | null;
  kycStatus: KycStatus;
  role: Role;
  erpLinked: boolean;
}
export interface CreateOrgRequest {
  kind: OrgKind;
  legalName: string;
}
export interface Member {
  userId: string;
  fullName: string;
  email: string;
  role: Role;
  status: 'active' | 'invited';
}
export interface InviteRequest {
  email: string;
  role: Role;
}
export interface Invite {
  id: string;
  email: string;
  role: Role;
  expiresAt: string;
}

// ── KYC ─────────────────────────────────────────────────────────────────────────────────
export type KycDocumentType = 'bylaws' | 'afip_constancy' | 'dni_front' | 'dni_back' | 'selfie';
export interface KycDocument {
  id: string;
  type: KycDocumentType;
  fileName: string;
  representativeId: string | null;
  uploadedAt: string;
}
export interface Representative {
  id: string;
  fullName: string;
  dni: string;
}
export interface KycCase {
  id: string;
  orgId: string;
  kind: 'company' | 'person';
  status: KycStatus;
  cuit: string | null;
  legalName: string | null;
  legalAddress: string | null;
  fullName: string | null;
  dni: string | null;
  representatives: Representative[];
  documents: KycDocument[];
  timeline: { status: KycStatus; at: string }[];
  rejectionReason: string | null;
}
export interface KycUpdateRequest {
  kind: 'company' | 'person';
  cuit?: string;
  legalName?: string;
  legalAddress?: string;
  fullName?: string;
  dni?: string;
  representatives?: Representative[];
}
export interface KycDocumentUpload {
  type: KycDocumentType;
  fileName: string;
  fileUri: string;
  representativeId?: string;
}

// ── Accounts and balances ───────────────────────────────────────────────────────────────
export type AccountKind = 'main' | 'project' | 'personal';
export interface Balance {
  currency: Currency;
  /** Posted ledger balance, minor units. */
  posted: number;
  /** Held for payouts in flight, minor units. */
  held: number;
  /** posted − held, minor units. */
  available: number;
}
export interface Account {
  id: string;
  kind: AccountKind;
  name: string;
  archivedAt: string | null;
  balances: Balance[];
  payersCount: number;
}
export interface OrgBalances {
  totals: Balance[];
}
export interface CreateAccountRequest {
  name: string;
}
export interface UpdateAccountRequest {
  name?: string;
  archived?: boolean;
}
export interface InternalTransferRequest {
  fromAccountId: string;
  toAccountId: string;
  currency: Currency;
  amount: number;
  concept: string;
}
export interface InternalTransfer {
  id: string;
  fromAccountId: string;
  toAccountId: string;
  currency: Currency;
  amount: number;
  createdAt: string;
}

// ── Payers and CVUs ─────────────────────────────────────────────────────────────────────
export type DocType = 'CUIT' | 'DNI';
export interface Cvu {
  id: string;
  payerId: string;
  currency: Currency;
  cvu: string;
  alias: string;
  status: 'active' | 'disabled';
}
export interface Payer {
  id: string;
  accountId: string;
  accountName: string;
  name: string;
  docType: DocType;
  docNumber: string;
  email: string | null;
  erpRef: string | null;
  cvus: Cvu[];
  createdAt: string;
}
export interface CreatePayerRequest {
  accountId: string;
  name: string;
  docType: DocType;
  docNumber: string;
  email?: string;
}
export interface IssueCvuRequest {
  currency: Currency;
}
export interface UpdateCvuRequest {
  alias: string;
}

// ── Payins ──────────────────────────────────────────────────────────────────────────────
export interface Payin {
  id: string;
  cvuId: string;
  payerId: string;
  payerName: string;
  accountId: string;
  accountName: string;
  currency: Currency;
  amount: number;
  fee: number;
  net: number;
  originName: string;
  originCuit: string;
  createdAt: string;
}

// ── Payouts and approvals ───────────────────────────────────────────────────────────────
export type PayoutStatus =
  | 'pending_approval'
  | 'approved'
  | 'sending'
  | 'settled'
  | 'failed'
  | 'rejected'
  | 'cancelled';
export interface Destination {
  type: 'cbu' | 'cvu' | 'alias';
  cbu: string;
  alias: string | null;
  holderName: string;
  holderCuit: string;
  bankName: string;
}
export interface PayoutQuoteRequest {
  accountId: string;
  currency: Currency;
  amount: number;
}
export interface PayoutQuote {
  accountId: string;
  currency: Currency;
  amount: number;
  fee: number;
  total: number;
  available: number;
}
export interface CreatePayoutRequest {
  accountId: string;
  currency: Currency;
  amount: number;
  to: string;
  concept: string;
  attachmentName?: string;
}
export interface PersonRef {
  userId: string;
  fullName: string;
}
export interface Payout {
  id: string;
  accountId: string;
  accountName: string;
  currency: Currency;
  amount: number;
  fee: number;
  destination: Destination;
  concept: string;
  attachmentName: string | null;
  status: PayoutStatus;
  createdBy: PersonRef;
  decidedBy: PersonRef | null;
  rejectionReason: string | null;
  createdAt: string;
  events: { status: PayoutStatus; at: string }[];
}
export interface RejectPayoutRequest {
  reason: string;
}

// ── Movements ───────────────────────────────────────────────────────────────────────────
export type MovementType = 'payin' | 'payout' | 'transfer';
export type MovementStatus = 'pending' | 'settled' | 'failed' | 'cancelled';
export interface Movement {
  id: string;
  type: MovementType;
  direction: 'in' | 'out';
  status: MovementStatus;
  currency: Currency;
  amount: number;
  fee: number;
  accountId: string;
  accountName: string;
  counterparty: string;
  counterpartyDoc: string | null;
  concept: string | null;
  refId: string;
  createdAt: string;
}
export interface MovementFilters {
  account?: string;
  type?: MovementType;
  status?: MovementStatus;
  currency?: Currency;
  from?: string;
  to?: string;
}
export interface Page<T> {
  items: T[];
  nextCursor: string | null;
}

// ── Reconciliation ──────────────────────────────────────────────────────────────────────
export type ReconItemStatus = 'matched' | 'missing_in_ledger' | 'missing_at_psp';
export interface ReconciliationItem {
  id: string;
  status: ReconItemStatus;
  providerTransferId: string;
  amount: number;
  description: string;
}
export interface ReconciliationRun {
  id: string;
  currency: Currency;
  day: string;
  matched: number;
  missingInLedger: number;
  missingAtPsp: number;
  pspTotal: number;
  ledgerTotal: number;
  items: ReconciliationItem[];
  createdAt: string;
}

// ── ERP link ────────────────────────────────────────────────────────────────────────────
export interface ErpCandidate {
  found: boolean;
  erpOrgName: string | null;
  cuit: string | null;
  projectsCount: number;
  buyersCount: number;
}
export interface ErpLinkRequest {
  username: string;
  password: string;
}
export interface ErpLink {
  status: 'not_linked' | 'importing' | 'linked';
  linkedAt: string | null;
  projects: { done: number; total: number };
  payers: { done: number; total: number };
  importedProjects: string[];
}

// ── Realtime ────────────────────────────────────────────────────────────────────────────
export type LiveEvent =
  | { type: 'payin.credited'; payin: Payin }
  | { type: 'payout.updated'; payout: Payout };
