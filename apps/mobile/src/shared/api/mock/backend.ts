import { buildCvu, classifyDestination } from '@/shared/helpers/cbu';
import { isValidCuit } from '@/shared/helpers/cuit';
import { approvalBlocker, moneyActionBlocker } from '@/shared/helpers/permissions';

import type { MoneyActionOptions, PayerQuery } from '../client';
import { ApiError } from '../errors';
import type {
  Account,
  AccountCvu,
  AuthTokens,
  Balance,
  CreateAccountRequest,
  CreateOrgRequest,
  CreatePayerRequest,
  CreatePayoutRequest,
  Currency,
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
  Page,
  PayCompanyRequest,
  Payer,
  Payin,
  Payout,
  PayoutAttachment,
  PayoutQuote,
  PayoutQuoteRequest,
  PayoutStatus,
  Rail,
  ReconciliationItem,
  ReconciliationRun,
  RegisterDeviceRequest,
  RunReconciliationRequest,
  SignupRequest,
  SignupResponse,
  StepUpChallenge,
  StepUpChallengeRequest,
  StepUpToken,
  StepUpVerifyRequest,
  UpdateAccountRequest,
  UpdateMemberRequest,
  VerifyEmailRequest,
} from '../types';
import { CURRENCIES } from '../types';
import { buildAccountCvu, buildMockCvu, buildPayin, buildReconRun, createSeed, DEMO_USER_ID, DIRECTORY } from './fixtures';
import { payoutFee } from './feePlan';
import { buildMovements } from './movements';
import { companyView, linkMatchingPayers, payinsOf } from './myPayments';
import type { MockAccount, MockPayer, MockPayerLink, MockPayout, MockState } from './state';

/** The mock "server": business rules mirror the API so the UI meets the same error codes as live. */
export const MOCK_EMAIL_CODE = '123456';
const KYC_REJECTED_DNIS = ['11111111'];
const KYC_DECISION_MS = 8_000;
const ERP_IMPORT_MS = 6_000;
const PAGE_SIZE = 20;
const PSP_PREFIX = '0031';
/** Bank name the API reports for a Lenapay CVU or alias (on-us destination). */
const LENAPAY_BANK = 'Lenapay';
/** The API's cap on what the person types as the concept of a "Mis pagos" payment. */
const MAX_PAY_CONCEPT = 120;
/** Contract cap on payout attachments (`payout.attachment_invalid` past it). */
const MAX_ATTACHMENTS = 10;
const ATTACHMENT_TYPES: readonly PayoutAttachment['contentType'][] = ['application/pdf', 'image/jpeg', 'image/png'];
const IDENTITY_DOCS: readonly KycDocument['kind'][] = ['dni_front', 'dni_back', 'selfie'];

/** One live payin per tick, rotating payers and amounts (deterministic: no Math.random). */
const LIVE_AMOUNTS: { currency: Currency; minor: number }[] = [
  { currency: 'ARS', minor: 150_000_00 },
  { currency: 'ARS', minor: 85_000_00 },
  { currency: 'USD', minor: 2_500_00 },
  { currency: 'ARS', minor: 320_000_00 },
  { currency: 'ARS', minor: 1_200_000_00 },
  { currency: 'USD', minor: 750_00 },
];

interface MockClock {
  now(): number;
  setTimeout(callback: () => void, ms: number): unknown;
}

export class MockBackend {
  state: MockState;
  private readonly clock: MockClock;
  private readonly listeners = new Map<string, Set<(event: LiveEvent) => void>>();
  private liveTick = 0;

  constructor(clock: MockClock) {
    this.clock = clock;
    this.state = createSeed(clock.now());
  }

  private nextId(prefix: string): string {
    this.state.seq += 1;
    return `${prefix}_${this.state.seq}`;
  }

  private iso(offsetMs = 0): string {
    return new Date(this.clock.now() + offsetMs).toISOString();
  }

  private tokens(): AuthTokens {
    return { accessToken: this.nextId('at'), accessTokenExpiresAt: this.iso(900_000), refreshToken: this.nextId('rt'), refreshTokenExpiresAt: this.iso(30 * 86_400_000), tokenType: 'Bearer' };
  }

  // ── Auth ───────────────────────────────────────────────────────────────────────────────
  signup(body: SignupRequest): SignupResponse {
    const email = body.email.trim().toLowerCase();
    if (this.state.users.some((user) => user.email === email)) throw new ApiError('auth.email_taken', 409);
    this.state.pendingSignup = { email };
    return { userId: 'pending' };
  }

  /** Like the API: the email is verified (204) and the person then signs in. */
  verifyEmail(body: VerifyEmailRequest): void {
    const pending = this.state.pendingSignup;
    if (!pending || pending.email !== body.email.trim().toLowerCase() || body.code !== MOCK_EMAIL_CODE) {
      throw new ApiError('auth.code_invalid', 422);
    }
    const userId = this.nextId('u');
    this.state.users.push({ id: userId, email: pending.email, fullName: null, mfaEnabled: false });
    this.state.pendingSignup = null;
    this.openPersonalWallet(userId, pending.email);
  }

  /** Signup opens the person's wallet (R-ACC-5): personal org in KYC draft + its personal account. */
  private openPersonalWallet(userId: string, email: string): void {
    const id = this.insertOrg(userId, { kind: 'personal', legalName: email });
    this.state.accounts.push({ id: this.nextId('acc'), orgId: id, kind: 'personal', name: 'Mi cuenta', archived: false, createdAt: this.iso(), ledger: emptyLedger(), cvus: [] });
  }

  login(body: LoginRequest): AuthTokens {
    if (body.password.length < 6) throw new ApiError('auth.invalid_credentials', 401);
    const email = body.email.trim().toLowerCase();
    this.state.currentUserId = this.state.users.find((user) => user.email === email)?.id ?? DEMO_USER_ID;
    return this.tokens();
  }

  /** In-memory state resets on reload, so a stored refresh token signs the demo user back in. */
  refresh(): AuthTokens {
    this.state.currentUserId ??= DEMO_USER_ID;
    return this.tokens();
  }

  logout(): void {
    this.state.currentUserId = null;
  }

  registerDevice(body: RegisterDeviceRequest): Device {
    const userId = this.requireUserId();
    const device = { id: this.nextId('dev'), userId, publicKey: body.publicKey };
    this.state.devices.push(device);
    return { id: device.id, name: body.name, createdAt: this.iso() };
  }

  stepUpChallenge(body: StepUpChallengeRequest): StepUpChallenge {
    const userId = this.requireUserId();
    if (body.deviceId && !this.state.devices.some((item) => item.id === body.deviceId && item.userId === userId)) throw new ApiError('device.not_found', 404);
    // The nonce only has to be valid base64url: the mock does not verify signatures.
    return { challengeId: this.nextId('ch'), nonce: `bm9uY2U${this.state.seq}`.padEnd(12, 'A'), expiresAt: this.iso(120_000) };
  }

  stepUpVerify(body: StepUpVerifyRequest): StepUpToken {
    const userId = this.requireUserId();
    if (body.method === 'password' && (body.password ?? '').length < 6) throw new ApiError('auth.invalid_credentials', 401);
    if (body.method === 'device_signature' && (!body.signature || !this.state.devices.some((item) => item.id === body.deviceId && item.userId === userId))) {
      throw new ApiError('auth.challenge_invalid', 422);
    }
    return { stepUpToken: this.nextId('su'), expiresAt: this.iso(300_000) };
  }

  me(): Me {
    const user = this.requireUser();
    const memberships = this.state.memberships
      .filter((item) => item.userId === user.id)
      .flatMap((item) => {
        const org = this.state.orgs.find((candidate) => candidate.id === item.orgId);
        return org ? [{ orgId: org.id, orgName: org.legalName, orgKind: org.kind, role: item.role, kycStatus: org.kycStatus }] : [];
      });
    // Like the live mapper: personal mode follows owning a personal wallet.
    const personalModeEnabled = memberships.some((item) => item.orgKind === 'personal');
    return { id: user.id, email: user.email, fullName: user.fullName, mfaEnabled: user.mfaEnabled, personalModeEnabled, memberships };
  }

  private requireUserId(): string {
    if (!this.state.currentUserId) throw new ApiError('auth.session_expired', 401);
    return this.state.currentUserId;
  }

  private requireUser() {
    const id = this.requireUserId();
    const user = this.state.users.find((item) => item.id === id);
    if (!user) throw new ApiError('auth.session_expired', 401);
    return user;
  }

  private personRef(userId: string) {
    return { userId, label: this.state.users.find((item) => item.id === userId)?.fullName ?? null };
  }

  // ── Orgs and team ──────────────────────────────────────────────────────────────────────
  private orgFor(orgId: string): Org {
    this.settleAsyncWork(orgId);
    const userId = this.requireUserId();
    const membership = this.state.memberships.find((item) => item.orgId === orgId && item.userId === userId);
    const org = this.state.orgs.find((item) => item.id === orgId);
    // Someone else's org is "not found", never "forbidden" (A7).
    if (!membership || !org) throw new ApiError('org.not_found', 404);
    return { ...org, role: membership.role };
  }

  private requireMoneyAction(orgId: string): Org {
    const org = this.orgFor(orgId);
    const blocker = moneyActionBlocker(org);
    if (blocker === 'kyc') throw new ApiError('kyc.not_approved', 403);
    if (blocker === 'role') throw new ApiError('auth.forbidden', 403);
    return org;
  }

  private requireAdmin(orgId: string): Org {
    const org = this.orgFor(orgId);
    if (org.role !== 'admin') throw new ApiError('auth.forbidden', 403);
    return org;
  }

  listOrgs(): Org[] {
    const userId = this.requireUserId();
    return this.state.memberships.filter((item) => item.userId === userId).map((item) => this.orgFor(item.orgId));
  }

  createOrg(body: CreateOrgRequest): Org {
    const userId = this.requireUserId();
    // Signup already opened the personal wallet: a second one is a conflict, like the API.
    const hasPersonal = this.state.memberships.some((item) => item.userId === userId && this.state.orgs.find((org) => org.id === item.orgId)?.kind === 'personal');
    if (body.kind === 'personal' && hasPersonal) throw new ApiError('resource.conflict', 409);
    return this.orgFor(this.insertOrg(userId, body));
  }

  private insertOrg(userId: string, body: CreateOrgRequest): string {
    const id = this.nextId('org');
    this.state.orgs.push({ id, kind: body.kind, legalName: body.legalName.trim(), cuit: null, kycStatus: 'draft', createdAt: this.iso() });
    this.state.memberships.push({ orgId: id, userId, role: 'admin' });
    this.state.kyc[id] = {
      id: `kyc_${id}`, orgId: id, kind: body.kind === 'company' ? 'company' : 'person', status: 'draft',
      documents: [], decisionReason: null, submittedAt: null, decidedAt: null, updatedAt: this.iso(),
    };
    this.state.erp[id] = { candidate: { found: false, erpOrgId: null, displayName: null }, projects: [], payers: [], linkStartedAt: null, linkedAt: null };
    return id;
  }

  members(orgId: string): Member[] {
    this.orgFor(orgId);
    const active: Member[] = this.state.memberships
      .filter((item) => item.orgId === orgId)
      .flatMap((item) => {
        const user = this.state.users.find((candidate) => candidate.id === item.userId);
        return user ? [{ userId: user.id, fullName: user.fullName, email: user.email, role: item.role, status: 'active' as const }] : [];
      });
    const invited: Member[] = this.state.invites
      .filter((item) => item.orgId === orgId)
      .map((item) => ({ userId: item.id, fullName: null, email: item.email, role: item.role, status: 'invited' }));
    return [...active, ...invited];
  }

  invite(orgId: string, body: InviteRequest): Invite {
    this.requireAdmin(orgId);
    const invite = { id: this.nextId('inv'), orgId, email: body.email.trim().toLowerCase(), role: body.role, expiresAt: this.iso(7 * 86_400_000), createdAt: this.iso() };
    this.state.invites.push(invite);
    const { orgId: _orgId, ...dto } = invite;
    return dto;
  }

  private membership(orgId: string, userId: string) {
    const found = this.state.memberships.find((item) => item.orgId === orgId && item.userId === userId);
    if (!found) throw new ApiError('member.not_found', 404);
    return found;
  }

  private isLastAdmin(orgId: string, userId: string): boolean {
    const admins = this.state.memberships.filter((item) => item.orgId === orgId && item.role === 'admin');
    return admins.length === 1 && admins[0]?.userId === userId;
  }

  updateMember(orgId: string, userId: string, body: UpdateMemberRequest): Member {
    this.requireAdmin(orgId);
    const membership = this.membership(orgId, userId);
    if (membership.role === 'admin' && body.role !== 'admin' && this.isLastAdmin(orgId, userId)) throw new ApiError('org.last_admin', 409);
    membership.role = body.role;
    const member = this.members(orgId).find((item) => item.userId === userId);
    if (!member) throw new ApiError('member.not_found', 404);
    return member;
  }

  removeMember(orgId: string, userId: string): void {
    this.requireAdmin(orgId);
    this.membership(orgId, userId);
    if (this.isLastAdmin(orgId, userId)) throw new ApiError('org.last_admin', 409);
    this.state.memberships = this.state.memberships.filter((item) => !(item.orgId === orgId && item.userId === userId));
  }

  // ── KYC ────────────────────────────────────────────────────────────────────────────────
  private kycCase(orgId: string): KycCase {
    this.orgFor(orgId);
    const found = this.state.kyc[orgId];
    if (!found) throw new ApiError('kyc.case_not_found', 404);
    return found;
  }

  getKyc(orgId: string): KycCase {
    return this.kycCase(orgId);
  }

  /** PUT replaces the case data; a rejected case goes back to draft (the API's state machine). */
  updateKyc(orgId: string, body: KycUpdateRequest): KycCase {
    const current = this.kycCase(orgId);
    if (current.status === 'submitted' || current.status === 'approved') throw new ApiError('kyc.invalid_state', 409);
    const data = current.kind === 'company' ? body.company : body.person;
    if (!data) throw new ApiError('validation.failed', 422);
    const next: KycCase = { ...current, ...(current.kind === 'company' ? { company: body.company } : { person: body.person }), status: 'draft', updatedAt: this.iso() };
    this.state.kyc[orgId] = next;
    return next;
  }

  uploadKycDocument(orgId: string, body: KycDocumentUpload): KycDocument {
    const current = this.kycCase(orgId);
    if (current.status === 'submitted' || current.status === 'approved') throw new ApiError('kyc.invalid_state', 409);
    const isIdentity = IDENTITY_DOCS.includes(body.kind);
    const dni = body.representativeDni ?? null;
    const misplaced = current.kind === 'person' ? dni !== null : isIdentity !== (dni !== null);
    if (misplaced) throw new ApiError('validation.failed', 422);
    const document: KycDocument = {
      id: this.nextId('doc'), kind: body.kind, representativeDni: dni, fileName: body.fileName, contentType: body.mimeType, sizeBytes: 120_000, createdAt: this.iso(),
    };
    const others = current.documents.filter((item) => !(item.kind === body.kind && item.representativeDni === dni));
    this.state.kyc[orgId] = { ...current, documents: [...others, document] };
    return document;
  }

  submitKyc(orgId: string): KycCase {
    const current = this.kycCase(orgId);
    if (current.status !== 'draft' && current.status !== 'rejected') throw new ApiError('kyc.invalid_state', 409);
    if (!isKycComplete(current)) throw new ApiError('kyc.incomplete', 422);
    const next: KycCase = { ...current, status: 'submitted', decisionReason: null, submittedAt: this.iso(), decidedAt: null, updatedAt: this.iso() };
    this.state.kyc[orgId] = next;
    this.state.kycSubmittedAt[orgId] = this.clock.now();
    this.setOrgKyc(orgId, 'submitted');
    return next;
  }

  private setOrgKyc(orgId: string, status: KycCase['status']): void {
    const org = this.state.orgs.find((item) => item.id === orgId);
    if (org) org.kycStatus = status;
  }

  /** The fake KYC provider decides a few seconds after submit (R-KYC-2), lazily on read. */
  private settleAsyncWork(orgId: string): void {
    const submittedAt = this.state.kycSubmittedAt[orgId];
    const current = this.state.kyc[orgId];
    if (submittedAt !== undefined && current?.status === 'submitted' && this.clock.now() - submittedAt >= KYC_DECISION_MS) {
      this.decideKyc(orgId, current);
    }
    this.advanceErpImport(orgId);
  }

  private decideKyc(orgId: string, current: KycCase): void {
    const dnis = [current.person?.dni, ...(current.company?.representatives ?? []).map((item) => item.dni)];
    const rejected = dnis.some((dni) => dni !== undefined && KYC_REJECTED_DNIS.includes(dni));
    const status = rejected ? 'rejected' : 'approved';
    this.state.kyc[orgId] = { ...current, status, decisionReason: rejected ? 'No pudimos validar la identidad del DNI informado.' : null, decidedAt: this.iso(), updatedAt: this.iso() };
    this.setOrgKyc(orgId, status);
    const org = this.state.orgs.find((item) => item.id === orgId);
    if (status === 'approved' && org && current.company) org.cuit = current.company.cuit;
    if (status === 'approved' && !this.state.accounts.some((item) => item.orgId === orgId)) {
      const kind = current.kind === 'company' ? 'main' : 'personal';
      this.state.accounts.push({ id: this.nextId('acc'), orgId, kind, name: kind === 'main' ? 'Cuenta principal' : 'Mi cuenta', archived: false, createdAt: this.iso(), ledger: emptyLedger(), cvus: [] });
    }
    if (status === 'approved' && current.person) this.approvePerson(orgId, current.person.dni, current.person.fullName);
  }

  /** Person approved (R-MP-1, R-CVU-4): the DNI is now verified and the wallet gets its ARS + USD CVUs. */
  private approvePerson(orgId: string, dni: string, fullName: string): void {
    this.state.verifiedDni[orgId] = dni;
    const personal = this.state.accounts.find((item) => item.orgId === orgId && item.kind === 'personal');
    if (!personal || personal.cvus.length > 0) return;
    personal.cvus = CURRENCIES.map((currency) =>
      buildAccountCvu({ id: this.nextId('acvu'), currency, cvu: buildCvu(PSP_PREFIX, String(4_000_000_000_000 + this.state.seq)), alias: this.uniqueAlias(fullName, currency) }),
    );
  }

  // ── Accounts ───────────────────────────────────────────────────────────────────────────
  private account(orgId: string, accountId: string): MockAccount {
    const found = this.state.accounts.find((item) => item.orgId === orgId && item.id === accountId);
    if (!found) throw new ApiError('account.not_found', 404);
    return found;
  }

  private toAccount(account: MockAccount): Account {
    return {
      id: account.id,
      kind: account.kind,
      name: account.name,
      archived: account.archived,
      erpProjectRef: null,
      balances: CURRENCIES.map((currency) => toBalance(currency, account.ledger[currency])),
      payersCount: this.state.payers.filter((item) => item.accountId === account.id).length,
      cvus: account.cvus.map((cvu) => ({ ...cvu })),
      createdAt: account.createdAt,
    };
  }

  private accountName(accountId: string): string {
    return this.state.accounts.find((item) => item.id === accountId)?.name ?? '';
  }

  listAccounts(orgId: string): Account[] {
    this.orgFor(orgId);
    return this.state.accounts.filter((item) => item.orgId === orgId).map((item) => this.toAccount(item));
  }

  balances(orgId: string): OrgBalances {
    this.orgFor(orgId);
    const open = this.state.accounts.filter((item) => item.orgId === orgId && !item.archived);
    return {
      totals: CURRENCIES.map((currency) =>
        toBalance(currency, open.reduce((sum, item) => ({ posted: sum.posted + item.ledger[currency].posted, held: sum.held + item.ledger[currency].held }), { posted: 0, held: 0 })),
      ),
    };
  }

  createAccount(orgId: string, body: CreateAccountRequest): Account {
    this.requireMoneyAction(orgId);
    const account: MockAccount = { id: this.nextId('acc'), orgId, kind: 'project', name: body.name.trim(), archived: false, createdAt: this.iso(), ledger: emptyLedger(), cvus: [] };
    this.state.accounts.push(account);
    return this.toAccount(account);
  }

  updateAccount(orgId: string, accountId: string, body: UpdateAccountRequest): Account {
    this.requireMoneyAction(orgId);
    const account = this.account(orgId, accountId);
    if (body.archived) {
      if (account.kind !== 'project') throw new ApiError('account.not_archivable', 409);
      const hasMoney = CURRENCIES.some((currency) => account.ledger[currency].posted !== 0 || account.ledger[currency].held !== 0);
      if (hasMoney) throw new ApiError('account.not_empty', 409);
      account.archived = true;
    }
    if (body.archived === false) account.archived = false;
    if (body.name !== undefined) account.name = body.name.trim();
    return this.toAccount(account);
  }

  transfer(orgId: string, body: InternalTransferRequest, credentials: MoneyActionOptions): InternalTransfer {
    return this.idempotent(`transfer:${orgId}`, credentials, body, () => {
      this.requireMoneyAction(orgId);
      requireStepUp(credentials);
      if (body.fromAccountId === body.toAccountId) throw new ApiError('transfer.same_account', 422);
      if (!Number.isSafeInteger(body.amount) || body.amount <= 0) throw new ApiError('money.invalid_amount', 422);
      const from = this.account(orgId, body.fromAccountId);
      const to = this.account(orgId, body.toAccountId);
      if (from.archived || to.archived) throw new ApiError('account.archived', 409);
      const balance = from.ledger[body.currency];
      if (balance.posted - balance.held < body.amount) throw new ApiError('ledger.insufficient_funds', 422);
      balance.posted -= body.amount;
      to.ledger[body.currency].posted += body.amount;
      const transfer = { id: this.nextId('tr'), orgId, fromAccountId: from.id, toAccountId: to.id, currency: body.currency, amount: body.amount, concept: body.concept, createdAt: this.iso() };
      this.state.transfers.push(transfer);
      this.emit(orgId, { type: 'balance.changed', accountId: from.id });
      return { id: transfer.id, fromAccountId: transfer.fromAccountId, toAccountId: transfer.toAccountId, currency: transfer.currency, amount: transfer.amount, concept: transfer.concept, createdAt: transfer.createdAt };
    });
  }

  /** Alias of the account's own CVU (R-CVU-4); a payer's or another org's CVU is not found. */
  updateAccountCvu(orgId: string, accountId: string, cvuId: string, alias: string): AccountCvu {
    this.orgFor(orgId);
    const cvu = this.account(orgId, accountId).cvus.find((item) => item.id === cvuId);
    if (!cvu) throw new ApiError('cvu.not_found', 404);
    const next = alias.trim().toLowerCase();
    if (classifyDestination(next) !== 'alias') throw new ApiError('validation.failed', 422);
    if (next !== cvu.alias && this.aliasTaken(next)) throw new ApiError('cvu.alias_taken', 409);
    cvu.alias = next;
    return { ...cvu };
  }

  /** Same key + same payload replays the stored answer; same key + other payload is 409 (R-OUT-3). */
  private idempotent<T>(scope: string, credentials: MoneyActionOptions, payload: unknown, effect: () => T): T {
    if (!credentials.idempotencyKey) throw new ApiError('validation.failed', 422);
    const storeKey = `${scope}:${credentials.idempotencyKey}`;
    const hash = JSON.stringify(payload);
    const stored = this.state.idempotency[storeKey];
    if (stored) {
      if (stored.hash !== hash) throw new ApiError('idempotency.key_reused', 409);
      return stored.response as T;
    }
    const response = effect();
    this.state.idempotency[storeKey] = { hash, response };
    return response;
  }

  // ── Payers and CVUs ────────────────────────────────────────────────────────────────────
  private toPayer(record: MockPayer): Payer {
    const { orgId: _orgId, ...rest } = record;
    return { ...rest, accountName: this.accountName(record.accountId) };
  }

  private payer(orgId: string, payerId: string): MockPayer {
    const found = this.state.payers.find((item) => item.orgId === orgId && item.id === payerId);
    if (!found) throw new ApiError('payer.not_found', 404);
    return found;
  }

  listPayers(orgId: string, query: PayerQuery): Payer[] {
    this.orgFor(orgId);
    const needle = normalize(query.search ?? '');
    return this.state.payers
      .filter((item) => item.orgId === orgId && (!query.account || item.accountId === query.account))
      .filter((item) => needle === '' || [item.name, item.docNumber, ...item.cvus.map((cvu) => cvu.alias ?? '')].some((field) => normalize(field).includes(needle)))
      .sort((a, b) => a.name.localeCompare(b.name, 'es'))
      .map((item) => this.toPayer(item));
  }

  getPayer(orgId: string, payerId: string): Payer {
    this.orgFor(orgId);
    return this.toPayer(this.payer(orgId, payerId));
  }

  createPayer(orgId: string, body: CreatePayerRequest): Payer {
    this.requireMoneyAction(orgId);
    const account = this.account(orgId, body.accountId);
    if (account.archived) throw new ApiError('account.archived', 409);
    if (body.docType !== 'dni' && !isValidCuit(body.docNumber)) throw new ApiError('validation.failed', 422);
    const record: MockPayer = {
      id: this.nextId('pay'), orgId, accountId: account.id, name: body.name.trim(), docType: body.docType, docNumber: body.docNumber,
      email: body.email?.trim() || null, erpRef: null, cvus: [], createdAt: this.iso(),
    };
    this.state.payers.push(record);
    return this.toPayer(record);
  }

  issueCvu(orgId: string, payerId: string, currency: Currency, credentials: MoneyActionOptions): Cvu {
    return this.idempotent(`cvu:${orgId}:${payerId}`, credentials, { currency }, () => {
      this.requireMoneyAction(orgId);
      requireStepUp(credentials);
      const record = this.payer(orgId, payerId);
      if (record.cvus.some((item) => item.currency === currency)) throw new ApiError('cvu.already_exists', 409);
      const cvu = buildMockCvu({
        id: this.nextId('cvu'), payerId, accountId: record.accountId, currency, cvu: buildCvu(PSP_PREFIX, String(2_000_000_000_000 + this.state.seq)),
        alias: this.uniqueAlias(record.name, currency), createdAt: this.iso(),
      });
      record.cvus.push(cvu);
      return cvu;
    });
  }

  private uniqueAlias(name: string, currency: Currency): string {
    const base = normalize(name).replace(/[^a-z0-9]+/g, '.').replace(/^\.|\.$/g, '').slice(0, 12);
    const suffix = currency === 'USD' ? '.usd' : '.lnp';
    let alias = `${base}${suffix}`;
    for (let n = 2; this.aliasTaken(alias); n += 1) alias = `${base}${n}${suffix}`;
    return alias;
  }

  private aliasTaken(alias: string): boolean {
    const taken = (cvus: { alias: string | null }[]) => cvus.some((cvu) => cvu.alias === alias);
    return this.state.payers.some((item) => taken(item.cvus)) || this.state.accounts.some((item) => taken(item.cvus));
  }

  private cvu(orgId: string, cvuId: string): Cvu {
    const found = this.state.payers.filter((item) => item.orgId === orgId).flatMap((item) => item.cvus).find((item) => item.id === cvuId);
    if (!found) throw new ApiError('cvu.not_found', 404);
    return found;
  }

  updateCvuAlias(orgId: string, cvuId: string, alias: string): Cvu {
    this.requireMoneyAction(orgId);
    const cvu = this.cvu(orgId, cvuId);
    const next = alias.trim().toLowerCase();
    if (classifyDestination(next) !== 'alias') throw new ApiError('validation.failed', 422);
    if (next !== cvu.alias && this.aliasTaken(next)) throw new ApiError('cvu.alias_taken', 409);
    cvu.alias = next;
    return cvu;
  }

  disableCvu(orgId: string, cvuId: string): void {
    this.requireMoneyAction(orgId);
    this.cvu(orgId, cvuId).status = 'disabled';
  }

  getPayin(orgId: string, payinId: string): Payin {
    this.orgFor(orgId);
    const found = this.state.payins.find((item) => item.orgId === orgId && item.id === payinId);
    if (!found) throw new ApiError('payin.not_found', 404);
    const { orgId: _orgId, ...payin } = found;
    return payin;
  }

  // ── Payouts and approvals ──────────────────────────────────────────────────────────────
  resolveDestination(orgId: string, to: string): Destination {
    this.orgFor(orgId);
    const value = to.trim().toLowerCase();
    const kind = classifyDestination(value);
    if (kind === 'invalid' || kind === 'incomplete') throw new ApiError('destination.invalid', 422);
    const known = DIRECTORY.find((item) => item.accountNumber === value || item.value === value);
    if (known) return { ...known, type: kind, value };
    const onUs = this.lenapayCvu(value);
    if (onUs) return { type: kind, value, accountNumber: onUs.cvu, holderName: onUs.holderName, holderCuit: onUs.holderCuit, bankName: LENAPAY_BANK };
    throw new ApiError('destination.not_found', 404);
  }

  /** A Lenapay CVU or alias (a payer's or a personal wallet's): money to it never reaches the bank (R-MP-5). */
  private lenapayCvu(value: string): { cvu: string; holderName: string; holderCuit: string; credit: { accountId: string; payerId: string | null; cvuId: string } } | null {
    const matches = (cvu: { cvu: string; alias: string | null; status: string }) => cvu.status === 'active' && (cvu.cvu === value || cvu.alias === value);
    for (const payer of this.state.payers) {
      const cvu = payer.cvus.find(matches);
      if (cvu) return { cvu: cvu.cvu, holderName: payer.name, holderCuit: payer.docNumber, credit: { accountId: payer.accountId, payerId: payer.id, cvuId: cvu.id } };
    }
    for (const account of this.state.accounts) {
      const cvu = account.cvus.find(matches);
      const org = this.state.orgs.find((item) => item.id === account.orgId);
      if (cvu && org) return { cvu: cvu.cvu, holderName: org.legalName, holderCuit: org.cuit ?? '', credit: { accountId: account.id, payerId: null, cvuId: cvu.id } };
    }
    return null;
  }

  private railFor(to: string): Rail {
    return this.lenapayCvu(to.trim().toLowerCase()) ? 'internal' : 'bank';
  }

  quote(orgId: string, body: PayoutQuoteRequest): PayoutQuote {
    this.orgFor(orgId);
    this.resolveDestination(orgId, body.to);
    const account = this.account(orgId, body.accountId);
    const rail = this.railFor(body.to);
    const fee = payoutFee(body.amount, body.currency, rail);
    const balance = account.ledger[body.currency];
    const total = body.amount + fee;
    return { currency: body.currency, amount: body.amount, fee, total, availableAfter: balance.posted - balance.held - total, rail };
  }

  createPayout(orgId: string, body: CreatePayoutRequest, credentials: MoneyActionOptions): Payout {
    return this.idempotent(`payout:${orgId}`, credentials, body, () => {
      const org = this.requireMoneyAction(orgId);
      requireStepUp(credentials);
      if (!Number.isSafeInteger(body.amount) || body.amount <= 0) throw new ApiError('money.invalid_amount', 422);
      const destination = this.resolveDestination(orgId, body.to);
      const account = this.account(orgId, body.accountId);
      if (account.archived) throw new ApiError('account.archived', 409);
      const rail = this.railFor(body.to);
      const fee = payoutFee(body.amount, body.currency, rail);
      const balance = account.ledger[body.currency];
      if (balance.posted - balance.held < body.amount + fee) throw new ApiError('ledger.insufficient_funds', 422);
      balance.held += body.amount + fee;
      const user = this.requireUser();
      const status: PayoutStatus = org.kind === 'personal' ? 'approved' : 'pending_approval';
      const payout: MockPayout = {
        id: this.nextId('po'), orgId, accountId: account.id, accountName: account.name, currency: body.currency, amount: body.amount, fee, total: body.amount + fee,
        destination, concept: body.concept.trim(), attachments: [], status, rail, createdBy: this.personRef(user.id), approvedBy: null, rejectReason: null,
        failureCode: null, createdAt: this.iso(), settledAt: null,
        events: [{ status: 'pending_approval', at: this.iso() }, ...(status === 'approved' ? [{ status, at: this.iso() }] : [])],
      };
      this.state.payouts.push(payout);
      if (status === 'approved') this.dispatch(payout);
      else this.emit(orgId, { type: 'approval.requested', payout: toPayout(payout) });
      return toPayout(payout);
    });
  }

  private payout(orgId: string, payoutId: string): MockPayout {
    const found = this.state.payouts.find((item) => item.orgId === orgId && item.id === payoutId);
    if (!found) throw new ApiError('payout.not_found', 404);
    return found;
  }

  getPayout(orgId: string, payoutId: string): Payout {
    this.orgFor(orgId);
    return toPayout(this.payout(orgId, payoutId));
  }

  approvePayout(orgId: string, payoutId: string, credentials: MoneyActionOptions): Payout {
    return this.idempotent(`approve:${orgId}:${payoutId}`, credentials, { payoutId }, () => {
      const org = this.orgFor(orgId);
      requireStepUp(credentials);
      const payout = this.payout(orgId, payoutId);
      const blocker = approvalBlocker({ payout, userId: this.requireUserId(), role: org.role });
      if (blocker === 'status') throw new ApiError('payout.invalid_state', 409);
      if (blocker === 'creator') throw new ApiError('payout.self_approval_forbidden', 403);
      if (blocker === 'role') throw new ApiError('auth.forbidden', 403);
      payout.approvedBy = this.personRef(this.requireUserId());
      this.transition(payout, 'approved');
      this.dispatch(payout);
      return toPayout(payout);
    });
  }

  rejectPayout(orgId: string, payoutId: string, reason: string, credentials: MoneyActionOptions): Payout {
    return this.idempotent(`reject:${orgId}:${payoutId}`, credentials, { payoutId, reason }, () => {
      const org = this.orgFor(orgId);
      requireStepUp(credentials);
      const payout = this.payout(orgId, payoutId);
      const blocker = approvalBlocker({ payout, userId: this.requireUserId(), role: org.role });
      if (blocker === 'status') throw new ApiError('payout.invalid_state', 409);
      if (blocker === 'role') throw new ApiError('auth.forbidden', 403);
      payout.rejectReason = reason.trim() || null;
      this.releaseHold(payout);
      this.transition(payout, 'rejected');
      return toPayout(payout);
    });
  }

  listApprovals(orgId: string): Payout[] {
    this.orgFor(orgId);
    return this.state.payouts
      .filter((item) => item.orgId === orgId && item.status === 'pending_approval')
      .sort((a, b) => b.createdAt.localeCompare(a.createdAt))
      .map(toPayout);
  }

  uploadAttachment(orgId: string, payoutId: string, file: LocalUpload): PayoutAttachment {
    this.orgFor(orgId);
    const payout = this.payout(orgId, payoutId);
    const contentType = ATTACHMENT_TYPES.find((type) => type === file.mimeType);
    if (!contentType || payout.attachments.length >= MAX_ATTACHMENTS) throw new ApiError('payout.attachment_invalid', 422);
    const attachment: PayoutAttachment = { id: this.nextId('att'), fileName: file.fileName, contentType, sizeBytes: 120_000, uploadedBy: this.requireUserId(), createdAt: this.iso() };
    payout.attachments = [...payout.attachments, attachment];
    return attachment;
  }

  private transition(payout: MockPayout, status: PayoutStatus): void {
    payout.status = status;
    if (status === 'settled') payout.settledAt = this.iso();
    payout.events = [...payout.events, { status, at: this.iso() }];
    this.emit(payout.orgId, { type: 'payout.updated', payout: toPayout(payout) });
  }

  private releaseHold(payout: MockPayout): void {
    const account = this.state.accounts.find((item) => item.id === payout.accountId);
    if (account) account.ledger[payout.currency].held -= payout.total;
  }

  /** The PSP dispatch runs off the request (R-OUT-5): sending, then settled. On-us settles at once. */
  private dispatch(payout: MockPayout): void {
    if (payout.rail === 'internal') {
      this.settleOnUs(payout);
      return;
    }
    this.clock.setTimeout(() => this.transition(payout, 'sending'), 1_500);
    this.clock.setTimeout(() => {
      const account = this.state.accounts.find((item) => item.id === payout.accountId);
      if (account) {
        account.ledger[payout.currency].held -= payout.total;
        account.ledger[payout.currency].posted -= payout.total;
      }
      this.transition(payout, 'settled');
      this.emit(payout.orgId, { type: 'balance.changed', accountId: payout.accountId });
    }, 4_000);
  }

  /** On-us (R-MP-5): ONE entry, payer −total → receiver +amount, and the receiver sees a credited payin. */
  private settleOnUs(payout: MockPayout): void {
    const receiver = this.lenapayCvu(payout.destination.accountNumber);
    const from = this.state.accounts.find((item) => item.id === payout.accountId);
    const to = receiver ? this.state.accounts.find((item) => item.id === receiver.credit.accountId) : undefined;
    if (!receiver || !from || !to) {
      this.releaseHold(payout);
      payout.failureCode = 'destination_unavailable';
      this.transition(payout, 'failed');
      return;
    }
    from.ledger[payout.currency].held -= payout.total;
    from.ledger[payout.currency].posted -= payout.total;
    to.ledger[payout.currency].posted += payout.amount;
    this.transition(payout, 'settled');
    this.emit(payout.orgId, { type: 'balance.changed', accountId: from.id });
    const origin = this.state.orgs.find((item) => item.id === payout.orgId);
    const payin = {
      id: this.nextId('pin'), orgId: to.orgId, accountId: to.id, accountName: to.name, payerId: receiver.credit.payerId, cvuId: receiver.credit.cvuId, currency: payout.currency,
      amount: payout.amount, fee: 0, net: payout.amount, originName: origin?.legalName ?? null, originCuit: origin?.cuit ?? null, createdAt: this.iso(), rail: 'internal' as const,
    };
    this.state.payins.push(payin);
    const { orgId: receiverOrgId, rail: _rail, ...dto } = payin;
    this.emit(receiverOrgId, { type: 'payin.credited', payin: dto });
    this.emit(receiverOrgId, { type: 'balance.changed', accountId: to.id });
  }

  // ── Mis pagos (payer view) ─────────────────────────────────────────────────────────────
  /** The caller's own personal org with a verified DNI, else `kyc.not_approved` (403). */
  private verifiedPersonal(): { orgId: string; dni: string } {
    const userId = this.requireUserId();
    for (const membership of this.state.memberships.filter((item) => item.userId === userId)) {
      const org = this.state.orgs.find((item) => item.id === membership.orgId);
      const dni = this.state.verifiedDni[membership.orgId];
      if (org?.kind === 'personal' && org.kycStatus === 'approved' && dni) return { orgId: org.id, dni };
    }
    throw new ApiError('kyc.not_approved', 403);
  }

  /** Another person's link answers exactly like a missing one (T2). */
  private payerLink(linkId: string): MockPayerLink {
    const { orgId } = this.verifiedPersonal();
    const link = this.state.payerLinks.find((item) => item.id === linkId && item.personalOrgId === orgId);
    if (!link) throw new ApiError('payer_link.not_found', 404);
    return link;
  }

  myPaymentCompanies(includeHidden: boolean): MyPaymentCompany[] {
    const { orgId, dni } = this.verifiedPersonal();
    linkMatchingPayers(this.state, orgId, dni, () => this.nextId('lnk'));
    return this.state.payerLinks
      .filter((item) => item.personalOrgId === orgId && (includeHidden || !item.hidden))
      .map((item) => companyView(this.state, item))
      .reverse();
  }

  setMyPaymentHidden(linkId: string, hidden: boolean): MyPaymentCompany {
    const link = this.payerLink(linkId);
    link.hidden = hidden;
    return companyView(this.state, link);
  }

  myPaymentPayins(linkId: string, cursor: string | null): Page<MyPaymentPayin> {
    const all = payinsOf(this.state, this.payerLink(linkId));
    const offset = cursor ? Number(cursor) : 0;
    return { items: all.slice(offset, offset + PAGE_SIZE), nextCursor: offset + PAGE_SIZE < all.length ? String(offset + PAGE_SIZE) : null };
  }

  myPaymentInstallments(linkId: string): InstallmentList {
    const link = this.payerLink(linkId);
    const payer = this.state.payers.find((item) => item.id === link.payerId);
    if (!payer || !this.state.erp[payer.orgId]?.linkedAt) return { erpLinked: false, items: [] };
    return { erpLinked: true, items: (this.state.installments[payer.id] ?? []).map((item) => ({ ...item })) };
  }

  /** Pays the company from the personal account to the person's CVU there (R-MP-4); no approval. */
  payMyPaymentCompany(linkId: string, body: PayCompanyRequest, credentials: MoneyActionOptions): Payout {
    const link = this.payerLink(linkId);
    const payer = this.state.payers.find((item) => item.id === link.payerId);
    const cvu = payer?.cvus.find((item) => item.currency === body.currency && item.status === 'active');
    if (!payer || !cvu) throw new ApiError('money.currency_mismatch', 422);
    const typed = body.concept?.trim();
    if (typed !== undefined && (typed.length < 1 || typed.length > MAX_PAY_CONCEPT)) throw new ApiError('validation.failed', 422);
    const company = this.state.orgs.find((item) => item.id === payer.orgId)?.legalName ?? '';
    let concept = typed ?? company;
    if (body.installmentNumber !== undefined) {
      const known = (this.state.installments[payer.id] ?? []).some((item) => item.number === body.installmentNumber);
      if (!known) throw new ApiError('installment.not_found', 404);
      concept = `Cuota ${body.installmentNumber} - ${concept}`;
    }
    const personal = this.state.accounts.find((item) => item.orgId === link.personalOrgId && item.kind === 'personal');
    if (!personal) throw new ApiError('account.not_found', 404);
    return this.createPayout(link.personalOrgId, { accountId: personal.id, currency: body.currency, amount: body.amount, to: cvu.cvu, concept }, credentials);
  }

  // ── Movements, reconciliation ──────────────────────────────────────────────────────────
  listMovements(orgId: string, filters: MovementFilters, cursor: string | null): Page<Movement> {
    this.orgFor(orgId);
    const all = buildMovements(this.state, orgId, filters);
    const offset = cursor ? Number(cursor) : 0;
    const items = all.slice(offset, offset + PAGE_SIZE);
    return { items, nextCursor: offset + PAGE_SIZE < all.length ? String(offset + PAGE_SIZE) : null };
  }

  getMovement(orgId: string, movementId: string): Movement {
    this.orgFor(orgId);
    const found = buildMovements(this.state, orgId, {}).find((item) => item.id === movementId);
    if (!found) throw new ApiError('movement.not_found', 404);
    return found;
  }

  reconciliationRuns(orgId: string): ReconciliationRun[] {
    this.orgFor(orgId);
    return (this.state.recon[orgId] ?? []).map((item) => item.run).sort((a, b) => b.day.localeCompare(a.day) || a.currency.localeCompare(b.currency));
  }

  reconciliationItems(orgId: string, runId: string): ReconciliationItem[] {
    this.orgFor(orgId);
    const found = (this.state.recon[orgId] ?? []).find((item) => item.run.id === runId);
    if (!found) throw new ApiError('reconciliation.run_not_found', 404);
    return found.items;
  }

  /** On-demand run (admins): every payin and settled payout of that day matches in the mock. */
  runReconciliation(orgId: string, body: RunReconciliationRequest): ReconciliationRun {
    this.requireAdmin(orgId);
    const sameDay = (iso: string) => iso.slice(0, 10) === body.day;
    const id = this.nextId('rec');
    const items: ReconciliationItem[] = [
      ...this.state.payins
        .filter((item) => item.orgId === orgId && item.currency === body.currency && sameDay(item.createdAt))
        .map((item) => ({ id: `${id}_${item.id}`, status: 'matched' as const, providerTransferId: `BIND-${item.id.toUpperCase()}`, direction: 'in' as const, pspAmount: item.amount, ledgerAmount: item.amount, createdAt: item.createdAt })),
      ...this.state.payouts
        .filter((item) => item.orgId === orgId && item.currency === body.currency && item.status === 'settled' && sameDay(item.createdAt))
        .map((item) => ({ id: `${id}_${item.id}`, status: 'matched' as const, providerTransferId: `BIND-${item.id.toUpperCase()}`, direction: 'out' as const, pspAmount: item.total, ledgerAmount: item.total, createdAt: item.createdAt })),
    ];
    const run = buildReconRun({ id, currency: body.currency, day: body.day, items, createdAt: this.iso() });
    this.state.recon[orgId] = [run, ...(this.state.recon[orgId] ?? [])];
    return run.run;
  }

  // ── ERP link ───────────────────────────────────────────────────────────────────────────
  private erp(orgId: string) {
    const found = this.state.erp[orgId];
    if (!found) throw new ApiError('erp_link.no_candidate', 404);
    return found;
  }

  erpCandidate(orgId: string): ErpCandidate {
    const org = this.orgFor(orgId);
    if (org.kycStatus !== 'approved') return { found: false, erpOrgId: null, displayName: null };
    return this.erp(orgId).candidate;
  }

  linkErp(orgId: string, body: ErpLinkRequest): ErpLink {
    this.requireAdmin(orgId);
    if (body.username.trim() === '' || body.password.length < 6) throw new ApiError('erp_link.credentials_invalid', 422);
    const erp = this.erp(orgId);
    if (!erp.candidate.found) throw new ApiError('erp_link.no_candidate', 404);
    if (erp.linkStartedAt !== null) throw new ApiError('erp_link.already_linked', 409);
    erp.linkStartedAt = this.clock.now();
    return this.erpLink(orgId);
  }

  erpLink(orgId: string): ErpLink {
    this.orgFor(orgId);
    const erp = this.erp(orgId);
    if (erp.linkStartedAt === null) return { status: 'not_linked', displayName: null, linkedAt: null, importedAt: null };
    return { status: erp.linkedAt ? 'linked' : 'importing', displayName: erp.candidate.displayName, linkedAt: new Date(erp.linkStartedAt).toISOString(), importedAt: erp.linkedAt };
  }

  /** Import finishes after ERP_IMPORT_MS: projects become accounts, buyers become payers (R-ERP-3). */
  private advanceErpImport(orgId: string): void {
    const erp = this.state.erp[orgId];
    if (!erp || erp.linkStartedAt === null || erp.linkedAt !== null || this.clock.now() - erp.linkStartedAt < ERP_IMPORT_MS) return;
    for (const project of erp.projects) {
      if (!this.state.accounts.some((item) => item.orgId === orgId && item.name === project)) {
        this.state.accounts.push({ id: this.nextId('acc'), orgId, kind: 'project', name: project, archived: false, createdAt: this.iso(), ledger: emptyLedger(), cvus: [] });
      }
    }
    for (const imported of erp.payers) {
      const account = this.state.accounts.find((item) => item.orgId === orgId && item.name === imported.project);
      if (!account) continue;
      const id = this.nextId('pay');
      const cvu = buildMockCvu({
        id: this.nextId('cvu'), payerId: id, accountId: account.id, currency: 'ARS', cvu: buildCvu(PSP_PREFIX, String(3_000_000_000_000 + this.state.seq)),
        alias: this.uniqueAlias(imported.name, 'ARS'), createdAt: this.iso(),
      });
      this.state.payers.push({ id, orgId, accountId: account.id, name: imported.name, docType: imported.docType, docNumber: imported.docNumber, email: null, erpRef: `ERP-${imported.docNumber}`, createdAt: this.iso(), cvus: [cvu] });
    }
    erp.linkedAt = this.iso();
  }

  // ── Realtime ───────────────────────────────────────────────────────────────────────────
  subscribe(orgId: string, listener: (event: LiveEvent) => void): () => void {
    const set = this.listeners.get(orgId) ?? new Set();
    set.add(listener);
    this.listeners.set(orgId, set);
    return () => {
      set.delete(listener);
    };
  }

  hasListeners(): boolean {
    return [...this.listeners.values()].some((set) => set.size > 0);
  }

  private emit(orgId: string, event: LiveEvent): void {
    for (const listener of this.listeners.get(orgId) ?? []) listener(event);
  }

  /** A payer transfers to their CVU: credit net of fee and notify (R-IN-1, R-IN-3). */
  simulateIncomingPayin(): void {
    for (const orgId of this.listeners.keys()) {
      if ((this.listeners.get(orgId)?.size ?? 0) === 0) continue;
      const candidates = this.state.payers.filter((item) => item.orgId === orgId && item.cvus.some((cvu) => cvu.status === 'active'));
      if (candidates.length === 0) continue;
      const pick = LIVE_AMOUNTS[this.liveTick % LIVE_AMOUNTS.length] ?? { currency: 'ARS' as const, minor: 150_000_00 };
      const payerRecord = candidates[this.liveTick % candidates.length];
      this.liveTick += 1;
      if (!payerRecord) continue;
      const currency = payerRecord.cvus.some((cvu) => cvu.currency === pick.currency && cvu.status === 'active') ? pick.currency : 'ARS';
      const account = this.state.accounts.find((item) => item.id === payerRecord.accountId);
      if (!account) continue;
      const payin = buildPayin({ id: this.nextId('pin'), payer: payerRecord, accountName: account.name, currency, amount: currency === pick.currency ? pick.minor : 150_000_00, createdAt: this.iso() });
      account.ledger[currency].posted += payin.net;
      this.state.payins.push(payin);
      const { orgId: _orgId, ...dto } = payin;
      this.emit(orgId, { type: 'payin.credited', payin: dto });
      this.emit(orgId, { type: 'balance.changed', accountId: account.id });
    }
  }
}

function emptyLedger(): MockAccount['ledger'] {
  return { ARS: { posted: 0, held: 0 }, USD: { posted: 0, held: 0 } };
}

function toBalance(currency: Currency, ledger: { posted: number; held: number }): Balance {
  return { currency, posted: ledger.posted, held: ledger.held, available: ledger.posted - ledger.held };
}

function toPayout(record: MockPayout): Payout {
  const { orgId: _orgId, ...payout } = record;
  return { ...payout, events: [...payout.events], attachments: [...payout.attachments] };
}

function requireStepUp(credentials: MoneyActionOptions): void {
  if (!credentials.stepUpToken) throw new ApiError('auth.step_up_required', 403);
}

function normalize(value: string): string {
  return value.normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase().trim();
}

/** The API's completeness rule (kyc/rules.go `missingItems`): data plus every required document. */
function isKycComplete(kyc: KycCase): boolean {
  const has = (kind: KycDocument['kind'], representativeDni: string | null = null) =>
    kyc.documents.some((item) => item.kind === kind && item.representativeDni === representativeDni);
  if (kyc.kind === 'person') return Boolean(kyc.person) && IDENTITY_DOCS.every((kind) => has(kind));
  if (!kyc.company) return false;
  const { representatives } = kyc.company;
  return has('bylaws') && has('afip_constancy') && representatives.every((rep) => IDENTITY_DOCS.every((kind) => has(kind, rep.dni)));
}

