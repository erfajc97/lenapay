import type { components } from '@lenapay/api-types';

/**
 * The app's API types. Every wire shape comes from the generated contract types
 * (`@lenapay/api-types`, `make gen`); this file only re-exports them and adds the few view
 * models the screens need (money as plain minor units, names joined in). The live client maps
 * contract DTOs to these view models (`live/mappers.ts`); the mock backend produces them
 * directly. Money is always integer minor units + ISO currency (R-NF-2).
 */
type Schemas = components['schemas'];

/**
 * Exhaustive list of a contract enum: the compiler fails when the contract adds a member the
 * app does not list, so a new role or currency cannot be silently missing from a picker.
 */
function enumValues<T extends string>() {
  return <const V extends readonly T[]>(values: V & ([T] extends [V[number]] ? unknown : 'misses a contract member')): readonly T[] => values;
}

// ── Contract enums ──────────────────────────────────────────────────────────────────────
export type Currency = Schemas['Currency'];
export const CURRENCIES = enumValues<Currency>()(['ARS', 'USD']);
/** A role that can be granted (invites, role changes); `owner` is never one of them. */
export type Role = Schemas['Role'];
export const ROLES = enumValues<Role>()(['viewer', 'operator', 'approver', 'admin']);
/** A member's role, least to most privileged; exactly one `owner` per organization (R-ORG-3). */
export type MemberRole = Schemas['MemberRole'];
export const MEMBER_ROLES = enumValues<MemberRole>()(['viewer', 'operator', 'approver', 'admin', 'owner']);
export type ApprovalMode = Schemas['ApprovalMode'];
export type Locale = Schemas['Locale'];
export type Theme = Schemas['Theme'];
export type OrgKind = Schemas['OrgKind'];
export type KycStatus = Schemas['KycStatus'];
export type DocType = Schemas['DocType'];
export type AccountKind = Schemas['AccountKind'];
export type PayoutStatus = Schemas['PayoutStatus'];
export type MovementType = Schemas['MovementType'];
export const MOVEMENT_TYPES = enumValues<MovementType>()(['payin', 'payout', 'internal_transfer']);
export type MovementStatus = Schemas['MovementStatus'];
export const MOVEMENT_STATUSES = enumValues<MovementStatus>()(['pending', 'completed', 'failed', 'cancelled']);
type MovementDirection = Schemas['MovementDirection'];
export type KycDocumentKind = Schemas['KycDocumentKind'];
type ReconciliationItemStatus = Schemas['ReconciliationItemStatus'];
export type PushPlatform = Schemas['RegisterPushTokenRequest']['platform'];
/** How the money moves: `bank` through the PSP, `internal` instantly between Lenapay accounts. */
export type Rail = Schemas['Rail'];
export const RAILS = enumValues<Rail>()(['bank', 'internal']);
export type InstallmentStatus = Schemas['InstallmentStatus'];
export type AdjustmentIndex = Schemas['AdjustmentIndex'];

// ── Contract DTOs used as they are ──────────────────────────────────────────────────────
export type ApiErrorBody = Schemas['Error'];
export type AuthTokens = Schemas['TokenPair'];
export type SignupRequest = Schemas['SignupRequest'];
export type SignupResponse = Schemas['SignupResponse'];
export type VerifyEmailRequest = Schemas['VerifyEmailRequest'];
export type ResendCodeRequest = Schemas['ResendCodeRequest'];
export type LoginRequest = Schemas['LoginRequest'];
export type StepUpChallengeRequest = Schemas['StepUpChallengeRequest'];
export type StepUpChallenge = Schemas['StepUpChallenge'];
export type StepUpVerifyRequest = Schemas['StepUpVerifyRequest'];
export type StepUpToken = Schemas['StepUpToken'];
export type RegisterDeviceRequest = Schemas['RegisterDeviceRequest'];
export type Device = Schemas['Device'];
type Membership = Schemas['Membership'];
export type Org = Schemas['Organization'];
export type CreateOrgRequest = Schemas['CreateOrgRequest'];
export type InviteRequest = Schemas['CreateInviteRequest'];
export type Invite = Schemas['Invite'];
export type UpdateMemberRequest = Schemas['UpdateMemberRequest'];
export type Address = Schemas['Address'];
export type KycPersonData = Schemas['KycPersonData'];
export type KycUpdateRequest = Schemas['UpdateKycRequest'];
export type KycDocument = Schemas['KycDocument'];
export type KycCase = Schemas['KycCase'];
export type CreateAccountRequest = Schemas['CreateAccountRequest'];
export type UpdateAccountRequest = Schemas['UpdateAccountRequest'];
export type Cvu = Schemas['Cvu'];
export type CreatePayerRequest = Schemas['CreatePayerRequest'];
export type IssueCvuRequest = Schemas['CreateCvuRequest'];
export type UpdateCvuRequest = Schemas['UpdateCvuRequest'];
/** A receiving CVU of the account itself (the personal wallet's ARS and USD CVUs). */
export type AccountCvu = Schemas['AccountCvu'];
export type MyPaymentCvu = Schemas['MyPaymentCvu'];
export type PayinStatus = Schemas['PayinStatus'];
export type Destination = Schemas['Destination'];
export type PayoutAttachment = Schemas['PayoutAttachment'];
export type RejectPayoutRequest = Schemas['RejectPayoutRequest'];
export type ErpCandidate = Schemas['ErpLinkCandidate'];
export type ErpLinkRequest = Schemas['CreateErpLinkRequest'];
export type RunReconciliationRequest = Schemas['RunReconciliationRequest'];
export type UpdateMeRequest = Schemas['UpdateMeRequest'];
export type UserPreferences = Schemas['UserPreferences'];
export type UpdatePreferencesRequest = Schemas['UpdatePreferencesRequest'];
export type TransferOwnershipRequest = Schemas['TransferOwnershipRequest'];
/** The org's payout approval policy (R-OUT-7); thresholds per currency in minor units. */
export type ApprovalPolicy = Schemas['ApprovalPolicy'];
export type UpdateApprovalPolicyRequest = Schemas['UpdateApprovalPolicyRequest'];
export type ErpImportSummary = Schemas['ErpImportSummary'];
export type ErpInvitedUser = Schemas['ErpInvitedUser'];

// ── View models (mapped from the contract in live/mappers.ts) ─────────────────────────────
export interface Me {
  id: string;
  email: string;
  /** Null until the person sets one (screens fall back to the email). */
  fullName: string | null;
  mfaEnabled: boolean;
  /** The person owns a personal wallet (a `personal` membership). */
  personalModeEnabled: boolean;
  memberships: Membership[];
  /** Synced across devices (R-ID-5): every field null until set. */
  preferences: UserPreferences;
}

export interface Member {
  userId: string;
  email: string;
  /** Null when the person has not set one. */
  fullName: string | null;
  role: MemberRole;
  /** Live lists members only; the mock also lists pending invites. */
  status: 'active' | 'invited';
}

/** `POST /ownership-transfer`: the previous owner (now admin) and the new owner. */
export interface OwnershipTransfer {
  previousOwner: Member;
  owner: Member;
}

/** A file picked on the device, ready for a multipart upload. */
export interface LocalUpload {
  fileName: string;
  fileUri: string;
  mimeType: string;
}

export interface KycDocumentUpload extends LocalUpload {
  kind: KycDocumentKind;
  representativeDni?: string;
}

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
  archived: boolean;
  erpProjectRef: string | null;
  balances: Balance[];
  /** Mock only (the contract has no count): null in live mode. */
  payersCount: number | null;
  /** The account's own receiving CVUs (personal wallet); `[]` for company accounts. */
  cvus: AccountCvu[];
  createdAt: string;
}

export interface OrgBalances {
  totals: Balance[];
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
  concept: string | null;
  createdAt: string;
}

export type Payer = Schemas['Payer'] & {
  /** Joined from the org's accounts. */
  accountName: string;
};

export interface Payin {
  id: string;
  accountId: string;
  accountName: string;
  payerId: string | null;
  cvuId: string;
  currency: Currency;
  /** Gross amount the payer sent, minor units. */
  amount: number;
  fee: number;
  net: number;
  originName: string | null;
  originCuit: string | null;
  createdAt: string;
}

export interface PayoutQuoteRequest {
  accountId: string;
  currency: Currency;
  amount: number;
  to: string;
}

export interface PayoutQuote {
  currency: Currency;
  amount: number;
  fee: number;
  total: number;
  /** Available balance after holding the total; negative means it does not fit. */
  availableAfter: number;
  /** `internal` when the destination is a Lenapay CVU or alias: instant, never through the bank. */
  rail: Rail;
}

export interface CreatePayoutRequest {
  accountId: string;
  currency: Currency;
  amount: number;
  to: string;
  concept: string;
}

/** A member referenced by a payout; the label is the name or email when known. */
interface PersonRef {
  userId: string;
  label: string | null;
}

interface PayoutEvent {
  status: PayoutStatus;
  at: string;
}

export interface Payout {
  id: string;
  accountId: string;
  accountName: string;
  currency: Currency;
  amount: number;
  fee: number;
  total: number;
  destination: Destination;
  concept: string;
  status: PayoutStatus;
  rail: Rail;
  createdBy: PersonRef;
  approvedBy: PersonRef | null;
  rejectReason: string | null;
  failureCode: string | null;
  attachments: PayoutAttachment[];
  events: PayoutEvent[];
  createdAt: string;
  settledAt: string | null;
}

export interface Movement {
  id: string;
  type: MovementType;
  direction: MovementDirection;
  status: MovementStatus;
  currency: Currency;
  amount: number;
  fee: number | null;
  accountId: string;
  accountName: string;
  counterparty: string | null;
  counterpartyDoc: string | null;
  concept: string | null;
  refId: string;
  rail: Rail;
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

export interface ReconciliationRun {
  id: string;
  currency: Currency;
  day: string;
  status: Schemas['ReconciliationRunStatus'];
  matched: number;
  missingInLedger: number;
  missingAtPsp: number;
  pspTotal: number;
  ledgerTotal: number;
  difference: number;
  createdAt: string;
  finishedAt: string | null;
}

export interface ReconciliationItem {
  id: string;
  status: ReconciliationItemStatus;
  providerTransferId: string;
  direction: MovementDirection;
  /** Null when the transfer is missing at the PSP. */
  pspAmount: number | null;
  /** Null when the transfer is missing in the ledger. */
  ledgerAmount: number | null;
  createdAt: string;
}

export type ErpLinkStatus = 'not_linked' | 'importing' | 'linked' | 'failed';

export interface ErpLink {
  status: ErpLinkStatus;
  displayName: string | null;
  linkedAt: string | null;
  importedAt: string | null;
  /** What the import brought (R-ERP-4); null until it ran. */
  importSummary: ErpImportSummary | null;
}

// ── Mis pagos (payer view) ──────────────────────────────────────────────────────────────
interface MoneyAmount {
  currency: Currency;
  amount: number;
}

export interface MyPaymentCompany {
  linkId: string;
  companyName: string;
  /** Null when the person pays into the company's main account. */
  projectName: string | null;
  cvus: MyPaymentCvu[];
  /** Credited payins summed per currency, minor units. */
  totalPaid: MoneyAmount[];
  lastPaymentAt: string | null;
  erpLinked: boolean;
  hidden: boolean;
}

export interface MyPaymentPayin {
  id: string;
  cvu: string;
  currency: Currency;
  amount: number;
  status: PayinStatus;
  createdAt: string;
  creditedAt: string | null;
}

export interface Installment {
  number: number;
  dueDate: string;
  currency: Currency;
  amount: number;
  status: InstallmentStatus;
  adjustmentIndex: AdjustmentIndex | null;
}

export interface InstallmentList {
  erpLinked: boolean;
  items: Installment[];
}

export interface PayCompanyRequest {
  currency: Currency;
  amount: number;
  /** The installment paid; the API writes "Cuota N - …" in the transfer concept. */
  installmentNumber?: number;
  concept?: string;
}

// ── Realtime ────────────────────────────────────────────────────────────────────────────
export type LiveEvent =
  | { type: 'payin.credited'; payin: Payin }
  | { type: 'payout.updated'; payout: Payout }
  | { type: 'approval.requested'; payout: Payout }
  | { type: 'balance.changed'; accountId: string | null }
  /** App-level: the stream reconnected and frames may have been missed, so refetch. */
  | { type: 'resync' };
