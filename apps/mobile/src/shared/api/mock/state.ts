import type {
  AccountKind,
  Currency,
  ErpCandidate,
  InternalTransfer,
  Invite,
  KycCase,
  Org,
  Payer,
  Payin,
  Payout,
  ReconciliationRun,
  Role,
} from '../types';

/** Internal records of the in-memory backend. Never leave `mock/`: the API returns DTOs. */
interface MockUser {
  id: string;
  email: string;
  fullName: string;
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
  archivedAt: string | null;
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

export interface MockTransfer extends InternalTransfer {
  orgId: string;
  concept: string;
}

interface MockErp {
  candidate: ErpCandidate;
  projects: string[];
  payers: { name: string; docType: 'CUIT' | 'DNI'; docNumber: string; project: string }[];
  linkStartedAt: number | null;
  linkedAt: string | null;
}

export interface MockState {
  seq: number;
  currentUserId: string | null;
  pendingSignup: { email: string; fullName: string } | null;
  users: MockUser[];
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
  recon: Record<string, ReconciliationRun[]>;
  erp: Record<string, MockErp>;
  idempotency: Record<string, { hash: string; response: unknown }>;
}
