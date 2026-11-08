import type { components } from '@lenapay/api-types';
import { File, Paths } from 'expo-file-system';
import EventSource from 'react-native-sse';

import { config } from '@/shared/config/env';
import { getAccessToken, readRefreshToken, saveTokens } from '@/shared/session/session';
import { useSessionStore } from '@/shared/stores/useSessionStore';

import type { LenapayApi, MoneyActionOptions } from '../client';
import { endpoints } from '../endpoints';
import { ApiError } from '../errors';
import type { AuthTokens, Currency, LocalUpload, MovementFilters } from '../types';
import { createHttp, type Http } from './http';
import {
  type OrgDirectory,
  parseRealtimeFrame,
  toAccount,
  toBalance,
  toErpLink,
  toInternalTransfer,
  toMe,
  toMember,
  toMovement,
  toPayer,
  toPayin,
  toPayout,
  toQuote,
  toReconciliationItem,
  toReconciliationRun,
} from './mappers';

type Schemas = components['schemas'];
interface List<T> {
  items: T[];
  nextCursor?: string | null;
}

/** Lists the screens show whole (payers, approvals, runs) ask for the contract's maximum page. */
const FULL_PAGE = '100';
/** Account names and member emails change rarely; one load serves a burst of mapped lists. */
const DIRECTORY_TTL_MS = 30_000;

/** Contract headers of money mutations (R-ID-3, R-OUT-3). */
function moneyHeaders(options: MoneyActionOptions): Record<string, string> {
  return { 'Idempotency-Key': options.idempotencyKey, 'X-Step-Up-Token': options.stepUpToken };
}

function money(minor: number, currency: Currency): Schemas['Money'] {
  return { minor, currency };
}

function movementQuery(filters: MovementFilters, cursor: string | null) {
  return { account: filters.account, type: filters.type, status: filters.status, currency: filters.currency, from: filters.from, to: filters.to, cursor };
}

/** React Native's FormData takes `{uri, name, type}` for a file part. */
function filePart(file: LocalUpload): Blob {
  return { uri: file.fileUri, name: file.fileName, type: file.mimeType } as unknown as Blob;
}

/** `null` when the API answers the given "not there yet" code (a state, not an error). */
async function orNull<T>(request: Promise<T>, code: string): Promise<T | null> {
  try {
    return await request;
  } catch (error) {
    if (error instanceof ApiError && error.code === code) return null;
    throw error;
  }
}

/** Per-org names the contract does not embed (account names, member emails), cached briefly. */
function createDirectory(http: Http) {
  const cache = new Map<string, { loadedAt: number; directory: Promise<OrgDirectory> }>();

  async function load(orgId: string): Promise<OrgDirectory> {
    const [accounts, members] = await Promise.all([
      http<List<Schemas['Account']>>('GET', endpoints.accounts.list(orgId)),
      // A member list the role cannot read only costs the creator's email on payouts.
      http<List<Schemas['Member']>>('GET', endpoints.orgs.members(orgId)).catch(() => ({ items: [] })),
    ]);
    const names = new Map(accounts.items.map((account) => [account.id, account.name]));
    const emails = new Map(members.items.map((member) => [member.userId, member.email]));
    return { accountName: (id) => names.get(id) ?? '', memberLabel: (id) => emails.get(id) ?? null };
  }

  return {
    get(orgId: string): Promise<OrgDirectory> {
      const cached = cache.get(orgId);
      if (cached && Date.now() - cached.loadedAt < DIRECTORY_TTL_MS) return cached.directory;
      const directory = load(orgId);
      cache.set(orgId, { loadedAt: Date.now(), directory });
      // A failed load is not cached: the next screen retries it.
      directory.catch(() => cache.delete(orgId));
      return directory;
    },
    forget(orgId: string): void {
      cache.delete(orgId);
    },
  };
}

const EMPTY_DIRECTORY: OrgDirectory = { accountName: () => '', memberLabel: () => null };

/** `live`: the Lenapay API at `baseUrl`, same interface as the mock. */
export function createLiveApi(options: { baseUrl: string }): LenapayApi {
  const { baseUrl } = options;
  const http: Http = createHttp({
    baseUrl,
    appVersion: config.appVersion,
    timeoutMs: config.requestTimeoutMs,
    getAccessToken,
    refreshSession,
    onSessionExpired: () => useSessionStore.getState().setStatus('signedOut'),
  });
  const directories = createDirectory(http);

  async function refreshSession(): Promise<string | null> {
    const refreshToken = await readRefreshToken();
    if (!refreshToken) return null;
    try {
      const tokens = await http<AuthTokens>('POST', endpoints.auth.refresh, { auth: false, body: { refreshToken } });
      await saveTokens(tokens);
      return tokens.accessToken;
    } catch {
      // A rejected or unreachable refresh means "signed out"; the caller expires the session.
      return null;
    }
  }

  async function withDirectory<T, R>(orgId: string, request: Promise<T>, map: (dto: T, directory: OrgDirectory) => R): Promise<R> {
    const [dto, directory] = await Promise.all([request, directories.get(orgId)]);
    return map(dto, directory);
  }

  function upload<T>(path: string, fields: Record<string, string | undefined>, file: LocalUpload): Promise<T> {
    const form = new FormData();
    for (const [key, value] of Object.entries(fields)) if (value !== undefined) form.append(key, value);
    form.append('file', filePart(file));
    return http<T>('POST', path, { body: form });
  }

  function subscribe(orgId: string, onEvent: Parameters<LenapayApi['events']['subscribe']>[1]): () => void {
    let source: EventSource | null = null;
    let closed = false;
    let opened = false;

    function connect(): void {
      const token = getAccessToken();
      const next = new EventSource(`${baseUrl}${endpoints.events(orgId)}`, {
        headers: { ...(token ? { Authorization: `Bearer ${token}` } : {}), 'X-App-Version': config.appVersion, Accept: 'text/event-stream' },
      });
      next.addEventListener('open', () => {
        // Delivery is best effort: after a reconnect, refetch what may have been missed.
        if (opened) onEvent({ type: 'resync' });
        opened = true;
      });
      next.addEventListener('message', (event) => {
        void directories
          .get(orgId)
          .catch(() => EMPTY_DIRECTORY)
          .then((directory) => {
            const parsed = parseRealtimeFrame(event.data, directory);
            if (parsed && !closed) onEvent(parsed);
          });
      });
      next.addEventListener('error', (event) => {
        // An expired access token: rotate it once and reopen with the new bearer.
        if (event.type === 'error' && event.xhrStatus === 401 && !closed) {
          next.removeAllEventListeners();
          next.close();
          void http.refresh().then((fresh) => {
            if (fresh && !closed) connect();
          });
        }
      });
      source = next;
    }

    connect();
    return () => {
      closed = true;
      source?.removeAllEventListeners();
      source?.close();
    };
  }

  return {
    auth: {
      signup: (body) => http('POST', endpoints.auth.signup, { auth: false, body }),
      verifyEmail: (body) => http('POST', endpoints.auth.verifyEmail, { auth: false, body }),
      resendCode: (body) => http('POST', endpoints.auth.resendCode, { auth: false, body }),
      login: (body) => http('POST', endpoints.auth.login, { auth: false, body }),
      refresh: (refreshToken) => http('POST', endpoints.auth.refresh, { auth: false, body: { refreshToken } }),
      logout: () => http('POST', endpoints.auth.logout),
      stepUpChallenge: (body) => http('POST', endpoints.auth.stepUpChallenge, { body }),
      stepUpVerify: (body) => http('POST', endpoints.auth.stepUpVerify, { body }),
      registerDevice: (body) => http('POST', endpoints.devices, { body }),
      me: async () => toMe(await http<Schemas['Me']>('GET', endpoints.me)),
    },
    orgs: {
      list: async () => (await http<List<Schemas['Organization']>>('GET', endpoints.orgs.list)).items,
      create: (body) => http('POST', endpoints.orgs.list, { body }),
      members: async (orgId) => (await http<List<Schemas['Member']>>('GET', endpoints.orgs.members(orgId))).items.map(toMember),
      invite: (orgId, body) => http('POST', endpoints.orgs.invites(orgId), { body }),
      updateMember: async (orgId, userId, body) => {
        directories.forget(orgId);
        return toMember(await http<Schemas['Member']>('PATCH', endpoints.orgs.member(orgId, userId), { body }));
      },
      removeMember: (orgId, userId) => {
        directories.forget(orgId);
        return http('DELETE', endpoints.orgs.member(orgId, userId));
      },
    },
    kyc: {
      get: (orgId) => http('GET', endpoints.kyc.case(orgId)),
      update: (orgId, body) => http('PUT', endpoints.kyc.case(orgId), { body }),
      uploadDocument: (orgId, body) => upload(endpoints.kyc.documents(orgId), { kind: body.kind, representativeDni: body.representativeDni }, body),
      submit: (orgId) => http('POST', endpoints.kyc.submit(orgId)),
    },
    accounts: {
      list: async (orgId) => (await http<List<Schemas['Account']>>('GET', endpoints.accounts.list(orgId))).items.map(toAccount),
      balances: async (orgId) => ({ totals: (await http<Schemas['Balances']>('GET', endpoints.accounts.balances(orgId))).totals.map(toBalance) }),
      create: async (orgId, body) => {
        directories.forget(orgId);
        return toAccount(await http<Schemas['Account']>('POST', endpoints.accounts.list(orgId), { body }));
      },
      update: async (orgId, accountId, body) => {
        directories.forget(orgId);
        return toAccount(await http<Schemas['Account']>('PATCH', endpoints.accounts.one(orgId, accountId), { body }));
      },
      transfer: async (orgId, body, moneyOptions) => {
        const request: Schemas['InternalTransferRequest'] = {
          fromAccountId: body.fromAccountId,
          toAccountId: body.toAccountId,
          amount: money(body.amount, body.currency),
          concept: body.concept,
        };
        return toInternalTransfer(await http<Schemas['InternalTransfer']>('POST', endpoints.accounts.internalTransfers(orgId), { body: request, headers: moneyHeaders(moneyOptions) }));
      },
    },
    payers: {
      list: (orgId, query) =>
        withDirectory(orgId, http<List<Schemas['Payer']>>('GET', endpoints.payers.list(orgId), { query: { q: query.search, accountId: query.account, limit: FULL_PAGE } }), (page, directory) =>
          page.items.map((payer) => toPayer(payer, directory)),
        ),
      get: (orgId, payerId) => withDirectory(orgId, http<Schemas['Payer']>('GET', endpoints.payers.one(orgId, payerId)), toPayer),
      create: (orgId, body) => withDirectory(orgId, http<Schemas['Payer']>('POST', endpoints.payers.list(orgId), { body }), toPayer),
      issueCvu: (orgId, payerId, body, moneyOptions) => http('POST', endpoints.payers.cvus(orgId, payerId), { body, headers: moneyHeaders(moneyOptions) }),
      updateCvu: (orgId, cvuId, body) => http('PATCH', endpoints.payers.cvu(orgId, cvuId), { body }),
      disableCvu: (orgId, cvuId) => http('DELETE', endpoints.payers.cvu(orgId, cvuId)),
    },
    payins: {
      get: (orgId, payinId) => withDirectory(orgId, http<Schemas['Payin']>('GET', endpoints.payins.one(orgId, payinId)), toPayin),
    },
    payouts: {
      resolveDestination: (orgId, to) => http('GET', endpoints.payouts.resolveDestination(orgId), { query: { to } }),
      quote: async (orgId, body) => {
        const request: Schemas['PayoutQuoteRequest'] = { accountId: body.accountId, amount: money(body.amount, body.currency), to: body.to };
        return toQuote(await http<Schemas['PayoutQuote']>('POST', endpoints.payouts.quote(orgId), { body: request }));
      },
      create: (orgId, body, moneyOptions) => {
        const request: Schemas['CreatePayoutRequest'] = { accountId: body.accountId, amount: money(body.amount, body.currency), to: body.to, concept: body.concept };
        return withDirectory(orgId, http<Schemas['Payout']>('POST', endpoints.payouts.create(orgId), { body: request, headers: moneyHeaders(moneyOptions) }), toPayout);
      },
      get: (orgId, payoutId) => withDirectory(orgId, http<Schemas['Payout']>('GET', endpoints.payouts.one(orgId, payoutId)), toPayout),
      approve: (orgId, payoutId, moneyOptions) =>
        withDirectory(orgId, http<Schemas['Payout']>('POST', endpoints.payouts.approve(orgId, payoutId), { headers: moneyHeaders(moneyOptions) }), toPayout),
      reject: (orgId, payoutId, body, moneyOptions) =>
        withDirectory(orgId, http<Schemas['Payout']>('POST', endpoints.payouts.reject(orgId, payoutId), { body, headers: moneyHeaders(moneyOptions) }), toPayout),
      approvals: (orgId) =>
        withDirectory(orgId, http<List<Schemas['Payout']>>('GET', endpoints.payouts.approvals(orgId), { query: { limit: FULL_PAGE } }), (page, directory) =>
          page.items.map((payout) => toPayout(payout, directory)),
        ),
      uploadAttachment: (orgId, payoutId, file) => upload(endpoints.payouts.attachments(orgId, payoutId), {}, file),
      proof: async (orgId, payoutId) => {
        const token = getAccessToken();
        const file = await File.downloadFileAsync(`${baseUrl}${endpoints.payouts.proof(orgId, payoutId)}`, new File(Paths.cache, `comprobante-${payoutId}.pdf`), {
          headers: { ...(token ? { Authorization: `Bearer ${token}` } : {}), 'X-App-Version': config.appVersion },
          idempotent: true,
        });
        return { uri: file.uri, mimeType: 'application/pdf' };
      },
    },
    movements: {
      list: (orgId, filters, cursor) =>
        withDirectory(orgId, http<List<Schemas['Movement']>>('GET', endpoints.movements.list(orgId), { query: movementQuery(filters, cursor) }), (page, directory) => ({
          items: page.items.map((movement) => toMovement(movement, directory)),
          nextCursor: page.nextCursor ?? null,
        })),
      get: (orgId, movementId) => withDirectory(orgId, http<Schemas['Movement']>('GET', endpoints.movements.one(orgId, movementId)), toMovement),
    },
    reconciliation: {
      runs: async (orgId) => (await http<List<Schemas['ReconciliationRun']>>('GET', endpoints.reconciliation.runs(orgId), { query: { limit: FULL_PAGE } })).items.map(toReconciliationRun),
      items: async (orgId, runId) =>
        (await http<List<Schemas['ReconciliationItem']>>('GET', endpoints.reconciliation.items(orgId, runId), { query: { limit: FULL_PAGE } })).items.map(toReconciliationItem),
      run: async (orgId, body) => toReconciliationRun(await http<Schemas['ReconciliationRun']>('POST', endpoints.reconciliation.runs(orgId), { body })),
    },
    erp: {
      candidate: async (orgId) =>
        (await orNull(http<Schemas['ErpLinkCandidate']>('GET', endpoints.erp.candidate(orgId)), 'erp_link.no_candidate')) ?? { found: false, erpOrgId: null, displayName: null },
      link: async (orgId, body) => toErpLink(await http<Schemas['ErpLink']>('POST', endpoints.erp.link(orgId), { body })),
      get: async (orgId) => toErpLink(await orNull(http<Schemas['ErpLink']>('GET', endpoints.erp.link(orgId)), 'erp_link.not_found')),
    },
    notifications: {
      registerPushToken: (token, platform) => http('POST', endpoints.pushTokens, { body: { token, platform } }),
    },
    events: { subscribe },
  };
}
