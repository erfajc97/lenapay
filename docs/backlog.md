# Lenapay — Backlog (MVP)

Tracking: this file is the plan; `docs/progress.md` is the log. Points: 1 = half a day.
Lanes run in parallel once E0 is done; a lane only touches its own packages/folders.

## E0 — Foundation (sequential, blocks everything) · 8 pts
- [x] S0.1 Monorepo, Makefile, docker-compose (postgres, api, bind-mock), `.env.example`.
- [x] S0.2 `contracts/openapi.yaml` for the whole MVP surface (architecture §5) + codegen (Go strict
  server, TS types) wired in `make gen`.
- [x] S0.3 All goose migrations for architecture §3 + River migrations; sqlc config per feature.
- [x] S0.4 Platform packages: config, db (pool, WithTx, serializable retry), httpx (error writer,
  middleware, SSE helper), money, idempotency store, audit, clock, auth (JWT/JWKS verify,
  step-up check, role guard with default-deny route table).
- [x] S0.5 `main.go` wiring every feature with stub handlers returning `501 not_implemented`, so
  lanes only fill their packages.
- [ ] S0.6 Expo app skeleton: router, providers (Query, theme tokens from brand, i18n es-AR), API
  client from the generated types, secure session store, UI kit primitives.

## Lane A — Identity, orgs, KYC (api) · 14 pts
- [x] A1 Signup, email code, login, refresh rotation, logout, `/me` (R-ID-1, R-ID-2).
- [x] A2 Local IdP + JWKS, device registration, step-up challenge/verify (R-ID-3, R-ID-4).
- [x] A3 Orgs, memberships, roles, invites (R-ORG-1, R-ORG-2).
- [x] A4 KYC cases company/person, documents, submit, fake provider, internal decision (R-KYC-*).

## Lane B — Ledger, accounts, fees, movements (api) · 14 pts
- [x] B1 Ledger: postings, balances with holds, invariants, posting API used by other features
  (R-ACC-2, R-NF-2).
- [x] B2 Accounts (main/project/personal), balances endpoint, internal transfers (R-ACC-1/3/4).
- [x] B3 Fee plans and fee calculator (R-FEE-1).
- [x] B4 Movements read model + cursor pagination + filters (R-MOV-1).

## Lane C — PSP: bind-mock, payers/CVU, payins, realtime (api + bind-mock) · 16 pts
- [x] C1 `apps/bind-mock`: BIND-shaped endpoints (CVU create/alias/disable, transfers, transfer by
  id, destination resolve, master balance, statement) + `/_mock` control + signed webhooks.
- [x] C2 `psp.Provider` port + HTTP adapter (timeouts, retries, per-attempt origin id) + fake.
- [x] C3 Payers and CVUs (R-CVU-*).
- [x] C4 Webhook intake, payin processing job, ledger posting, notifications, SSE (R-IN-*,
  R-NOT-1).

## Lane D — Payouts, approvals, reconciliation, ERP link (api) · 22 pts
- [x] D1 Destination resolve + quote (R-OUT-1, R-OUT-2).
- [x] D2 Create payout with idempotency + hold; approvals 4-eyes; cancel/reject (R-OUT-3/4).
- [x] D3 Dispatch job, sweep for uncertain states, settle/fail postings, proof PDF (R-OUT-5/6).
- [x] D4 Reconciliation job + runs endpoints (R-REC-*).
- [x] D5 ERP identity index (fixtures), link with fake credentials check, import job (R-ERP-*).

## Lane M — Mobile app (expo) · 24 pts
- [ ] M1 Auth: signup, verify, login, biometric unlock + device key, step-up sheet.
- [ ] M2 Onboarding/KYC company + person, org switcher, team & invites.
- [ ] M3 Home (balances ARS/USD, held, recent movements, pending approvals), accounts.
- [ ] M4 Payers + CVU create/share (QR, link), live payins (SSE + push).
- [ ] M5 Payout flow (destination resolve, quote, confirm with step-up), approvals inbox.
- [ ] M6 Movements list/detail + proof, reconciliation runs, ERP link screens, settings.

## I — Backend integration pass (contract gaps for live mode) · 6 pts
- [x] I1 `POST /auth/resend-code` (R-ID-1, rate limited with the signup codes, no enumeration).
- [x] I2 `PATCH/DELETE /orgs/{org}/members/{userId}`: no escalation, never the last admin (R-ORG-1).
- [x] I3 `GET /orgs/{org}/movements/{id}` movement detail (R-MOV-1).
- [x] I4 `POST /orgs/{org}/reconciliation/runs` (admin) + `GET .../runs/{id}/items` paginated (R-REC-*).
- [x] I5 `account.not_archivable` (409); push token `platform` optional (`ios|android|web|unknown`).
- [x] I6 Realtime framing documented; `payout.status_changed` → `payout.updated` with the payout.
- [x] I7 Payout attachments (PDF/JPEG/PNG ≤ 10 MB by magic bytes, document store port + disk fake).
- [x] I8 Nullable response fields always present as `null`; payer email clearable with `null`.
- [x] I9 KYC approval copies the verified CUIT/legal name (or DNI/name) onto the org (R-ERP-1).
- [x] I10 Compose: API on host port 18080, ERP fixtures in the image, demo defaults without `.env`.

## E9 — Demo hardening · 6 pts
- [x] Seed (`cmd/seed`, `make seed`, `scripts/seed.ps1`): demo org (ERP candidate found by CUIT),
  projects, payers + CVUs, payins through bind-mock webhooks, payouts, transfer, reconciliation,
  personal wallet. Not linked to the ERP on purpose (the demo shows the link flow).
- [x] End-to-end smoke script driving the bind-mock (`scripts/smoke.mjs`); p95 check (`scripts/p95.mjs`).
- [x] README with the demo path and credentials.
- [ ] Screenshots (after the app runs in live mode).
