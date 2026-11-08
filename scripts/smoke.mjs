#!/usr/bin/env node
// End-to-end smoke test of the running stack (docker compose up), no dependencies (Node 18+).
// It walks the whole money path through the public API and the bind-mock's control endpoints:
// signup → verify → login → org → KYC (dev auto-approval) → payer + CVU → payin through the
// bind-mock (signed webhook, worker credits the ledger) → balance + movement + SSE event →
// quote + payout → a second user approves → dispatch settles → attachment + proof PDF →
// reconciliation run → "Mis pagos" (a verified person sees the company that registered their
// CUIT, its payins, hides/shows it and pays it on-us from the personal wallet: personal org
// and account at signup, personal CVUs on the account, alias change), and the owner decisions
// (approval policy threshold with the attacker case, ownership transfer, profile and preferences,
// the seeded ERP users import with its auto-accepted invites). Prints a PASS/FAIL table; exit
// code 1 when any step fails. The ERP step needs the seed (`make seed`).
//
//   node scripts/smoke.mjs
//   LENAPAY_API_URL=http://localhost:18080 BINDMOCK_URL=http://localhost:8090 node scripts/smoke.mjs
//   (also DEV_FIXED_EMAIL_CODE, SEED_DEMO_PASSWORD and SMOKE_PASSWORD; each has the demo default)

const API = (process.env.LENAPAY_API_URL ?? 'http://localhost:18080').replace(/\/$/, '');
const BANK = (process.env.BINDMOCK_URL ?? 'http://localhost:8090').replace(/\/$/, '');
const CODE = process.env.DEV_FIXED_EMAIL_CODE ?? '123456';
const run = Date.now().toString(36) + Math.floor(Math.random() * 1e6).toString(36);
// Password of the users the smoke signs up. It must stay the same across runs: the invited ERP
// user (contable@constructoraandina.com.ar) signs up once and logs in on later runs.
const PASSWORD = process.env.SMOKE_PASSWORD ?? 'Smoke-test-2026!';
// The seeded users' password (SEED_DEMO_PASSWORD, the published demo default otherwise).
const DEMO_PASSWORD = process.env.SEED_DEMO_PASSWORD ?? 'Lenapay2026!';
const results = [];
const ctx = {};

class StepError extends Error {}

function check(cond, message) {
  if (!cond) throw new StepError(message);
}

async function call(method, path, { token, body, headers = {}, form, raw = false, base = API } = {}) {
  const h = { ...headers };
  if (token) h.Authorization = `Bearer ${token}`;
  let payload;
  if (form) payload = form;
  else if (body !== undefined) {
    h['Content-Type'] = 'application/json';
    payload = JSON.stringify(body);
  }
  const res = await fetch(base + path, { method, headers: h, body: payload });
  if (raw) return res;
  const text = await res.text();
  let json = null;
  try {
    json = text ? JSON.parse(text) : null;
  } catch {
    json = { raw: text.slice(0, 200) };
  }
  return { status: res.status, json };
}

async function must(method, path, status, opts) {
  const r = await call(method, path, opts);
  check(r.status === status, `${method} ${path} → ${r.status} (want ${status}) ${JSON.stringify(r.json)?.slice(0, 300)}`);
  return r.json;
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function waitFor(what, fn, timeoutMs = 60_000) {
  const end = Date.now() + timeoutMs;
  for (;;) {
    const v = await fn();
    if (v) return v;
    if (Date.now() > end) throw new StepError(`timed out waiting for ${what}`);
    await sleep(500);
  }
}

async function step(name, fn) {
  const start = Date.now();
  try {
    const detail = await fn();
    results.push({ step: name, result: 'PASS', ms: Date.now() - start, detail: detail ?? '' });
    return true;
  } catch (err) {
    results.push({ step: name, result: 'FAIL', ms: Date.now() - start, detail: String(err.message ?? err).slice(0, 160) });
    return false;
  }
}

async function stepUp(token) {
  const ch = await must('POST', '/auth/step-up/challenge', 201, { token, body: {} });
  const t = await must('POST', '/auth/step-up/verify', 200, { token, body: { challengeId: ch.challengeId, method: 'password', password: PASSWORD } });
  return t.stepUpToken;
}

async function newUser(email) {
  await must('POST', '/auth/signup', 201, { body: { email, password: PASSWORD } });
  await must('POST', '/auth/verify-email', 204, { body: { email, code: CODE } });
  const tokens = await must('POST', '/auth/login', 200, { body: { email, password: PASSWORD } });
  return tokens.accessToken;
}

const key = (label) => `smoke-${label}-${run}`.slice(0, 120);
const pdfBlob = (text) => new Blob([`%PDF-1.7\n${text}\n%%EOF`], { type: 'application/pdf' });

// SSE reader: collects every frame of the org stream in ctx.events (unnamed frames, JSON data).
async function openEvents(token, orgId) {
  const controller = new AbortController();
  const res = await fetch(`${API}/orgs/${orgId}/events`, { headers: { Authorization: `Bearer ${token}` }, signal: controller.signal });
  check(res.status === 200, `GET events → ${res.status}`);
  check((res.headers.get('content-type') ?? '').startsWith('text/event-stream'), 'events content-type');
  const events = [];
  (async () => {
    const decoder = new TextDecoder();
    let buf = '';
    try {
      for await (const chunk of res.body) {
        buf += decoder.decode(chunk, { stream: true });
        let i;
        while ((i = buf.indexOf('\n\n')) >= 0) {
          const frame = buf.slice(0, i);
          buf = buf.slice(i + 2);
          const named = frame.split('\n').some((l) => l.startsWith('event:'));
          const data = frame.split('\n').filter((l) => l.startsWith('data:')).map((l) => l.slice(5).trim()).join('\n');
          if (data) events.push({ named, ...JSON.parse(data) });
        }
      }
    } catch {
      /* aborted at the end */
    }
  })();
  return { events, close: () => controller.abort() };
}

function today() {
  // Business day in America/Argentina/Buenos_Aires (UTC-3, no DST).
  return new Date(Date.now() - 3 * 3600_000).toISOString().slice(0, 10);
}

async function main() {
  const email = `smoke-${run}@example.com`;
  const approverEmail = `smoke-approver-${run}@example.com`;

  const ok = await step('health', async () => {
    await must('GET', '/healthz', 200);
    const bank = await call('GET', '/healthz', { base: BANK });
    check(bank.status === 200, `bind-mock healthz ${bank.status}`);
  });
  if (!ok) return;

  await step('signup → verify (fixed code) → login', async () => {
    await must('POST', '/auth/signup', 201, { body: { email, password: PASSWORD } });
    await must('POST', '/auth/resend-code', 202, { body: { email } });
    await must('POST', '/auth/verify-email', 204, { body: { email, code: CODE } });
    const t = await must('POST', '/auth/login', 200, { body: { email, password: PASSWORD } });
    ctx.token = t.accessToken;
    return email;
  });

  await step('create company org', async () => {
    ctx.cuit = '30' + String(Math.floor(Math.random() * 1e8)).padStart(8, '0') + '9';
    const org = await must('POST', '/orgs', 201, { token: ctx.token, body: { kind: 'company', legalName: `Smoke ${run} SA` } });
    check(org.cuit === null && org.kycStatus === 'draft', `org ${JSON.stringify(org)}`);
    ctx.org = org.id;
    ctx.orgPath = `/orgs/${org.id}`;
    return org.id;
  });

  await step('KYC submit → dev auto-approval', async () => {
    const dni = '20111222';
    await must('PUT', `${ctx.orgPath}/kyc`, 200, {
      token: ctx.token,
      body: {
        company: {
          cuit: ctx.cuit, legalName: `Smoke ${run} Verificada SA`,
          legalAddress: { street: 'Mitre', number: '50', city: 'Rosario', province: 'Santa Fe', postalCode: '2000' },
          representatives: [{ fullName: 'Carla Díaz', dni, position: 'Directora' }],
        },
      },
    });
    const docs = [{ kind: 'bylaws' }, { kind: 'afip_constancy' }, { kind: 'dni_front', rep: dni }, { kind: 'dni_back', rep: dni }, { kind: 'selfie', rep: dni }];
    for (const d of docs) {
      const form = new FormData();
      form.append('kind', d.kind);
      if (d.rep) form.append('representativeDni', d.rep);
      form.append('file', pdfBlob(d.kind), `${d.kind}.pdf`);
      await must('POST', `${ctx.orgPath}/kyc/documents`, 201, { token: ctx.token, form });
    }
    await must('POST', `${ctx.orgPath}/kyc/submit`, 200, { token: ctx.token });
    const c = await waitFor('KYC approval', async () => {
      const k = await must('GET', `${ctx.orgPath}/kyc`, 200, { token: ctx.token });
      return k.status === 'approved' ? k : null;
    });
    const org = await must('GET', ctx.orgPath, 200, { token: ctx.token });
    check(org.cuit === ctx.cuit && org.legalName === `Smoke ${run} Verificada SA`, `verified identity not on org ${JSON.stringify(org)}`);
    return `approved ${c.decidedAt}`;
  });

  await step('create payer + ARS CVU (step-up)', async () => {
    const accounts = await must('GET', `${ctx.orgPath}/accounts`, 200, { token: ctx.token });
    const main = accounts.items.find((a) => a.kind === 'main');
    check(main, 'main account opened at approval');
    ctx.account = main.id;
    const payer = await must('POST', `${ctx.orgPath}/payers`, 201, {
      token: ctx.token, body: { accountId: main.id, name: 'Comprador Smoke', docType: 'dni', docNumber: '30111222', email: 'comprador@example.com' },
    });
    const cleared = await must('PATCH', `${ctx.orgPath}/payers/${payer.id}`, 200, { token: ctx.token, body: { email: null } });
    check(cleared.email === null, 'email: null clears the payer email');
    ctx.stepUp = await stepUp(ctx.token);
    const cvu = await must('POST', `${ctx.orgPath}/payers/${payer.id}/cvus`, 201, {
      token: ctx.token, body: { currency: 'ARS' }, headers: { 'Idempotency-Key': key('cvu'), 'X-Step-Up-Token': ctx.stepUp },
    });
    ctx.cvu = cvu.cvu;
    return cvu.cvu;
  });

  await step('payin via bind-mock → balance + movement + SSE', async () => {
    ctx.stream = await openEvents(ctx.token, ctx.org);
    await sleep(300);
    const r = await call('POST', '/_mock/incoming-transfer', {
      base: BANK, body: { cvu: ctx.cvu, amount: '250000.00', currency: 'ARS', originName: 'Comprador Smoke', originCuit: '20301112229' },
    });
    check(r.status === 201, `incoming-transfer → ${r.status} ${JSON.stringify(r.json)}`);
    const ev = await waitFor('payin.credited event', async () => ctx.stream.events.find((e) => e.type === 'payin.credited'), 30_000);
    check(!ev.named && ev.orgId === ctx.org && ev.data?.payinId && ev.payin?.id === ev.data.payinId, `event framing ${JSON.stringify(ev).slice(0, 200)}`);
    const balances = await must('GET', `${ctx.orgPath}/balances`, 200, { token: ctx.token });
    const ars = balances.totals.find((t) => t.currency === 'ARS');
    check(ars && ars.available.minor > 0, `balance ${JSON.stringify(balances.totals)}`);
    const list = await must('GET', `${ctx.orgPath}/movements?type=payin`, 200, { token: ctx.token });
    check(list.items.length === 1 && list.items[0].status === 'completed', `movements ${JSON.stringify(list.items)}`);
    const detail = await must('GET', `${ctx.orgPath}/movements/${list.items[0].id}`, 200, { token: ctx.token });
    check(detail.refId === ev.data.payinId && 'concept' in detail, 'movement detail');
    ctx.available = ars.available.minor;
    return `available ARS ${ars.available.minor / 100}`;
  });

  await step('quote + create payout (pending approval)', async () => {
    const amount = { minor: 10_000_000, currency: 'ARS' };
    const q = await must('POST', `${ctx.orgPath}/payouts/quote`, 200, { token: ctx.token, body: { accountId: ctx.account, amount, to: 'proveedor.smoke' } });
    check(q.total.minor === amount.minor + q.fee.minor && q.availableAfter.minor === ctx.available - q.total.minor, `quote ${JSON.stringify(q)}`);
    const po = await must('POST', `${ctx.orgPath}/payouts`, 201, {
      token: ctx.token, body: { accountId: ctx.account, amount, to: 'proveedor.smoke', concept: 'Pago smoke' },
      headers: { 'Idempotency-Key': key('payout'), 'X-Step-Up-Token': ctx.stepUp },
    });
    check(po.status === 'pending_approval' && Array.isArray(po.attachments) && po.approvedBy === null, `payout ${JSON.stringify(po).slice(0, 200)}`);
    ctx.payout = po.id;
    return `fee ARS ${q.fee.minor / 100}`;
  });

  await step('second user approves (4-eyes)', async () => {
    ctx.approverToken = await newUser(approverEmail);
    await must('POST', `${ctx.orgPath}/members`, 201, { token: ctx.token, body: { email: approverEmail, role: 'approver' } });
    const self = await call('POST', `${ctx.orgPath}/payouts/${ctx.payout}/approve`, {
      token: ctx.token, headers: { 'Idempotency-Key': key('self-approve'), 'X-Step-Up-Token': ctx.stepUp },
    });
    check(self.status === 403 && self.json?.error?.code === 'payout.self_approval_forbidden', `self approval ${self.status}`);
    const su = await stepUp(ctx.approverToken);
    const po = await must('POST', `${ctx.orgPath}/payouts/${ctx.payout}/approve`, 200, {
      token: ctx.approverToken, headers: { 'Idempotency-Key': key('approve'), 'X-Step-Up-Token': su },
    });
    check(po.status === 'approved' || po.status === 'sending' || po.status === 'settled', `approved ${po.status}`);
  });

  await step('dispatch settles + payout.updated SSE', async () => {
    const po = await waitFor('payout settled', async () => {
      const p = await must('GET', `${ctx.orgPath}/payouts/${ctx.payout}`, 200, { token: ctx.token });
      return p.status === 'settled' ? p : null;
    });
    const ev = await waitFor('payout.updated settled event', async () =>
      ctx.stream.events.find((e) => e.type === 'payout.updated' && e.payout?.status === 'settled'), 10_000);
    check(ev.data.payoutId === ctx.payout, 'event payout id');
    return `settled ${po.settledAt}`;
  });

  await step('attachment + proof PDF', async () => {
    const form = new FormData();
    form.append('file', pdfBlob('factura'), 'factura.pdf');
    const a = await must('POST', `${ctx.orgPath}/payouts/${ctx.payout}/attachments`, 201, { token: ctx.token, form });
    check(a.contentType === 'application/pdf' && a.fileName === 'factura.pdf', `attachment ${JSON.stringify(a)}`);
    const po = await must('GET', `${ctx.orgPath}/payouts/${ctx.payout}`, 200, { token: ctx.token });
    check(po.attachments.length === 1, 'payout lists the attachment');
    const res = await call('GET', `${ctx.orgPath}/payouts/${ctx.payout}/proof.pdf`, { token: ctx.token, raw: true });
    check(res.status === 200 && res.headers.get('content-type') === 'application/pdf', `proof ${res.status}`);
    const bytes = new Uint8Array(await res.arrayBuffer());
    check(new TextDecoder().decode(bytes.slice(0, 5)) === '%PDF-', 'proof is a PDF');
    return `${bytes.length} bytes`;
  });

  await step('reconciliation run + items', async () => {
    const run = await must('POST', `${ctx.orgPath}/reconciliation/runs`, 202, { token: ctx.token, body: { currency: 'ARS', day: today() } });
    const done = await waitFor('run finished', async () => {
      const list = await must('GET', `${ctx.orgPath}/reconciliation/runs`, 200, { token: ctx.token });
      const r = list.items.find((x) => x.id === run.id);
      return r && r.finishedAt ? r : null;
    });
    const items = await must('GET', `${ctx.orgPath}/reconciliation/runs/${run.id}/items`, 200, { token: ctx.token });
    check(done.status === 'completed' && done.matchedCount === 2 && items.items.length === done.matchedCount + done.missingInLedgerCount + done.missingAtPspCount,
      `run ${JSON.stringify(done)} items ${items.items.length}`);
    return `matched ${done.matchedCount}, diff ${done.difference.minor}`;
  });

  await step('owner: approval policy threshold (attacker 403)', async () => {
    const me = await must('GET', '/me', 200, { token: ctx.token });
    check(me.memberships.find((m) => m.orgId === ctx.org)?.role === 'owner' && 'preferences' in me, `the creator is the owner ${JSON.stringify(me.memberships)}`);
    const policyPath = `${ctx.orgPath}/approval-policy`;
    const threshold = { mode: 'threshold', thresholdMinor: { ARS: 100_001 }, allowOwnerSelfApproval: true };
    const attacker = await call('PUT', policyPath, { token: ctx.approverToken, body: threshold, headers: { 'X-Step-Up-Token': await stepUp(ctx.approverToken) } });
    check(attacker.status === 403 && attacker.json?.error?.code === 'auth.forbidden', `an approver changing the policy → ${attacker.status}`);
    const policy = await must('PUT', policyPath, 200, { token: ctx.token, body: threshold, headers: { 'X-Step-Up-Token': await stepUp(ctx.token) } });
    check(policy.mode === 'threshold' && policy.thresholdMinor.ARS === 100_001 && policy.thresholdMinor.USD === null && policy.ownerSelfApprovalEffective === false,
      `policy ${JSON.stringify(policy)}`);
    const su = await stepUp(ctx.token);
    const pay = (minor, label) => must('POST', `${ctx.orgPath}/payouts`, 201, {
      token: ctx.token, body: { accountId: ctx.account, amount: { minor, currency: 'ARS' }, to: 'proveedor.smoke', concept: `Umbral ${label}` },
      headers: { 'Idempotency-Key': key(`threshold-${label}`), 'X-Step-Up-Token': su },
    });
    const below = await pay(100_000, 'below');
    check(below.status !== 'pending_approval' && below.events[0]?.reason === 'policy_threshold', `below the threshold ${JSON.stringify(below).slice(0, 200)}`);
    const at = await pay(100_001, 'at');
    check(at.status === 'pending_approval', `at the threshold ${at.status}`);
    await must('POST', `${ctx.orgPath}/payouts/${at.id}/cancel`, 200, { token: ctx.token });
    await waitFor('payout below the threshold settled', async () => {
      const p = await must('GET', `${ctx.orgPath}/payouts/${below.id}`, 200, { token: ctx.token });
      return p.status === 'settled';
    });
    await must('PUT', policyPath, 200, { token: ctx.token, body: { mode: 'always', allowOwnerSelfApproval: true }, headers: { 'X-Step-Up-Token': su } });
    return 'ARS 1000.00 approved at creation and settled; ARS 1000.01 waited';
  });

  await step('owner: ownership transfer and back', async () => {
    const approverId = (await must('GET', '/me', 200, { token: ctx.approverToken })).user.id;
    const ownerId = (await must('GET', '/me', 200, { token: ctx.token })).user.id;
    const transfer = `${ctx.orgPath}/ownership-transfer`;
    const notOwner = await call('POST', transfer, { token: ctx.approverToken, body: { userId: approverId }, headers: { 'X-Step-Up-Token': await stepUp(ctx.approverToken) } });
    check(notOwner.status === 403 && notOwner.json?.error?.code === 'auth.forbidden', `a non-owner transfers → ${notOwner.status}`);
    const moved = await must('POST', transfer, 200, { token: ctx.token, body: { userId: approverId }, headers: { 'X-Step-Up-Token': await stepUp(ctx.token) } });
    check(moved.owner.userId === approverId && moved.owner.role === 'owner' && moved.previousOwner.role === 'admin', `transfer ${JSON.stringify(moved)}`);
    const removeOwner = await call('DELETE', `${ctx.orgPath}/members/${approverId}`, { token: ctx.token });
    check(removeOwner.status === 403 && removeOwner.json?.error?.code === 'role.escalation_forbidden', `an admin removes the owner → ${removeOwner.status}`);
    const back = await must('POST', transfer, 200, { token: ctx.approverToken, body: { userId: ownerId }, headers: { 'X-Step-Up-Token': await stepUp(ctx.approverToken) } });
    check(back.owner.userId === ownerId, `transfer back ${JSON.stringify(back)}`);
    await must('PATCH', `${ctx.orgPath}/members/${approverId}`, 200, { token: ctx.token, body: { role: 'approver' } });
    return 'owner → approver → owner';
  });

  await step('profile + preferences', async () => {
    const named = await must('PATCH', '/me', 200, { token: ctx.token, body: { fullName: '  Smoke Owner  ' } });
    check(named.user.fullName === 'Smoke Owner', `fullName ${named.user.fullName}`);
    const prefs = await must('PATCH', '/me/preferences', 200, { token: ctx.token, body: { defaultOrgId: ctx.org, defaultAccountId: ctx.account, locale: 'es-AR', theme: 'dark' } });
    check(prefs.defaultOrgId === ctx.org && prefs.defaultAccountId === ctx.account && prefs.theme === 'dark', `preferences ${JSON.stringify(prefs)}`);
    const merged = await call('PATCH', '/me/preferences', { token: ctx.token, body: { defaultOrgId: null } });
    check(merged.status === 422 && merged.json?.error?.code === 'validation.failed', `an account without its org → ${merged.status}`);
    const me = await must('GET', '/me', 200, { token: ctx.token });
    check(me.preferences.defaultAccountId === ctx.account && me.user.fullName === 'Smoke Owner', `me ${JSON.stringify(me.preferences)}`);
    const members = await must('GET', `${ctx.orgPath}/members`, 200, { token: ctx.approverToken });
    check(members.items.some((m) => m.role === 'owner' && m.fullName === 'Smoke Owner'), 'members list the owner by name');
  });

  await step('ERP users import (seeded Andina) + auto-accept', async () => {
    const login = await call('POST', '/auth/login', { body: { email: 'andina@lenapay.app', password: DEMO_PASSWORD } });
    check(login.status === 200, `andina@lenapay.app cannot log in (${login.status}): run the seed first`);
    const token = login.json.accessToken;
    const list = await must('GET', '/orgs', 200, { token });
    const andina = list.items.find((o) => o.legalName === 'Constructora Andina SRL');
    check(andina && andina.role === 'owner', `Andina ${JSON.stringify(andina)}`);
    const link = await must('GET', `/orgs/${andina.id}/erp-link`, 200, { token });
    const sum = link.importSummary;
    check(sum && sum.usersInvited >= 3 && sum.usersWithoutPaymentsAccess >= 1 && sum.invitedUsers.some((u) => u.email === 'contable@constructoraandina.com.ar' && u.role === 'viewer'),
      `import summary ${JSON.stringify(sum)}`);
    // An invited ERP user joins by verifying the email (first run) or logging in (later runs).
    const email = 'contable@constructoraandina.com.ar';
    const signup = await call('POST', '/auth/signup', { body: { email, password: PASSWORD, fullName: 'Lorena Videla' } });
    if (signup.status === 201) await must('POST', '/auth/verify-email', 204, { body: { email, code: CODE } });
    else check(signup.status === 409, `signup ${signup.status}`);
    const invitee = await must('POST', '/auth/login', 200, { body: { email, password: PASSWORD } });
    const joined = await must('GET', '/orgs', 200, { token: invitee.accessToken });
    check(joined.items.some((o) => o.id === andina.id && o.role === 'viewer'), `invitee orgs ${JSON.stringify(joined.items)}`);
    const members = await must('GET', `/orgs/${andina.id}/members`, 200, { token });
    check(members.items.some((m) => m.fullName === 'Paula Sosa' && m.role === 'operator'), 'the seeded invitee joined as operator');
    return `invited ${sum.usersInvited}, members skipped ${sum.usersAlreadyMembers}, without payments access ${sum.usersWithoutPaymentsAccess}`;
  });

  await step('Mis pagos: person sees + pays the company', async () => {
    const companyName = `Smoke ${run} Verificada SA`;
    const personName = `Persona Smoke ${run}`;
    const dni = String(30_000_000 + Math.floor(Math.random() * 9_000_000));
    const token = await newUser(`smoke-persona-${run}@example.com`);
    // Signup created her personal org (KYC draft) with its personal account and no CVUs yet.
    const orgs = await must('GET', '/orgs', 200, { token });
    const org = orgs.items.find((o) => o.kind === 'personal');
    check(org && org.kycStatus === 'draft', `personal org at signup ${JSON.stringify(orgs.items)}`);
    const orgPath = `/orgs/${org.id}`;
    const draft = await must('GET', `${orgPath}/accounts`, 200, { token });
    check(draft.items.length === 1 && draft.items[0].kind === 'personal' && draft.items[0].cvus.length === 0, `draft accounts ${JSON.stringify(draft)}`);
    await must('PUT', `${orgPath}/kyc`, 200, {
      token, body: { person: { fullName: personName, dni, birthDate: '1990-01-02', address: { street: 'Gorriti', number: '4500', city: 'CABA', province: 'CABA', postalCode: 'C1414' } } },
    });
    for (const kind of ['dni_front', 'dni_back', 'selfie']) {
      const form = new FormData();
      form.append('kind', kind);
      form.append('file', pdfBlob(kind), `${kind}.pdf`);
      await must('POST', `${orgPath}/kyc/documents`, 201, { token, form });
    }
    const kyc = await must('POST', `${orgPath}/kyc/submit`, 200, { token });
    check(kyc.status === 'approved', `person KYC ${kyc.status}`);

    // The company registers the person by a CUIT around the verified DNI, with a CVU and a payin.
    const payer = await must('POST', `${ctx.orgPath}/payers`, 201, {
      token: ctx.token, body: { accountId: ctx.account, name: personName, docType: 'cuit', docNumber: `27${dni}4` },
    });
    const cvu = await must('POST', `${ctx.orgPath}/payers/${payer.id}/cvus`, 201, {
      token: ctx.token, body: { currency: 'ARS' }, headers: { 'Idempotency-Key': key('mp-cvu'), 'X-Step-Up-Token': await stepUp(ctx.token) },
    });
    const sent = await call('POST', '/_mock/incoming-transfer', { base: BANK, body: { cvu: cvu.cvu, amount: '120000.00', currency: 'ARS', originName: personName } });
    check(sent.status === 201, `incoming-transfer → ${sent.status}`);

    const company = await waitFor('company with the payin', async () => {
      const list = await must('GET', '/me/payments/companies', 200, { token });
      const c = list.items.find((x) => x.companyName === companyName);
      return c && c.totalPaid.some((t) => t.currency === 'ARS' && t.minor === 12_000_000) ? c : null;
    }, 30_000);
    check(Object.keys(company).sort().join() === 'companyName,cvus,erpLinked,hidden,lastPaymentAt,linkId,projectName,totalPaid', `company keys ${Object.keys(company)}`);
    check(company.cvus.length === 1 && company.cvus[0].cvu === cvu.cvu && company.erpLinked === false, `company ${JSON.stringify(company)}`);
    const linkPath = `/me/payments/companies/${company.linkId}`;
    const payins = await must('GET', `${linkPath}/payins`, 200, { token });
    check(payins.items.length === 1 && payins.items[0].status === 'credited' && payins.items[0].cvu === cvu.cvu, `payins ${JSON.stringify(payins)}`);
    const plan = await must('GET', `${linkPath}/installments`, 200, { token });
    check(plan.erpLinked === false && plan.items.length === 0, `installments ${JSON.stringify(plan)}`);
    const hidden = await must('PATCH', linkPath, 200, { token, body: { hidden: true } });
    const visible = await must('GET', '/me/payments/companies', 200, { token });
    check(hidden.hidden === true && !visible.items.some((x) => x.linkId === company.linkId), 'hide removes it from the list');
    await must('PATCH', linkPath, 200, { token, body: { hidden: false } });
    const stranger = await call('GET', `${linkPath}/payins`, { token: ctx.token });
    check(stranger.status === 403 && stranger.json?.error?.code === 'kyc.not_approved', `company user ${stranger.status}`);

    // Her ARS + USD CVUs are issued on the KYC approval and listed on her personal account; she
    // names the ARS one. Fund the wallet through it at the bank, then pay the company.
    const account = await waitFor('personal CVUs', async () => {
      const list = await must('GET', `${orgPath}/accounts`, 200, { token });
      return list.items[0]?.cvus.length === 2 ? list.items[0] : null;
    }, 30_000);
    const personal = account.cvus.find((c) => c.currency === 'ARS');
    const alias = `smk.${run}`.slice(0, 20);
    const renamed = await must('PATCH', `${orgPath}/accounts/${account.id}/cvus/${personal.id}`, 200, { token, body: { alias } });
    check(renamed.alias === alias && renamed.cvu === personal.cvu, `alias ${JSON.stringify(renamed)}`);
    const funded = await call('POST', '/_mock/incoming-transfer', { base: BANK, body: { cvu: personal.cvu, amount: '500000.00', currency: 'ARS', originName: personName } });
    check(funded.status === 201, `fund personal → ${funded.status}`);
    await waitFor('personal balance', async () => {
      const b = await must('GET', `${orgPath}/balances`, 200, { token });
      return b.totals.some((t) => t.currency === 'ARS' && t.available.minor > 0);
    }, 30_000);
    const unsigned = await call('POST', `${linkPath}/pay`, { token, body: { amount: { minor: 10_000_000, currency: 'ARS' } }, headers: { 'Idempotency-Key': key('mp-pay-nosu') } });
    check(unsigned.status === 403 && unsigned.json?.error?.code === 'auth.step_up_required', `pay without step-up ${unsigned.status}`);
    const po = await must('POST', `${linkPath}/pay`, 201, {
      token, body: { amount: { minor: 10_000_000, currency: 'ARS' }, concept: 'Pago smoke' },
      headers: { 'Idempotency-Key': key('mp-pay'), 'X-Step-Up-Token': await stepUp(token) },
    });
    // On-us (P5): the company's CVU is a Lenapay one, so it settles at once on the internal rail,
    // never reaches the bank, and the company receives it as a payin she sees in her history.
    check(po.destination.accountNumber === cvu.cvu && po.concept === 'Pago smoke' && po.status === 'settled' && po.rail === 'internal',
      `payout ${JSON.stringify(po).slice(0, 200)}`);
    const atBank = await call('GET', `/cvucollectentidad-financial/v1/v1.201/banks/322/accounts/owner/transaction-request-types/TRANSFER/po-${po.id}-1`, { base: BANK });
    check(atBank.status === 422, `the bank knows the on-us payout: ${atBank.status}`);
    const history = await must('GET', `${linkPath}/payins`, 200, { token });
    check(history.items.length === 2 && history.items.every((x) => x.status === 'credited'), `history ${JSON.stringify(history)}`);
    const received = await must('GET', `${ctx.orgPath}/movements?type=payin&limit=50`, 200, { token: ctx.token });
    check(received.items.some((m) => m.rail === 'internal' && m.amount.minor === 10_000_000), 'company movements lack the on-us payin');
    return `${companyName}: paid ARS ${po.amount.minor / 100} on-us (${po.status})`;
  });
}

try {
  await main();
} catch (err) {
  results.push({ step: 'unexpected', result: 'FAIL', ms: 0, detail: String(err?.stack ?? err).slice(0, 200) });
} finally {
  ctx.stream?.close();
}

const w = Math.max(...results.map((r) => r.step.length));
console.log(`\nLenapay smoke against ${API} (bind-mock ${BANK})\n`);
console.log(`${'STEP'.padEnd(w)}  RESULT  ${'MS'.padStart(6)}  DETAIL`);
for (const r of results) console.log(`${r.step.padEnd(w)}  ${r.result.padEnd(6)}  ${String(r.ms).padStart(6)}  ${r.detail}`);
const failed = results.filter((r) => r.result !== 'PASS').length;
console.log(`\n${failed === 0 ? 'PASS' : 'FAIL'}: ${results.length - failed}/${results.length} steps passed`);
process.exit(failed === 0 ? 0 : 1);
