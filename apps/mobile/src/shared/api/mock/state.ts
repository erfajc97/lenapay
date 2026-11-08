import type {
  AccountKind,
  Currency,
  ErpCandidate,
  Invite,
  KycCase,
  Org,
  Payer,
  Payin,
  Payout,
  ReconciliationItem,
  ReconciliationRun,
  Role,
} from '../types';

/** Internal records of the in-memory backend. Never leave `mock/`: the API returns view models. */
interface MockUser {
  id: string;
  email: string;
  fullName: string | null;
  mfaEnabled: boolean;
  personalModeEnabled: boolean;
}

type MockOrg = Omit<Org, 'role'>;

interface MockMembership {
  orgId: string;
  userId: string;
  role: Role;
}

export interface MockAccount {
  id: string;
  orgId: string;
  kind: AccountKind;
  name: string;
  archived: boolean;
  createdAt: string;
  ledger: Record<Currency, { posted: number; held: number }>;
}

export interface MockPayer extends Omit<Payer, 'accountName'> {
  orgId: string;
}

export interface MockPayin extends Payin {
  orgId: string;
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

interface MockErp {
  candidate: ErpCandidate;
  projects: string[];
  payers: MockErpPayer[];
  linkStartedAt: number | null;
  linkedAt: string | null;
}

export interface MockReconRun {
  run: ReconciliationRun;
  items: ReconciliationItem[];
}

export interface MockState {
  seq: number;
  currentUserId: string | null;
  pendingSignup: { email: string } | null;
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
  idempotency: Record<string, { hash: string; response: unknown }>;
}
