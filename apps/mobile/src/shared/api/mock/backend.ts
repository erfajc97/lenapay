import { buildCvu, classifyDestination } from '@/shared/helpers/cbu';
import { isValidCuit } from '@/shared/helpers/cuit';
import { approvalBlocker, moneyActionBlocker } from '@/shared/helpers/permissions';

import type { MoneyActionOptions, PayerQuery } from '../client';
import { ApiError } from '../errors';
import type {
  Account,
  AuthTokens,
  Balance,
  CreateAccountRequest,
  CreateOrgRequest,
  CreatePayerRequest,
  CreatePayoutRequest,
  Currency,
  Cvu,
  Destination,
  ErpLink,
  ErpLinkRequest,
  InternalTransfer,
  InternalTransferRequest,
  Invite,
  InviteRequest,
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
  Payout,
  PayoutQuote,
  PayoutQuoteRequest,
  PayoutStatus,
  SignupRequest,
  SignupResponse,
  StepUpChallenge,
  StepUpToken,
  StepUpVerifyRequest,
  UpdateAccountRequest,
} from '../types';
import { CURRENCIES } from '../types';
import { buildPayin, createSeed, DEMO_USER_ID, DIRECTORY } from './fixtures';
import { payoutFee } from './feePlan';
import { buildMovements } from './movements';
import type { MockAccount, MockPayer, MockPayout, MockState } from './state';

/** The mock "server": business rules mirror the PRD so the UI meets the same errors as live. */
export const MOCK_EMAIL_CODE = '123456';
const KYC_REJECTED_DNIS = ['11111111'];
const KYC_DECISION_MS = 8_000;
const ERP_IMPORT_MS = 6_000;
const PAGE_SIZE = 20;
const PSP_PREFIX = '0031';

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

  private iso(): string {
    return new Date(this.clock.now()).toISOString();
  }

  private tokens(): AuthTokens {
    return { accessToken: this.nextId('at'), refreshToken: this.nextId('rt'), expiresIn: 900 };
  }

  // ── Auth ───────────────────────────────────────────────────────────────────────────────
  signup(body: SignupRequest): SignupResponse {
    const email = body.email.trim().toLowerCase();
    if (this.state.users.some((user) => user.email === email)) throw new ApiError('auth.email_taken', 409);
    this.state.pendingSignup = { email, fullName: body.fullName.trim() };
    return { userId: 'pending', email };
  }

  verifyEmail(body: { email: string; code: string }): AuthTokens {
    const pending = this.state.pendingSignup;
    if (!pending || pending.email !== body.email.trim().toLowerCase() || body.code !== MOCK_EMAIL_CODE) {
      throw new ApiError('auth.invalid_code', 400);
    }
    const id = this.nextId('u');
    this.state.users.push({ id, email: pending.email, fullName: pending.fullName, mfaEnabled: false, personalModeEnabled: false });
    this.state.pendingSignup = null;
    this.state.currentUserId = id;
    return this.tokens();
  }

  login(body: LoginRequest): AuthTokens {
    if (body.password.length < 6) throw new ApiError('auth.invalid_credentials', 401);
    const email = body.email.trim().toLowerCase();
    const known = body.provider === 'lebane' ? undefined : this.state.users.find((user) => user.email === email);
    this.state.currentUserId = known?.id ?? DEMO_USER_ID;
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

  stepUpChallenge(): StepUpChallenge {
    this.requireUserId();
    return { challengeId: this.nextId('ch'), nonce: this.nextId('nonce'), expiresAt: new Date(this.clock.now() + 120_000).toISOString() };
  }

  stepUpVerify(body: StepUpVerifyRequest): StepUpToken {
    this.requireUserId();
    if (body.method === 'password' && (body.password ?? '').length < 6) throw new ApiError('stepup.failed', 401);
    return { stepUpToken: this.nextId('su'), expiresAt: new Date(this.clock.now() + 300_000).toISOString() };
  }

  me(): Me {
    const user = this.requireUser();
    return {
      ...user,
      memberships: this.state.memberships.filter((item) => item.userId === user.id).map(({ orgId, role }) => ({ orgId, role })),
    };
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

  // ── Orgs and team ──────────────────────────────────────────────────────────────────────
  private orgFor(orgId: string): Org {
    this.settleAsyncWork(orgId);
    const userId = this.requireUserId();
    const membership = this.state.memberships.find((item) => item.orgId === orgId && item.userId === userId);
    const org = this.state.orgs.find((item) => item.id === orgId);
    if (!membership || !org) throw new ApiError('role.forbidden', 403);
    return { ...org, role: membership.role };
  }

  private requireMoneyAction(orgId: string): Org {
    const org = this.orgFor(orgId);
    const blocker = moneyActionBlocker(org);
    if (blocker === 'kyc') throw new ApiError('kyc.not_approved', 403);
    if (blocker === 'role') throw new ApiError('role.forbidden', 403);
    return org;
  }

  listOrgs(): Org[] {
    const userId = this.requireUserId();
    return this.state.memberships.filter((item) => item.userId === userId).map((item) => this.orgFor(item.orgId));
  }

  createOrg(body: CreateOrgRequest): Org {
    const userId = this.requireUserId();
    const id = this.nextId('org');
    this.state.orgs.push({ id, kind: body.kind, legalName: body.legalName.trim(), cuit: null, kycStatus: 'draft', erpLinked: false });
    this.state.memberships.push({ orgId: id, userId, role: 'admin' });
    this.state.kyc[id] = {
      id: `kyc_${id}`, orgId: id, kind: body.kind === 'company' ? 'company' : 'person', status: 'draft',
      cuit: null, legalName: null, legalAddress: null, fullName: null, dni: null,
      representatives: [], documents: [], timeline: [{ status: 'draft', at: this.iso() }], rejectionReason: null,
    };
    this.state.erp[id] = { candidate: { found: false, erpOrgName: null, cuit: null, projectsCount: 0, buyersCount: 0 }, projects: [], payers: [], linkStartedAt: null, linkedAt: null };
    return this.orgFor(id);
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
      .map((item) => ({ userId: item.id, fullName: item.email, email: item.email, role: item.role, status: 'invited' }));
    return [...active, ...invited];
  }

  invite(orgId: string, body: InviteRequest): Invite {
    const org = this.orgFor(orgId);
    if (org.role !== 'admin') throw new ApiError('role.forbidden', 403);
    const invite = { id: this.nextId('inv'), orgId, email: body.email.trim().toLowerCase(), role: body.role, expiresAt: new Date(this.clock.now() + 7 * 86_400_000).toISOString() };
    this.state.invites.push(invite);
    return { id: invite.id, email: invite.email, role: invite.role, expiresAt: invite.expiresAt };
  }

  // ── KYC ────────────────────────────────────────────────────────────────────────────────
  private kycCase(orgId: string): KycCase {
    this.orgFor(orgId);
    const found = this.state.kyc[orgId];
    if (!found) throw new ApiError('role.forbidden', 403);
    return found;
  }

  getKyc(orgId: string): KycCase {
    return this.kycCase(orgId);
  }

  updateKyc(orgId: string, body: KycUpdateRequest): KycCase {
    const current = this.kycCase(orgId);
    if (current.status === 'submitted' || current.status === 'approved') throw new ApiError('kyc.locked', 409);
    const next: KycCase = {
      ...current,
      cuit: body.cuit ?? current.cuit,
      legalName: body.legalName ?? current.legalName,
      legalAddress: body.legalAddress ?? current.legalAddress,
      fullName: body.fullName ?? current.fullName,
      dni: body.dni ?? current.dni,
      representatives: body.representatives ?? current.representatives,
    };
    this.state.kyc[orgId] = next;
    return next;
  }

  uploadKycDocument(orgId: string, body: KycDocumentUpload): KycDocument {
    const current = this.kycCase(orgId);
    const document: KycDocument = { id: this.nextId('doc'), type: body.type, fileName: body.fileName, representativeId: body.representativeId ?? null, uploadedAt: this.iso() };
    const others = current.documents.filter((item) => !(item.type === body.type && item.representativeId === document.representativeId));
    this.state.kyc[orgId] = { ...current, documents: [...others, document] };
    return document;
  }

  submitKyc(orgId: string): KycCase {
    const current = this.kycCase(orgId);
    if (!isKycComplete(current)) throw new ApiError('kyc.incomplete', 422);
    const next: KycCase = { ...current, status: 'submitted', rejectionReason: null, timeline: [...current.timeline, { status: 'submitted', at: this.iso() }] };
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
    const dnis = [current.dni, ...current.representatives.map((item) => item.dni)];
    const rejected = dnis.some((dni) => dni !== null && KYC_REJECTED_DNIS.includes(dni));
    const status = rejected ? 'rejected' : 'approved';
    this.state.kyc[orgId] = {
      ...current,
      status,
      rejectionReason: rejected ? 'No pudimos validar la identidad del DNI informado.' : null,
      timeline: [...current.timeline, { status, at: this.iso() }],
    };
    this.setOrgKyc(orgId, status);
    const org = this.state.orgs.find((item) => item.id === orgId);
    if (status === 'approved' && org && current.cuit) org.cuit = current.cuit;
    if (status === 'approved' && !this.state.accounts.some((item) => item.orgId === orgId)) {
      const kind = current.kind === 'company' ? 'main' : 'personal';
      this.state.accounts.push({ id: this.nextId('acc'), orgId, kind, name: kind === 'main' ? 'Cuenta principal' : 'Mi cuenta', archivedAt: null, ledger: emptyLedger() });
    }
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
      archivedAt: account.archivedAt,
      balances: CURRENCIES.map((currency) => toBalance(currency, account.ledger[currency])),
      payersCount: this.state.payers.filter((item) => item.accountId === account.id).length,
    };
  }

  listAccounts(orgId: string): Account[] {
    this.orgFor(orgId);
    return this.state.accounts.filter((item) => item.orgId === orgId).map((item) => this.toAccount(item));
  }

  balances(orgId: string): OrgBalances {
    this.orgFor(orgId);
    const open = this.state.accounts.filter((item) => item.orgId === orgId && !item.archivedAt);
    return {
      totals: CURRENCIES.map((currency) =>
        toBalance(currency, open.reduce((sum, item) => ({ posted: sum.posted + item.ledger[currency].posted, held: sum.held + item.ledger[currency].held }), { posted: 0, held: 0 })),
      ),
    };
  }

  createAccount(orgId: string, body: CreateAccountRequest): Account {
    this.requireMoneyAction(orgId);
    const account: MockAccount = { id: this.nextId('acc'), orgId, kind: 'project', name: body.name.trim(), archivedAt: null, ledger: emptyLedger() };
    this.state.accounts.push(account);
    return this.toAccount(account);
  }

  updateAccount(orgId: string, accountId: string, body: UpdateAccountRequest): Account {
    this.requireMoneyAction(orgId);
    const account = this.account(orgId, accountId);
    if (body.archived) {
      if (account.kind !== 'project') throw new ApiError('account.main_locked', 409);
      const hasMoney = CURRENCIES.some((currency) => account.ledger[currency].posted !== 0 || account.ledger[currency].held !== 0);
      if (hasMoney) throw new ApiError('account.not_empty', 409);
      account.archivedAt = this.iso();
    }
    if (body.archived === false) account.archivedAt = null;
    if (body.name !== undefined) account.name = body.name.trim();
    return this.toAccount(account);
  }

  transfer(orgId: string, body: InternalTransferRequest, credentials: MoneyActionOptions): InternalTransfer {
    return this.idempotent(`transfer:${orgId}`, credentials, body, () => {
      this.requireMoneyAction(orgId);
      requireStepUp(credentials);
      if (body.fromAccountId === body.toAccountId) throw new ApiError('transfer.same_account', 422);
      if (!Number.isSafeInteger(body.amount) || body.amount <= 0) throw new ApiError('validation.amount', 422);
      const from = this.account(orgId, body.fromAccountId);
      const to = this.account(orgId, body.toAccountId);
      if (from.archivedAt || to.archivedAt) throw new ApiError('account.archived', 409);
      const balance = from.ledger[body.currency];
      if (balance.posted - balance.held < body.amount) throw new ApiError('payout.insufficient_funds', 422);
      balance.posted -= body.amount;
      to.ledger[body.currency].posted += body.amount;
      const transfer = { id: this.nextId('tr'), orgId, fromAccountId: from.id, toAccountId: to.id, currency: body.currency, amount: body.amount, concept: body.concept, createdAt: this.iso() };
      this.state.transfers.push(transfer);
      return { id: transfer.id, fromAccountId: transfer.fromAccountId, toAccountId: transfer.toAccountId, currency: transfer.currency, amount: transfer.amount, createdAt: transfer.createdAt };
    });
  }

  /** Same key + same payload replays the stored answer; same key + other payload is 409 (R-OUT-3). */
  private idempotent<T>(scope: string, credentials: MoneyActionOptions, payload: unknown, effect: () => T): T {
    if (!credentials.idempotencyKey) throw new ApiError('idempotency.missing', 400);
    const storeKey = `${scope}:${credentials.idempotencyKey}`;
    const hash = JSON.stringify(payload);
    const stored = this.state.idempotency[storeKey];
    if (stored) {
      if (stored.hash !== hash) throw new ApiError('idempotency.conflict', 409);
      return stored.response as T;
    }
    const response = effect();
    this.state.idempotency[storeKey] = { hash, response };
    return response;
  }

  // ── Payers and CVUs ────────────────────────────────────────────────────────────────────
  private toPayer(record: MockPayer): Payer {
    const { orgId: _orgId, ...rest } = record;
    return { ...rest, accountName: this.state.accounts.find((item) => item.id === record.accountId)?.name ?? '' };
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
      .filter((item) => needle === '' || [item.name, item.docNumber, ...item.cvus.map((cvu) => cvu.alias)].some((field) => normalize(field).includes(needle)))
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
    if (account.archivedAt) throw new ApiError('account.archived', 409);
    if (body.docType === 'CUIT' && !isValidCuit(body.docNumber)) throw new ApiError('validation.doc', 422);
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
      const cvu: Cvu = { id: this.nextId('cvu'), payerId, currency, cvu: buildCvu(PSP_PREFIX, String(2_000_000_000_000 + this.state.seq)), alias: this.uniqueAlias(record.name, currency), status: 'active' };
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
    return this.state.payers.some((item) => item.cvus.some((cvu) => cvu.alias === alias));
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
    if (classifyDestination(next) !== 'alias') throw new ApiError('cvu.alias_invalid', 422);
    if (next !== cvu.alias && this.aliasTaken(next)) throw new ApiError('cvu.alias_taken', 409);
    cvu.alias = next;
    return cvu;
  }

  disableCvu(orgId: string, cvuId: string): void {
    this.requireMoneyAction(orgId);
    this.cvu(orgId, cvuId).status = 'disabled';
  }

  getPayin(orgId: string, payinId: string) {
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
    const known = DIRECTORY.find((item) => item.cbu === value || item.alias === value);
    if (known) return known;
    const ownCvu = this.state.payers.flatMap((item) => item.cvus.map((cvu) => ({ cvu, payer: item }))).find((item) => item.cvu.cvu === value || item.cvu.alias === value);
    if (ownCvu) {
      const { payer: holder, cvu } = ownCvu;
      return { type: 'cvu', cbu: cvu.cvu, alias: cvu.alias, holderName: holder.name, holderCuit: holder.docNumber, bankName: 'Lenapay' };
    }
    throw new ApiError('destination.not_found', 404);
  }

  quote(orgId: string, body: PayoutQuoteRequest): PayoutQuote {
    this.orgFor(orgId);
    const account = this.account(orgId, body.accountId);
    const fee = payoutFee(body.amount, body.currency);
    const balance = account.ledger[body.currency];
    return { accountId: account.id, currency: body.currency, amount: body.amount, fee, total: body.amount + fee, available: balance.posted - balance.held };
  }

  createPayout(orgId: string, body: CreatePayoutRequest, credentials: MoneyActionOptions): Payout {
    return this.idempotent(`payout:${orgId}`, credentials, body, () => {
      const org = this.requireMoneyAction(orgId);
      requireStepUp(credentials);
      if (!Number.isSafeInteger(body.amount) || body.amount <= 0) throw new ApiError('validation.amount', 422);
      const destination = this.resolveDestination(orgId, body.to);
      const account = this.account(orgId, body.accountId);
      if (account.archivedAt) throw new ApiError('account.archived', 409);
      const fee = payoutFee(body.amount, body.currency);
      const balance = account.ledger[body.currency];
      if (balance.posted - balance.held < body.amount + fee) throw new ApiError('payout.insufficient_funds', 422);
      balance.held += body.amount + fee;
      const user = this.requireUser();
      const status: PayoutStatus = org.kind === 'personal' ? 'approved' : 'pending_approval';
      const payout: MockPayout = {
        id: this.nextId('po'), orgId, accountId: account.id, accountName: account.name, currency: body.currency, amount: body.amount, fee,
        destination, concept: body.concept.trim(), attachmentName: body.attachmentName ?? null, status,
        createdBy: { userId: user.id, fullName: user.fullName }, decidedBy: null, rejectionReason: null, createdAt: this.iso(),
        events: [{ status: 'pending_approval', at: this.iso() }, ...(status === 'approved' ? [{ status, at: this.iso() }] : [])],
      };
      this.state.payouts.push(payout);
      if (status === 'approved') this.dispatch(payout);
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
      if (blocker === 'creator') throw new ApiError('payout.creator_cannot_approve', 403);
      if (blocker === 'role') throw new ApiError('role.forbidden', 403);
      const user = this.requireUser();
      payout.decidedBy = { userId: user.id, fullName: user.fullName };
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
      if (blocker === 'role') throw new ApiError('role.forbidden', 403);
      const user = this.requireUser();
      payout.decidedBy = { userId: user.id, fullName: user.fullName };
      payout.rejectionReason = reason.trim() || null;
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

  private transition(payout: MockPayout, status: PayoutStatus): void {
    payout.status = status;
    payout.events = [...payout.events, { status, at: this.iso() }];
    this.emit(payout.orgId, { type: 'payout.updated', payout: toPayout(payout) });
  }

  private releaseHold(payout: MockPayout): void {
    const account = this.state.accounts.find((item) => item.id === payout.accountId);
    if (account) account.ledger[payout.currency].held -= payout.amount + payout.fee;
  }

  /** The PSP dispatch runs off the request (R-OUT-5): sending, then settled. */
  private dispatch(payout: MockPayout): void {
    this.clock.setTimeout(() => this.transition(payout, 'sending'), 1_500);
    this.clock.setTimeout(() => {
      const account = this.state.accounts.find((item) => item.id === payout.accountId);
      if (account) {
        account.ledger[payout.currency].held -= payout.amount + payout.fee;
        account.ledger[payout.currency].posted -= payout.amount + payout.fee;
      }
      this.transition(payout, 'settled');
    }, 4_000);
  }

  // ── Movements, reconciliation ──────────────────────────────────────────────────────────
  listMovements(orgId: string, filters: MovementFilters, cursor: string | null): Page<Movement> {
    this.orgFor(orgId);
    const all = buildMovements(this.state, orgId, filters);
    const offset = cursor ? Number(cursor) : 0;
    const items = all.slice(offset, offset + PAGE_SIZE);
    return { items, nextCursor: offset + PAGE_SIZE < all.length ? String(offset + PAGE_SIZE) : null };
  }

  reconciliationRuns(orgId: string) {
    this.orgFor(orgId);
    return [...(this.state.recon[orgId] ?? [])].sort((a, b) => b.day.localeCompare(a.day) || a.currency.localeCompare(b.currency));
  }

  // ── ERP link ───────────────────────────────────────────────────────────────────────────
  private erp(orgId: string) {
    const found = this.state.erp[orgId];
    if (!found) throw new ApiError('erp.not_found', 404);
    return found;
  }

  erpCandidate(orgId: string) {
    const org = this.orgFor(orgId);
    if (org.kycStatus !== 'approved') return { found: false, erpOrgName: null, cuit: null, projectsCount: 0, buyersCount: 0 };
    return this.erp(orgId).candidate;
  }

  linkErp(orgId: string, body: ErpLinkRequest): ErpLink {
    const org = this.orgFor(orgId);
    if (org.role !== 'admin') throw new ApiError('role.forbidden', 403);
    if (body.username.trim() === '' || body.password.length < 6) throw new ApiError('erp.invalid_credentials', 401);
    const erp = this.erp(orgId);
    if (!erp.candidate.found) throw new ApiError('erp.not_found', 404);
    erp.linkStartedAt ??= this.clock.now();
    return this.erpLink(orgId);
  }

  erpLink(orgId: string): ErpLink {
    this.orgFor(orgId);
    const erp = this.erp(orgId);
    if (erp.linkStartedAt === null) return { status: 'not_linked', linkedAt: null, projects: { done: 0, total: 0 }, payers: { done: 0, total: 0 }, importedProjects: [] };
    const progress = Math.min(1, (this.clock.now() - erp.linkStartedAt) / ERP_IMPORT_MS);
    const done = (total: number) => Math.floor(total * progress);
    return {
      status: erp.linkedAt ? 'linked' : 'importing',
      linkedAt: erp.linkedAt,
      projects: { done: done(erp.projects.length), total: erp.projects.length },
      payers: { done: done(erp.payers.length), total: erp.payers.length },
      importedProjects: erp.linkedAt ? erp.projects : [],
    };
  }

  /** Import finishes after ERP_IMPORT_MS: projects become accounts, buyers become payers (R-ERP-3). */
  private advanceErpImport(orgId: string): void {
    const erp = this.state.erp[orgId];
    if (!erp || erp.linkStartedAt === null || erp.linkedAt !== null || this.clock.now() - erp.linkStartedAt < ERP_IMPORT_MS) return;
    for (const project of erp.projects) {
      if (!this.state.accounts.some((item) => item.orgId === orgId && item.name === project)) {
        this.state.accounts.push({ id: this.nextId('acc'), orgId, kind: 'project', name: project, archivedAt: null, ledger: emptyLedger() });
      }
    }
    for (const imported of erp.payers) {
      const account = this.state.accounts.find((item) => item.orgId === orgId && item.name === imported.project);
      if (!account) continue;
      const id = this.nextId('pay');
      this.state.payers.push({
        id, orgId, accountId: account.id, name: imported.name, docType: imported.docType, docNumber: imported.docNumber, email: null,
        erpRef: `ERP-${imported.docNumber}`, createdAt: this.iso(),
        cvus: [{ id: this.nextId('cvu'), payerId: id, currency: 'ARS', cvu: buildCvu(PSP_PREFIX, String(3_000_000_000_000 + this.state.seq)), alias: this.uniqueAlias(imported.name, 'ARS'), status: 'active' }],
      });
    }
    erp.linkedAt = this.iso();
    const org = this.state.orgs.find((item) => item.id === orgId);
    if (org) org.erpLinked = true;
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
  return { ...payout, events: [...payout.events] };
}

function requireStepUp(credentials: MoneyActionOptions): void {
  if (!credentials.stepUpToken) throw new ApiError('stepup.required', 401);
}

function normalize(value: string): string {
  return value.normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase().trim();
}

function isKycComplete(kyc: KycCase): boolean {
  const has = (type: KycDocument['type'], representativeId: string | null = null) =>
    kyc.documents.some((item) => item.type === type && item.representativeId === representativeId);
  if (kyc.kind === 'person') {
    return Boolean(kyc.fullName && kyc.dni && has('dni_front') && has('selfie'));
  }
  return Boolean(
    kyc.cuit && kyc.legalName && kyc.legalAddress && has('bylaws') && has('afip_constancy') && kyc.representatives.length > 0 &&
      kyc.representatives.every((rep) => has('dni_front', rep.id) && has('selfie', rep.id)),
  );
}

