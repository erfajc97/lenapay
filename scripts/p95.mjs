#!/usr/bin/env node
// Latency check of the hot reads (R-NF-1: p95 < 150 ms) against the seeded stack: logs in as
// the demo user and times N sequential requests per endpoint. Run `scripts/seed.ps1` (or
// `make seed`) first.   node scripts/p95.mjs [N=200]
// The API rate-limits each client IP (RATE_LIMIT_PER_MINUTE, 300 by default, sliding window),
// so requests are paced at 240 per minute instead of measuring 429s.
const API = (process.env.LENAPAY_API_URL ?? 'http://localhost:18080').replace(/\/$/, '');
const N = Number(process.argv[2] ?? 200);

async function json(method, path, token, body) {
  const res = await fetch(API + path, {
    method,
    headers: { ...(token ? { Authorization: `Bearer ${token}` } : {}), ...(body ? { 'Content-Type': 'application/json' } : {}) },
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!res.ok) throw new Error(`${method} ${path} → ${res.status}`);
  return res.json();
}

const { accessToken } = await json('POST', '/auth/login', null, { email: 'demo@lenapay.app', password: 'Lenapay2026!' });
const { items } = await json('GET', '/orgs', accessToken);
const org = items.find((o) => o.legalName === 'Desarrollos Del Sur SA');
if (!org) throw new Error('demo org missing: run the seed first');

const quantile = (sorted, q) => sorted[Math.min(sorted.length - 1, Math.ceil(q * sorted.length) - 1)];
console.log(`${N} sequential requests per endpoint against ${API}\n`);
console.log('ENDPOINT                         p50 ms   p95 ms   p99 ms   max ms');
const paths = [`/orgs/${org.id}/balances`, `/orgs/${org.id}/movements?limit=20`, `/orgs/${org.id}/payers?limit=20`];
const PACE_MS = 250;
for (const path of paths) {
  const times = [];
  for (let i = 0; i < N; i++) {
    await new Promise((r) => setTimeout(r, PACE_MS));
    const t0 = performance.now();
    const res = await fetch(API + path, { headers: { Authorization: `Bearer ${accessToken}` } });
    await res.arrayBuffer();
    if (!res.ok) throw new Error(`${path} → ${res.status}`);
    times.push(performance.now() - t0);
  }
  times.sort((a, b) => a - b);
  const label = path.replace(org.id, '{org}').split('?')[0];
  const f = (v) => v.toFixed(1).padStart(8);
  console.log(`GET ${label.padEnd(28)} ${f(quantile(times, 0.5))} ${f(quantile(times, 0.95))} ${f(quantile(times, 0.99))} ${f(times.at(-1))}`);
}
