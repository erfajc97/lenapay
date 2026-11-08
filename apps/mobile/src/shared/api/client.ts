import type {
  Account,
  AccountCvu,
  ApprovalPolicy,
  AuthTokens,
  CreateAccountRequest,
  CreateOrgRequest,
  CreatePayerRequest,
  CreatePayoutRequest,
  Cvu,
  Destination,
  Device,
  ErpCandidate,
  ErpLink,
  ErpLinkRequest,
  InstallmentList,
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
  LocalUpload,
  LoginRequest,
  Me,
  Member,
  Movement,
  MovementFilters,
  MyPaymentCompany,
  MyPaymentPayin,
  Org,
  OrgBalances,
  OwnershipTransfer,
  Page,
  PayCompanyRequest,
  Payer,
  Payin,
  Payout,
  PayoutAttachment,
  PayoutQuote,
  PayoutQuoteRequest,
  PushPlatform,
  ReconciliationItem,
  ReconciliationRun,
  RegisterDeviceRequest,
  RejectPayoutRequest,
  ResendCodeRequest,
  RunReconciliationRequest,
  SignupRequest,
  SignupResponse,
  StepUpChallenge,
  StepUpChallengeRequest,
  StepUpToken,
  StepUpVerifyRequest,
  TransferOwnershipRequest,
  UpdateAccountRequest,
  UpdateApprovalPolicyRequest,
  UpdateCvuRequest,
  UpdateMeRequest,
  UpdateMemberRequest,
  UpdatePreferencesRequest,
  UserPreferences,
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

/** Owner actions that decide who moves money (ownership, approval policy) need a fresh step-up. */
export interface StepUpOptions {
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
 * this interface through `api` (mock or live, chosen at runtime by `shared/config/apiTarget`).
 */
export interface LenapayApi {
  auth: {
    signup(body: SignupRequest): Promise<SignupResponse>;
    /** Verifies the email; the person then signs in (the contract answers 204, no tokens). */
    verifyEmail(body: VerifyEmailRequest): Promise<void>;
    resendCode(body: ResendCodeRequest): Promise<void>;
    login(body: LoginRequest): Promise<AuthTokens>;
    refresh(refreshToken: string): Promise<AuthTokens>;
    logout(): Promise<void>;
    stepUpChallenge(body: StepUpChallengeRequest): Promise<StepUpChallenge>;
    stepUpVerify(body: StepUpVerifyRequest): Promise<StepUpToken>;
    registerDevice(body: RegisterDeviceRequest): Promise<Device>;
    me(): Promise<Me>;
  };
  /** The caller's own profile and synced preferences (R-ID-5). */
  profile: {
    update(body: UpdateMeRequest): Promise<Me>;
    preferences(): Promise<UserPreferences>;
    updatePreferences(body: UpdatePreferencesRequest): Promise<UserPreferences>;
  };
  orgs: {
    list(): Promise<Org[]>;
    create(body: CreateOrgRequest): Promise<Org>;
    members(orgId: string): Promise<Member[]>;
    invite(orgId: string, body: InviteRequest): Promise<Invite>;
    updateMember(orgId: string, userId: string, body: UpdateMemberRequest): Promise<Member>;
    removeMember(orgId: string, userId: string): Promise<void>;
    /** Owner only, step-up (R-ORG-3): the previous owner stays as admin. */
    transferOwnership(orgId: string, body: TransferOwnershipRequest, options: StepUpOptions): Promise<OwnershipTransfer>;
    approvalPolicy(orgId: string): Promise<ApprovalPolicy>;
    /** Owner only, step-up (R-OUT-7). */
    updateApprovalPolicy(orgId: string, body: UpdateApprovalPolicyRequest, options: StepUpOptions): Promise<ApprovalPolicy>;
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
    /** Changes the alias of the account's own CVU (personal wallet, R-CVU-4). */
    updateCvu(orgId: string, accountId: string, cvuId: string, body: UpdateCvuRequest): Promise<AccountCvu>;
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
    uploadAttachment(orgId: string, payoutId: string, file: LocalUpload): Promise<PayoutAttachment>;
    proof(orgId: string, payoutId: string): Promise<LocalFile>;
  };
  /** "Mis pagos" (R-MP-*): the caller's own verified personal org, resolved by the API. */
  myPayments: {
    companies(options: { includeHidden: boolean }): Promise<MyPaymentCompany[]>;
    setHidden(linkId: string, hidden: boolean): Promise<MyPaymentCompany>;
    payins(linkId: string, cursor: string | null): Promise<Page<MyPaymentPayin>>;
    installments(linkId: string): Promise<InstallmentList>;
    pay(linkId: string, body: PayCompanyRequest, options: MoneyActionOptions): Promise<Payout>;
  };
  movements: {
    list(orgId: string, filters: MovementFilters, cursor: string | null): Promise<Page<Movement>>;
    get(orgId: string, movementId: string): Promise<Movement>;
  };
  reconciliation: {
    runs(orgId: string): Promise<ReconciliationRun[]>;
    items(orgId: string, runId: string): Promise<ReconciliationItem[]>;
    run(orgId: string, body: RunReconciliationRequest): Promise<ReconciliationRun>;
  };
  erp: {
    candidate(orgId: string): Promise<ErpCandidate>;
    link(orgId: string, body: ErpLinkRequest): Promise<ErpLink>;
    get(orgId: string): Promise<ErpLink>;
  };
  notifications: {
    registerPushToken(token: string, platform: PushPlatform): Promise<void>;
  };
  events: {
    /** Same interface for the mock timer and the live SSE stream. Returns the unsubscribe. */
    subscribe(orgId: string, onEvent: (event: LiveEvent) => void): () => void;
  };
}
