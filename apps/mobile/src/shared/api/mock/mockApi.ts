import type { LenapayApi } from '../client';
import { MockBackend } from './backend';
import { renderProofPdf } from './proof';

/** Simulated network latency of every mock call. */
const LATENCY_MS = 250;
/** A simulated live payin arrives every ~40 s while someone listens to the event stream. */
const LIVE_PAYIN_EVERY_MS = 40_000;

function wait(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/** Copies the answer so screens never hold references into the mock's state. */
function clone<T>(value: T): T {
  return value === undefined ? value : (JSON.parse(JSON.stringify(value)) as T);
}

export function createMockApi(options: { latencyMs?: number; backend?: MockBackend } = {}): LenapayApi {
  const backend = options.backend ?? new MockBackend({ now: () => Date.now(), setTimeout: (callback, ms) => setTimeout(callback, ms) });
  const latency = options.latencyMs ?? LATENCY_MS;
  let liveTimer: ReturnType<typeof setInterval> | null = null;

  async function respond<T>(work: () => T | Promise<T>): Promise<T> {
    await wait(latency);
    return clone(await work());
  }

  // The timer only runs while someone listens: it stops with the last unsubscribe (M3).
  function syncLiveTimer(): void {
    if (backend.hasListeners() && liveTimer === null) {
      liveTimer = setInterval(() => backend.simulateIncomingPayin(), LIVE_PAYIN_EVERY_MS);
    } else if (!backend.hasListeners() && liveTimer !== null) {
      clearInterval(liveTimer);
      liveTimer = null;
    }
  }

  return {
    auth: {
      signup: (body) => respond(() => backend.signup(body)),
      verifyEmail: (body) => respond(() => backend.verifyEmail(body)),
      resendCode: () => respond(() => undefined),
      login: (body) => respond(() => backend.login(body)),
      refresh: () => respond(() => backend.refresh()),
      logout: () => respond(() => backend.logout()),
      stepUpChallenge: (body) => respond(() => backend.stepUpChallenge(body)),
      stepUpVerify: (body) => respond(() => backend.stepUpVerify(body)),
      registerDevice: (body) => respond(() => backend.registerDevice(body)),
      me: () => respond(() => backend.me()),
    },
    profile: {
      update: (body) => respond(() => backend.updateMe(body)),
      preferences: () => respond(() => backend.preferences()),
      updatePreferences: (body) => respond(() => backend.updatePreferences(body)),
    },
    orgs: {
      list: () => respond(() => backend.listOrgs()),
      create: (body) => respond(() => backend.createOrg(body)),
      members: (orgId) => respond(() => backend.members(orgId)),
      invite: (orgId, body) => respond(() => backend.invite(orgId, body)),
      updateMember: (orgId, userId, body) => respond(() => backend.updateMember(orgId, userId, body)),
      removeMember: (orgId, userId) => respond(() => backend.removeMember(orgId, userId)),
      transferOwnership: (orgId, body, options) => respond(() => backend.transferOwnership(orgId, body, options)),
      approvalPolicy: (orgId) => respond(() => backend.approvalPolicy(orgId)),
      updateApprovalPolicy: (orgId, body, options) => respond(() => backend.updateApprovalPolicy(orgId, body, options)),
    },
    kyc: {
      get: (orgId) => respond(() => backend.getKyc(orgId)),
      update: (orgId, body) => respond(() => backend.updateKyc(orgId, body)),
      uploadDocument: (orgId, body) => respond(() => backend.uploadKycDocument(orgId, body)),
      submit: (orgId) => respond(() => backend.submitKyc(orgId)),
    },
    accounts: {
      list: (orgId) => respond(() => backend.listAccounts(orgId)),
      balances: (orgId) => respond(() => backend.balances(orgId)),
      create: (orgId, body) => respond(() => backend.createAccount(orgId, body)),
      update: (orgId, accountId, body) => respond(() => backend.updateAccount(orgId, accountId, body)),
      transfer: (orgId, body, credentials) => respond(() => backend.transfer(orgId, body, credentials)),
      updateCvu: (orgId, accountId, cvuId, body) => respond(() => backend.updateAccountCvu(orgId, accountId, cvuId, body.alias)),
    },
    myPayments: {
      companies: ({ includeHidden }) => respond(() => backend.myPaymentCompanies(includeHidden)),
      setHidden: (linkId, hidden) => respond(() => backend.setMyPaymentHidden(linkId, hidden)),
      payins: (linkId, cursor) => respond(() => backend.myPaymentPayins(linkId, cursor)),
      installments: (linkId) => respond(() => backend.myPaymentInstallments(linkId)),
      pay: (linkId, body, credentials) => respond(() => backend.payMyPaymentCompany(linkId, body, credentials)),
    },
    payers: {
      list: (orgId, query) => respond(() => backend.listPayers(orgId, query)),
      get: (orgId, payerId) => respond(() => backend.getPayer(orgId, payerId)),
      create: (orgId, body) => respond(() => backend.createPayer(orgId, body)),
      issueCvu: (orgId, payerId, body, credentials) => respond(() => backend.issueCvu(orgId, payerId, body.currency, credentials)),
      updateCvu: (orgId, cvuId, body) => respond(() => backend.updateCvuAlias(orgId, cvuId, body.alias)),
      disableCvu: (orgId, cvuId) => respond(() => backend.disableCvu(orgId, cvuId)),
    },
    payins: {
      get: (orgId, payinId) => respond(() => backend.getPayin(orgId, payinId)),
    },
    payouts: {
      resolveDestination: (orgId, to) => respond(() => backend.resolveDestination(orgId, to)),
      quote: (orgId, body) => respond(() => backend.quote(orgId, body)),
      create: (orgId, body, credentials) => respond(() => backend.createPayout(orgId, body, credentials)),
      get: (orgId, payoutId) => respond(() => backend.getPayout(orgId, payoutId)),
      approve: (orgId, payoutId, credentials) => respond(() => backend.approvePayout(orgId, payoutId, credentials)),
      reject: (orgId, payoutId, body, credentials) => respond(() => backend.rejectPayout(orgId, payoutId, body.reason, credentials)),
      approvals: (orgId) => respond(() => backend.listApprovals(orgId)),
      uploadAttachment: (orgId, payoutId, file) => respond(() => backend.uploadAttachment(orgId, payoutId, file)),
      proof: (orgId, payoutId) => respond(() => renderProofPdf(backend.getPayout(orgId, payoutId))),
    },
    movements: {
      list: (orgId, filters, cursor) => respond(() => backend.listMovements(orgId, filters, cursor)),
      get: (orgId, movementId) => respond(() => backend.getMovement(orgId, movementId)),
    },
    reconciliation: {
      runs: (orgId) => respond(() => backend.reconciliationRuns(orgId)),
      items: (orgId, runId) => respond(() => backend.reconciliationItems(orgId, runId)),
      run: (orgId, body) => respond(() => backend.runReconciliation(orgId, body)),
    },
    erp: {
      candidate: (orgId) => respond(() => backend.erpCandidate(orgId)),
      link: (orgId, body) => respond(() => backend.linkErp(orgId, body)),
      get: (orgId) => respond(() => backend.erpLink(orgId)),
    },
    notifications: {
      // Push delivery is faked by the API's notify adapter; the mock only accepts the token.
      registerPushToken: () => respond(() => undefined),
    },
    events: {
      subscribe(orgId, onEvent) {
        const unsubscribe = backend.subscribe(orgId, onEvent);
        syncLiveTimer();
        return () => {
          unsubscribe();
          syncLiveTimer();
        };
      },
    },
  };
}
