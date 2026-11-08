import { randomUUID } from 'node:crypto';

import { afterAll, beforeAll, describe, expect, it, vi } from 'vitest';

import { saveTokens } from '@/shared/session/session';
import { verifyStepUp } from '@/shared/stepup/stepUpFlow';

import type { LenapayApi } from '../../client';
import type { Org } from '../../types';
import { createLiveApi } from '../liveApi';

/**
 * O8 against the running stack (`pnpm test:live`): owner role and ladder, ownership transfer
 * guards, the approval policy (read by everyone, written by the owner with step-up), owner
 * self-approval, profile name and synced preferences, and the ERP import summary.
 * Seed users from the README; every change made here is put back at the end.
 */
const BASE_URL = process.env.LIVE_API_URL ?? 'http://localhost:18080';
const PASSWORD = 'Lenapay2026!';
const DEMO = 'demo@lenapay.app';
const APPROVER = 'approver@lenapay.app';
const ANDINA = 'andina@lenapay.app';
const DEMO_NAME = 'Ricardo Benítez';
const DESTINATION = 'hormigonera.sur';

vi.mock('react-native-sse', () => ({ default: class {} }));
vi.mock('expo-file-system', () => ({ File: class {}, Paths: { cache: '' } }));

async function signIn(api: LenapayApi, email: string): Promise<void> {
  await saveTokens(await api.auth.login({ email, password: PASSWORD }));
}

function stepUp(api: LenapayApi): Promise<string> {
  return verifyStepUp({ method: 'password', password: PASSWORD }, { auth: api.auth, readDeviceKey: async () => null }).then((token) => token.stepUpToken);
}

async function companyOf(api: LenapayApi): Promise<Org> {
  const org = (await api.orgs.list()).find((item) => item.kind === 'company');
  if (!org) throw new Error('no company org');
  return org;
}

describe(`owner, policy and preferences against ${BASE_URL}`, () => {
  const api = createLiveApi({ baseUrl: BASE_URL });
  let dds: Org;

  beforeAll(async () => {
    const health = await fetch(`${BASE_URL}/healthz`).catch(() => null);
    if (!health?.ok) throw new Error(`API not reachable at ${BASE_URL}: start the stack (docker compose up -d --wait) and seed it`);
    await signIn(api, DEMO);
    dds = await companyOf(api);
  });

  afterAll(async () => {
    await signIn(api, DEMO);
    await api.profile.update({ fullName: DEMO_NAME });
    await api.profile.updatePreferences({ defaultOrgId: null, defaultAccountId: null, locale: null, theme: null });
  });

  it('reads the policy as any member and refuses a write by a non-owner', async () => {
    await signIn(api, APPROVER);
    const policy = await api.orgs.approvalPolicy(dds.id);
    // With an approver on the team, the owner's self-approval does not apply.
    expect(policy).toMatchObject({ mode: 'threshold', thresholdMinor: { ARS: 500_000_00 }, ownerSelfApprovalEffective: false });
    const body = { mode: 'always' as const, allowOwnerSelfApproval: true };
    await expect(api.orgs.updateApprovalPolicy(dds.id, body, { stepUpToken: await stepUp(api) })).rejects.toMatchObject({ code: 'auth.forbidden' });
  });

  it('lets the owner save the policy with step-up (same values back)', async () => {
    await signIn(api, DEMO);
    const saved = await api.orgs.updateApprovalPolicy(dds.id, { mode: 'threshold', thresholdMinor: { ARS: 500_000_00 }, allowOwnerSelfApproval: true }, { stepUpToken: await stepUp(api) });
    expect(saved).toMatchObject({ mode: 'threshold', thresholdMinor: { ARS: 500_000_00, USD: null }, allowOwnerSelfApproval: true });
    await expect(api.orgs.updateApprovalPolicy(dds.id, { mode: 'threshold', allowOwnerSelfApproval: true }, { stepUpToken: await stepUp(api) })).rejects.toMatchObject({
      code: 'validation.failed',
    });
  });

  it('guards the ownership transfer and the member ladder', async () => {
    await expect(api.orgs.transferOwnership(dds.id, { userId: randomUUID() }, { stepUpToken: await stepUp(api) })).rejects.toMatchObject({ code: 'member.not_found' });
    const me = await api.auth.me();
    await expect(api.orgs.transferOwnership(dds.id, { userId: me.id }, { stepUpToken: await stepUp(api) })).rejects.toMatchObject({ code: 'validation.failed' });
    await expect(api.orgs.updateMember(dds.id, me.id, { role: 'admin' })).rejects.toMatchObject({ code: 'org.last_admin' });
    await signIn(api, APPROVER);
    await expect(api.orgs.transferOwnership(dds.id, { userId: me.id }, { stepUpToken: await stepUp(api) })).rejects.toMatchObject({ code: 'auth.forbidden' });
  });

  it('edits the full name and the synced preferences (default org + account, theme, locale)', async () => {
    await signIn(api, DEMO);
    expect((await api.profile.update({ fullName: '  Ricardo B.  ' })).fullName).toBe('Ricardo B.');
    const account = (await api.accounts.list(dds.id)).find((item) => item.name === 'Torre Palermo');
    const preferences = await api.profile.updatePreferences({ defaultOrgId: dds.id, defaultAccountId: account?.id ?? null, theme: 'dark', locale: 'en-US' });
    expect(preferences).toMatchObject({ defaultOrgId: dds.id, defaultAccountId: account?.id, theme: 'dark', locale: 'en-US' });
    expect((await api.auth.me()).preferences).toMatchObject({ defaultOrgId: dds.id, theme: 'dark' });
    await expect(api.profile.updatePreferences({ defaultOrgId: randomUUID() })).rejects.toMatchObject({ code: 'org.not_found' });
    expect(await api.profile.preferences()).toMatchObject({ defaultOrgId: dds.id, locale: 'en-US' });
  });

  it('shows the ERP import summary and lets a lone owner self-approve (Andina)', async () => {
    await signIn(api, ANDINA);
    const andina = await companyOf(api);
    expect(andina.role).toBe('owner');
    const link = await api.erp.get(andina.id);
    expect(link.status).toBe('linked');
    expect(link.importSummary?.invitedUsers.length).toBeGreaterThan(0);
    expect(link.importSummary?.usersWithoutPaymentsAccess).toBeGreaterThanOrEqual(0);
    const policy = await api.orgs.approvalPolicy(andina.id);
    expect(policy).toMatchObject({ mode: 'always', ownerSelfApprovalEffective: true });
    const account = (await api.accounts.list(andina.id)).find((item) => item.balances.some((balance) => balance.currency === 'ARS' && balance.available > 2_000_00));
    if (!account) throw new Error('no funded Andina account');
    const body = { accountId: account.id, currency: 'ARS' as const, amount: 1_000_00, to: DESTINATION, concept: `Live aprobación propia ${new Date().toISOString().slice(0, 19)}` };
    const payout = await api.payouts.create(andina.id, body, { idempotencyKey: randomUUID(), stepUpToken: await stepUp(api) });
    expect(payout.status).toBe('pending_approval');
    const approved = await api.payouts.approve(andina.id, payout.id, { idempotencyKey: randomUUID(), stepUpToken: await stepUp(api) });
    expect(approved.status).toBe('approved');
    expect(approved.approvedBy?.userId).toBe(approved.createdBy.userId);
  });
});
