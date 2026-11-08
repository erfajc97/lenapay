import { randomUUID } from 'node:crypto';

import { afterAll, beforeAll, describe, expect, it, vi } from 'vitest';

import { saveTokens } from '@/shared/session/session';
import { generateDeviceKey } from '@/shared/stepup/deviceKey';
import { verifyStepUp } from '@/shared/stepup/stepUpFlow';

import type { LenapayApi } from '../../client';
import type { LiveEvent, Org, Payout } from '../../types';
import { createLiveApi } from '../liveApi';

/**
 * The app's live client end to end against the running stack (`pnpm test:live`): login → me →
 * orgs → balances → payers → movements → quote → payout with password step-up → approval by a
 * second person (password and device-key step-up) → realtime events over SSE.
 * Seed users from the README (`make seed`); demo password only, never a real credential.
 */
const BASE_URL = process.env.LIVE_API_URL ?? 'http://localhost:18080';
const PASSWORD = 'Lenapay2026!';
const DEMO = 'demo@lenapay.app';
const APPROVER = 'approver@lenapay.app';
const DESTINATION = 'hormigonera.sur';

/**
 * react-native-sse rides on XMLHttpRequest, which node lacks: the same contract (headers,
 * `open` / `message` / `error`, close) over a streamed fetch, so the app's subscribe() and
 * frame parsing run unchanged against the real `text/event-stream`.
 */
vi.mock('react-native-sse', () => {
  type Listener = (event: Record<string, unknown>) => void;
  class NodeEventSource {
    private listeners = new Map<string, Set<Listener>>();
    private controller = new AbortController();
    constructor(url: string, options: { headers?: Record<string, string> } = {}) {
      void this.run(url, options.headers ?? {});
    }
    addEventListener(type: string, listener: Listener) {
      const set = this.listeners.get(type) ?? new Set<Listener>();
      set.add(listener);
      this.listeners.set(type, set);
    }
    removeAllEventListeners() {
      this.listeners.clear();
    }
    close() {
      this.controller.abort();
    }
    private emit(type: string, event: Record<string, unknown>) {
      for (const listener of this.listeners.get(type) ?? []) listener({ type, ...event });
    }
    private async run(url: string, headers: Record<string, string>) {
      try {
        const response = await fetch(url, { headers, signal: this.controller.signal });
        if (!response.ok || !response.body) {
          this.emit('error', { message: 'http', xhrState: 4, xhrStatus: response.status });
          return;
        }
        this.emit('open', {});
        const reader = response.body.pipeThrough(new TextDecoderStream()).getReader();
        let buffer = '';
        for (;;) {
          const { value, done } = await reader.read();
          if (done) return;
          buffer += value;
          let end = buffer.indexOf('\n\n');
          while (end >= 0) {
            const block = buffer.slice(0, end);
            buffer = buffer.slice(end + 2);
            const data = block
              .split('\n')
              .filter((line) => line.startsWith('data:'))
              .map((line) => line.slice(5).trimStart())
              .join('\n');
            if (data) this.emit('message', { data, lastEventId: null, url });
            end = buffer.indexOf('\n\n');
          }
        }
      } catch (error) {
        if (!this.controller.signal.aborted) this.emit('error', { message: String(error), xhrState: 4, xhrStatus: 0 });
      }
    }
  }
  return { default: NodeEventSource };
});

vi.mock('expo-file-system', () => ({ File: class {}, Paths: { cache: '' } }));

async function signIn(api: LenapayApi, email: string): Promise<void> {
  await saveTokens(await api.auth.login({ email, password: PASSWORD }));
}

function stepUpWithPassword(api: LenapayApi): Promise<string> {
  return verifyStepUp({ method: 'password', password: PASSWORD }, { auth: api.auth, readDeviceKey: async () => null }).then((token) => token.stepUpToken);
}

/** Resolves with the first event that matches, or rejects after the timeout. */
function waitFor(events: LiveEvent[], match: (event: LiveEvent) => boolean, timeoutMs = 15_000): Promise<LiveEvent> {
  const started = Date.now();
  return new Promise((resolve, reject) => {
    const tick = () => {
      const found = events.find(match);
      if (found) resolve(found);
      else if (Date.now() - started > timeoutMs) reject(new Error(`no matching event in ${timeoutMs} ms; got ${events.map((event) => event.type).join(', ')}`));
      else setTimeout(tick, 100);
    };
    tick();
  });
}

describe(`live client against ${BASE_URL}`, () => {
  const api = createLiveApi({ baseUrl: BASE_URL });
  const events: LiveEvent[] = [];
  let unsubscribe: () => void = () => undefined;
  let org: Org;
  let accountId: string;
  let payout: Payout;

  beforeAll(async () => {
    const health = await fetch(`${BASE_URL}/healthz`).catch(() => null);
    if (!health?.ok) throw new Error(`API not reachable at ${BASE_URL}: start the stack (docker compose up -d --wait) and seed it`);
  });

  afterAll(() => unsubscribe());

  it('logs in and reads the profile and the organizations', async () => {
    await signIn(api, DEMO);
    const me = await api.auth.me();
    expect(me).toMatchObject({ email: DEMO, fullName: null });
    // With PERSONAL_MODE_ENABLED the API opens a personal wallet at signup: personal mode follows it.
    expect(me.personalModeEnabled).toBe(me.memberships.some((membership) => membership.orgKind === 'personal'));
    const orgs = await api.orgs.list();
    const found = orgs.find((item) => item.legalName === 'Desarrollos Del Sur SA');
    expect(found).toMatchObject({ kind: 'company', kycStatus: 'approved', role: 'admin' });
    org = found as Org;
    expect(me.memberships.map((item) => item.orgId)).toContain(org.id);
  });

  it('reads balances, accounts, payers (with account names) and members', async () => {
    const balances = await api.accounts.balances(org.id);
    expect(balances.totals.map((item) => item.currency).sort()).toEqual(['ARS', 'USD']);
    for (const total of balances.totals) expect(total.available).toBe(total.posted - total.held);
    const accounts = await api.accounts.list(org.id);
    const torre = accounts.find((item) => item.name === 'Torre Palermo');
    expect(torre).toMatchObject({ kind: 'project', archived: false });
    accountId = torre?.id ?? '';
    const payers = await api.payers.list(org.id, {});
    expect(payers.length).toBeGreaterThan(0);
    expect(payers.every((payer) => payer.accountName !== '')).toBe(true);
    expect(payers.some((payer) => payer.cvus.length > 0)).toBe(true);
    const searched = await api.payers.list(org.id, { search: payers[0]?.name.split(' ')[0] });
    expect(searched.some((payer) => payer.id === payers[0]?.id)).toBe(true);
    const members = await api.orgs.members(org.id);
    expect(members.map((member) => member.email).sort()).toEqual([APPROVER, DEMO]);
  });

  it('pages movements, filters them and reads one by id', async () => {
    const page = await api.movements.list(org.id, {}, null);
    expect(page.items.length).toBeGreaterThan(0);
    const first = page.items[0];
    if (!first) throw new Error('no movements');
    expect(await api.movements.get(org.id, first.id)).toEqual(first);
    if (page.nextCursor) expect((await api.movements.list(org.id, {}, page.nextCursor)).items[0]?.id).not.toBe(first.id);
    const payins = await api.movements.list(org.id, { type: 'payin', currency: 'ARS' }, null);
    expect(payins.items.every((item) => item.type === 'payin' && item.currency === 'ARS' && item.direction === 'in')).toBe(true);
    await expect(api.movements.get(org.id, randomUUID())).rejects.toMatchObject({ code: 'movement.not_found' });
  });

  it('reads reconciliation runs with their items, KYC and the ERP link state', async () => {
    const runs = await api.reconciliation.runs(org.id);
    expect(runs.length).toBeGreaterThan(0);
    const run = runs[0];
    expect(run?.difference).toBe((run?.pspTotal ?? 0) - (run?.ledgerTotal ?? 0));
    const items = await api.reconciliation.items(org.id, run?.id ?? '');
    expect(items.filter((item) => item.status === 'matched')).toHaveLength(run?.matched ?? -1);
    expect((await api.kyc.get(org.id)).company?.representatives.length).toBeGreaterThan(0);
    expect((await api.erp.candidate(org.id)).found).toBe(true);
    expect(['not_linked', 'importing', 'linked']).toContain((await api.erp.get(org.id)).status);
  });

  it('opens the realtime stream of the organization', async () => {
    unsubscribe = api.events.subscribe(org.id, (event) => events.push(event));
    // The stream is up once the server sends its first bytes; a short wait keeps the order.
    await new Promise((resolve) => setTimeout(resolve, 500));
  });

  it('resolves the destination, quotes and creates a payout with a password step-up', async () => {
    const destination = await api.payouts.resolveDestination(org.id, DESTINATION);
    expect(destination).toMatchObject({ type: 'alias', value: DESTINATION });
    expect(destination.accountNumber).toMatch(/^\d{22}$/);
    const quote = await api.payouts.quote(org.id, { accountId, currency: 'ARS', amount: 1_000_00, to: DESTINATION });
    expect(quote.total).toBe(quote.amount + quote.fee);
    expect(quote.availableAfter).toBeGreaterThanOrEqual(0);
    const body = { accountId, currency: 'ARS' as const, amount: 1_000_00, to: DESTINATION, concept: `Prueba live ${new Date().toISOString().slice(0, 19)}` };
    const idempotencyKey = randomUUID();
    payout = await api.payouts.create(org.id, body, { idempotencyKey, stepUpToken: await stepUpWithPassword(api) });
    expect(payout).toMatchObject({ status: 'pending_approval', amount: 1_000_00, currency: 'ARS', accountName: 'Torre Palermo' });
    expect(payout.createdBy.label).toBe(DEMO);
    // Same key + same body replays the payout instead of paying twice (R-OUT-3).
    expect((await api.payouts.create(org.id, body, { idempotencyKey, stepUpToken: await stepUpWithPassword(api) })).id).toBe(payout.id);
    await expect(api.payouts.create(org.id, { ...body, amount: 1_000_01 }, { idempotencyKey, stepUpToken: await stepUpWithPassword(api) })).rejects.toMatchObject({ code: 'idempotency.key_reused' });
  });

  it('receives approval.requested for the new payout over SSE', async () => {
    const event = await waitFor(events, (item) => item.type === 'approval.requested' && item.payout.id === payout.id);
    expect(event).toMatchObject({ type: 'approval.requested', payout: { id: payout.id, status: 'pending_approval', accountName: 'Torre Palermo' } });
  });

  it('refuses the creator as approver', async () => {
    await expect(api.payouts.approve(org.id, payout.id, { idempotencyKey: randomUUID(), stepUpToken: await stepUpWithPassword(api) })).rejects.toMatchObject({
      code: 'payout.self_approval_forbidden',
    });
  });

  it('lets the second person approve with a device-key step-up', async () => {
    await signIn(api, APPROVER);
    const approvals = await api.payouts.approvals(org.id);
    expect(approvals.map((item) => item.id)).toContain(payout.id);
    const pair = generateDeviceKey();
    const device = await api.auth.registerDevice({ publicKey: pair.publicKey, name: 'lenapay-live-test' });
    const token = await verifyStepUp({ method: 'device_key' }, { auth: api.auth, readDeviceKey: async () => ({ deviceId: device.id, secretKey: pair.secretKey }) });
    const approved = await api.payouts.approve(org.id, payout.id, { idempotencyKey: randomUUID(), stepUpToken: token.stepUpToken });
    expect(approved.status).toBe('approved');
    expect(approved.approvedBy?.label).toBe(APPROVER);
    // A forged signature never passes: the challenge burns and the app falls back to the password.
    const forged = { deviceId: device.id, secretKey: generateDeviceKey().secretKey };
    await expect(verifyStepUp({ method: 'device_key' }, { auth: api.auth, readDeviceKey: async () => forged })).rejects.toMatchObject({ code: 'stepup.device_key_unavailable' });
  });

  it('receives payout.updated as the approved payout moves on', async () => {
    const event = await waitFor(events, (item) => item.type === 'payout.updated' && item.payout.id === payout.id && item.payout.status !== 'pending_approval');
    expect(['approved', 'sending', 'settled']).toContain(event.type === 'payout.updated' ? event.payout.status : '');
  });
});
