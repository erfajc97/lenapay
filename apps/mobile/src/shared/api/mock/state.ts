import type {
  AccountCvu,
  AccountKind,
  ApprovalPolicy,
  Currency,
  ErpCandidate,
  ErpImportSummary,
  Installment,
  Invite,
  KycCase,
  MemberRole,
  Org,
  Payer,
  Payin,
  Payout,
  Rail,
  ReconciliationItem,
  ReconciliationRun,
  Role,
  UserPreferences,
} from '../types';

/** Internal records of the in-memory backend. Never leave `mock/`: the API returns view models. */
interface MockUser {
  id: string;
  email: string;
  fullName: string | null;
  mfaEnabled: boolean;
}

type MockOrg = Omit<Org, 'role'>;

interface MockMembership {
  orgId: string;
  userId: string;
  role: MemberRole;
}

/** The stored policy; `ownerSelfApprovalEffective` is derived from the members on read. */
export type MockApprovalPolicy = Omit<ApprovalPolicy, 'ownerSelfApprovalEffective'>;

export interface MockAccount {
  id: string;
  orgId: string;
  kind: AccountKind;
  name: string;
  archived: boolean;
  createdAt: string;
  ledger: Record<Currency, { posted: number; held: number }>;
  /** The account's own receiving CVUs (personal wallets, once the person KYC is approved). */
  cvus: AccountCvu[];
}

export interface MockPayer extends Omit<Payer, 'accountName'> {
  orgId: string;
}

export interface MockPayin extends Payin {
  orgId: string;
  rail: Rail;
}

export interface MockPayout extends Payout {
  orgId: string;
}

export interface MockTransfer {
  id: string;
  orgId: string;
  fromAccountId: string;
  toAccountId: string;
  currency: Currency;
  amount: number;
  concept: string;
  createdAt: string;
}

export interface MockErpPayer {
  name: string;
  docType: Payer['docType'];
  docNumber: string;
  project: string;
}

/** An ERP user: `role` is mapped from their Lebane permissions; null = no payments access. */
export interface MockErpUser {
  email: string;
  role: Role | null;
}

interface MockErp {
  candidate: ErpCandidate;
  projects: string[];
  payers: MockErpPayer[];
  users: MockErpUser[];
  linkStartedAt: number | null;
  /** The admin or owner who linked: invites are capped at their role (never owner). */
  linkedBy: string | null;
  linkedAt: string | null;
  importSummary: ErpImportSummary | null;
}

/** "Mis pagos": a verified person (their personal org) ↔ a company's payer with their DNI. */
export interface MockPayerLink {
  id: string;
  personalOrgId: string;
  payerId: string;
  hidden: boolean;
}

export interface MockReconRun {
  run: ReconciliationRun;
  items: ReconciliationItem[];
}

export interface MockState {
  seq: number;
  currentUserId: string | null;
  pendingSignup: { email: string; fullName: string | null } | null;
  users: MockUser[];
  devices: { id: string; userId: string; publicKey: string }[];
  orgs: MockOrg[];
  memberships: MockMembership[];
  invites: (Invite & { orgId: string })[];
  kyc: Record<string, KycCase>;
  kycSubmittedAt: Record<string, number>;
  accounts: MockAccount[];
  payers: MockPayer[];
  payins: MockPayin[];
  payouts: MockPayout[];
  transfers: MockTransfer[];
  recon: Record<string, MockReconRun[]>;
  erp: Record<string, MockErp>;
  /** DNI the person KYC verified, per personal org (copied on approval, like the API). */
  verifiedDni: Record<string, string>;
  payerLinks: MockPayerLink[];
  /** ERP installments (cuotas) per payer id, for ERP-linked companies. */
  installments: Record<string, Installment[]>;
  idempotency: Record<string, { hash: string; response: unknown }>;
  /** Approval policy per org (R-OUT-7); an org without one has the API's default. */
  policies: Record<string, MockApprovalPolicy>;
  /** Synced preferences per user id (R-ID-5). */
  preferences: Record<string, UserPreferences>;
}
