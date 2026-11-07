import type {
  Account,
  AuthTokens,
  CreateAccountRequest,
  CreateOrgRequest,
  CreatePayerRequest,
  CreatePayoutRequest,
  Cvu,
  Destination,
  ErpCandidate,
  ErpLink,
  ErpLinkRequest,
  InternalTransfer,
  InternalTransferRequest,
  Invite,
  InviteRequest,
  IssueCvuRequest,
  KycCase,
  KycDocument,
  KycDocumentUpload,
  KycUpdateRequest,
  LiveEvent,
  LoginRequest,
  Me,
  Member,
  Movement,
  MovementFilters,
  Org,
  OrgBalances,
  Page,
  Payer,
  Payin,
  Payout,
  PayoutQuote,
  PayoutQuoteRequest,
  ReconciliationRun,
  RejectPayoutRequest,
  SignupRequest,
  SignupResponse,
  StepUpChallenge,
  StepUpToken,
  StepUpVerifyRequest,
  UpdateAccountRequest,
  UpdateCvuRequest,
  VerifyEmailRequest,
} from './types';

/**
 * Headers every money-moving call carries (R-ID-3, R-OUT-3, MO2): both are REQUIRED in the
 * signature so no caller can forget them.
 */
export interface MoneyActionOptions {
  idempotencyKey: string;
  stepUpToken: string;
}

export interface PayerQuery {
  search?: string;
  account?: string;
}

/** A local file the app can share (proof of payment PDF). */
export interface LocalFile {
  uri: string;
  mimeType: string;
}

/**
 * The typed client abstraction. `mock` and `live` both implement it; features only see
 * this interface through `api` (selected by EXPO_PUBLIC_API_MODE).
 */
export interface LenapayApi {
  auth: {
    signup(body: SignupRequest): Promise<SignupResponse>;
    verifyEmail(body: VerifyEmailRequest): Promise<AuthTokens>;
    login(body: LoginRequest): Promise<AuthTokens>;
    refresh(refreshToken: string): Promise<AuthTokens>;
    logout(): Promise<void>;
    stepUpChallenge(): Promise<StepUpChallenge>;
    stepUpVerify(body: StepUpVerifyRequest): Promise<StepUpToken>;
    me(): Promise<Me>;
  };
  orgs: {
    list(): Promise<Org[]>;
    create(body: CreateOrgRequest): Promise<Org>;
    members(orgId: string): Promise<Member[]>;
    invite(orgId: string, body: InviteRequest): Promise<Invite>;
  };
  kyc: {
    get(orgId: string): Promise<KycCase>;
    update(orgId: string, body: KycUpdateRequest): Promise<KycCase>;
    uploadDocument(orgId: string, body: KycDocumentUpload): Promise<KycDocument>;
    submit(orgId: string): Promise<KycCase>;
  };
  accounts: {
    list(orgId: string): Promise<Account[]>;
    balances(orgId: string): Promise<OrgBalances>;
    create(orgId: string, body: CreateAccountRequest): Promise<Account>;
    update(orgId: string, accountId: string, body: UpdateAccountRequest): Promise<Account>;
    transfer(orgId: string, body: InternalTransferRequest, options: MoneyActionOptions): Promise<InternalTransfer>;
  };
  payers: {
    list(orgId: string, query: PayerQuery): Promise<Payer[]>;
    get(orgId: string, payerId: string): Promise<Payer>;
    create(orgId: string, body: CreatePayerRequest): Promise<Payer>;
    issueCvu(orgId: string, payerId: string, body: IssueCvuRequest, options: MoneyActionOptions): Promise<Cvu>;
    updateCvu(orgId: string, cvuId: string, body: UpdateCvuRequest): Promise<Cvu>;
    disableCvu(orgId: string, cvuId: string): Promise<void>;
  };
  payins: {
    get(orgId: string, payinId: string): Promise<Payin>;
  };
  payouts: {
    resolveDestination(orgId: string, to: string): Promise<Destination>;
    quote(orgId: string, body: PayoutQuoteRequest): Promise<PayoutQuote>;
    create(orgId: string, body: CreatePayoutRequest, options: MoneyActionOptions): Promise<Payout>;
    get(orgId: string, payoutId: string): Promise<Payout>;
    approve(orgId: string, payoutId: string, options: MoneyActionOptions): Promise<Payout>;
    reject(orgId: string, payoutId: string, body: RejectPayoutRequest, options: MoneyActionOptions): Promise<Payout>;
    approvals(orgId: string): Promise<Payout[]>;
    proof(orgId: string, payoutId: string): Promise<LocalFile>;
  };
  movements: {
    list(orgId: string, filters: MovementFilters, cursor: string | null): Promise<Page<Movement>>;
  };
  reconciliation: {
    runs(orgId: string): Promise<ReconciliationRun[]>;
  };
  erp: {
    candidate(orgId: string): Promise<ErpCandidate>;
    link(orgId: string, body: ErpLinkRequest): Promise<ErpLink>;
    get(orgId: string): Promise<ErpLink>;
  };
  notifications: {
    registerPushToken(token: string): Promise<void>;
  };
  events: {
    /** Same interface for the mock timer and the live SSE stream. Returns the unsubscribe. */
    subscribe(orgId: string, onEvent: (event: LiveEvent) => void): () => void;
  };
}
