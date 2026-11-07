#!/usr/bin/env node
// End-to-end smoke test of the running stack (docker compose up), no dependencies (Node 18+).
// It walks the whole money path through the public API and the bind-mock's control endpoints:
// signup → verify → login → org → KYC (dev auto-approval) → payer + CVU → payin through the
// bind-mock (signed webhook, worker credits the ledger) → balance + movement + SSE event →
// quote + payout → a second user approves → dispatch settles → attachment + proof PDF →
// reconciliation run. Prints a PASS/FAIL table; exit code 1 when any step fails.
//
//   node scripts/smoke.mjs
//   LENAPAY_API_URL=http://localhost:18080 BINDMOCK_URL=http://localhost:8090 node scripts/smoke.mjs

const API = (process.env.LENAPAY_API_URL ?? 'http://localhost:18080').replace(/\/$/, '');
const BANK = (process.env.BINDMOCK_URL ?? 'http://localhost:8090').replace(/\/$/, '');
const CODE = process.env.DEV_FIXED_EMAIL_CODE ?? '123456';
const PASSWORD = 'Smoke-test-2026!';
const run = Date.now().toString(36) + Math.floor(Math.random() * 1e6).toString(36);
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
