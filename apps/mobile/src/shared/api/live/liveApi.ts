import { File, Paths } from 'expo-file-system';
import EventSource from 'react-native-sse';

import { config } from '@/shared/config/env';
import { getAccessToken, readRefreshToken, saveTokens } from '@/shared/session/session';
import { useSessionStore } from '@/shared/stores/useSessionStore';

import type { LenapayApi, MoneyActionOptions } from '../client';
import { endpoints } from '../endpoints';
import { ApiError } from '../errors';
import type { AuthTokens, LiveEvent, MovementFilters } from '../types';
import { createHttp, type Http } from './http';

/** Contract headers of money mutations (R-ID-3, R-OUT-3). */
function moneyHeaders(options: MoneyActionOptions): Record<string, string> {
  return { 'Idempotency-Key': options.idempotencyKey, 'X-Step-Up-Token': options.stepUpToken };
}

function movementQuery(filters: MovementFilters, cursor: string | null) {
  return { ...filters, cursor };
}

function parseLiveEvent(data: string | null): LiveEvent | null {
  if (!data) return null;
  try {
    const value = JSON.parse(data) as Partial<LiveEvent>;
    return value.type === 'payin.credited' || value.type === 'payout.updated' ? (value as LiveEvent) : null;
  } catch {
    return null;
  }
}

/** `live`: the Lenapay API at EXPO_PUBLIC_API_URL, same interface as the mock. */
export function createLiveApi(): LenapayApi {
  const http: Http = createHttp({
    baseUrl: config.apiUrl,
    appVersion: config.appVersion,
    timeoutMs: config.requestTimeoutMs,
    getAccessToken,
    refreshSession,
    onSessionExpired: () => useSessionStore.getState().setStatus('signedOut'),
  });

  async function refreshSession(): Promise<string | null> {
    const refreshToken = await readRefreshToken();
    if (!refreshToken) return null;
    try {
      const tokens = await http<AuthTokens>('POST', endpoints.auth.refresh, { auth: false, body: { refreshToken } });
      await saveTokens(tokens);
      return tokens.accessToken;
    } catch {
      return null;
    }
  }

  return {
    auth: {
      signup: (body) => http('POST', endpoints.auth.signup, { auth: false, body }),
      verifyEmail: (body) => http('POST', endpoints.auth.verifyEmail, { auth: false, body }),
      login: (body) => http('POST', endpoints.auth.login, { auth: false, body }),
      refresh: (refreshToken) => http('POST', endpoints.auth.refresh, { auth: false, body: { refreshToken } }),
      logout: () => http('POST', endpoints.auth.logout),
      stepUpChallenge: () => http('POST', endpoints.auth.stepUpChallenge),
      stepUpVerify: async (body) => {
        // Device-key signing needs a native Ed25519 module the app does not ship yet.
        if (body.method === 'device_key' && !body.signature) throw new ApiError('stepup.device_key_unavailable', 0);
        return http('POST', endpoints.auth.stepUpVerify, { body });
      },
      me: () => http('GET', endpoints.me),
    },
    orgs: {
      list: () => http('GET', endpoints.orgs.list),
      create: (body) => http('POST', endpoints.orgs.list, { body }),
      members: (orgId) => http('GET', endpoints.orgs.members(orgId)),
      invite: (orgId, body) => http('POST', endpoints.orgs.invites(orgId), { body }),
    },
    kyc: {
      get: (orgId) => http('GET', endpoints.kyc.case(orgId)),
      update: (orgId, body) => http('PUT', endpoints.kyc.case(orgId), { body }),
      uploadDocument: (orgId, body) => {
        const form = new FormData();
        form.append('type', body.type);
        if (body.representativeId) form.append('representativeId', body.representativeId);
        form.append('file', { uri: body.fileUri, name: body.fileName, type: body.fileName.endsWith('.pdf') ? 'application/pdf' : 'image/jpeg' } as unknown as Blob);
        return http('POST', endpoints.kyc.documents(orgId), { body: form });
      },
      submit: (orgId) => http('POST', endpoints.kyc.submit(orgId)),
    },
    accounts: {
      list: (orgId) => http('GET', endpoints.accounts.list(orgId)),
      balances: (orgId) => http('GET', endpoints.accounts.balances(orgId)),
      create: (orgId, body) => http('POST', endpoints.accounts.list(orgId), { body }),
      update: (orgId, accountId, body) => http('PATCH', endpoints.accounts.one(orgId, accountId), { body }),
      transfer: (orgId, body, options) => http('POST', endpoints.accounts.internalTransfers(orgId), { body, headers: moneyHeaders(options) }),
    },
    payers: {
      list: (orgId, query) => http('GET', endpoints.payers.list(orgId), { query: { search: query.search, account: query.account } }),
      get: (orgId, payerId) => http('GET', endpoints.payers.one(orgId, payerId)),
      create: (orgId, body) => http('POST', endpoints.payers.list(orgId), { body }),
      issueCvu: (orgId, payerId, body, options) => http('POST', endpoints.payers.cvus(orgId, payerId), { body, headers: moneyHeaders(options) }),
      updateCvu: (orgId, cvuId, body) => http('PATCH', endpoints.payers.cvu(orgId, cvuId), { body }),
      disableCvu: (orgId, cvuId) => http('DELETE', endpoints.payers.cvu(orgId, cvuId)),
    },
    payins: {
      get: (orgId, payinId) => http('GET', endpoints.payins.one(orgId, payinId)),
    },
    payouts: {
      resolveDestination: (orgId, to) => http('GET', endpoints.payouts.resolveDestination(orgId), { query: { to } }),
      quote: (orgId, body) => http('POST', endpoints.payouts.quote(orgId), { body }),
      create: (orgId, body, options) => http('POST', endpoints.payouts.create(orgId), { body, headers: moneyHeaders(options) }),
      get: (orgId, payoutId) => http('GET', endpoints.payouts.one(orgId, payoutId)),
      approve: (orgId, payoutId, options) => http('POST', endpoints.payouts.approve(orgId, payoutId), { headers: moneyHeaders(options) }),
      reject: (orgId, payoutId, body, options) => http('POST', endpoints.payouts.reject(orgId, payoutId), { body, headers: moneyHeaders(options) }),
      approvals: (orgId) => http('GET', endpoints.payouts.approvals(orgId)),
      proof: async (orgId, payoutId) => {
        const token = getAccessToken();
        const file = await File.downloadFileAsync(`${config.apiUrl}${endpoints.payouts.proof(orgId, payoutId)}`, new File(Paths.cache, `comprobante-${payoutId}.pdf`), {
          headers: { ...(token ? { Authorization: `Bearer ${token}` } : {}), 'X-App-Version': config.appVersion },
          idempotent: true,
        });
        return { uri: file.uri, mimeType: 'application/pdf' };
      },
    },
    movements: {
      list: (orgId, filters, cursor) => http('GET', endpoints.movements.list(orgId), { query: movementQuery(filters, cursor) }),
    },
    reconciliation: {
      runs: (orgId) => http('GET', endpoints.reconciliation.runs(orgId)),
    },
    erp: {
      candidate: (orgId) => http('GET', endpoints.erp.candidate(orgId)),
      link: (orgId, body) => http('POST', endpoints.erp.link(orgId), { body }),
      get: (orgId) => http('GET', endpoints.erp.link(orgId)),
    },
    notifications: {
      registerPushToken: (token) => http('POST', endpoints.pushTokens, { body: { token } }),
    },
    events: {
      subscribe(orgId, onEvent) {
        const token = getAccessToken();
        const source = new EventSource(`${config.apiUrl}${endpoints.events(orgId)}`, {
          headers: { ...(token ? { Authorization: `Bearer ${token}` } : {}), 'X-App-Version': config.appVersion },
        });
        source.addEventListener('message', (event) => {
          const parsed = parseLiveEvent(event.data);
          if (parsed) onEvent(parsed);
        });
        return () => {
          source.removeAllEventListeners();
          source.close();
        };
      },
    },
  };
}
