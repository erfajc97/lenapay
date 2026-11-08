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

## P — "Mis pagos": payer view, B2C bridge (api) · 6 pts
A person whose personal org passed KYC sees the companies that registered them as a payer and
pays them. Requirements R-MP-1..5, R-CVU-4 and R-ACC-5 are in `docs/prd.md` (owner's brief of
2026-11-07; CUIL counts like CUIT for matching, confirmed). Mobile screens are a separate lane.
- [x] P1 Matching (R-MP-1): `payer_links` (migration `20261109100000_payer_links.sql`, tenant =
  the personal org); KYC approval hook + lazy matching on read; DNI, or the middle 8 digits of
  an 11-digit CUIT/CUIL (7-digit DNIs left-padded); only the DNI KYC approval copied onto an
  approved personal org (never draft KYC data, never a company's CUIT). Deps: A4, I9.
  AC: approval links in the same transaction; later payers appear on the next read; unverified
  data → no link; another person's link is 404 on every door.
- [x] P2 Read endpoints (R-MP-2): `GET /me/payments/companies` (company name, project, own
  CVUs, totals per currency, last payment, ERP flag, hidden), `GET …/{linkId}/payins` (cursor),
  `PATCH …/{linkId}` `{hidden}`. AC: exact key sets; nothing else of the company is exposed.
- [x] P3 Installments (R-MP-3): `erplink.Directory.Installments` (port + fixture fake: number,
  due date, amount minor + currency, status pending|paid|overdue, CAC/UVA index) and
  `GET …/{linkId}/installments` (`erpLinked=false`, `[]` when the company is not linked).
- [x] P4 Pay (R-MP-4): `POST …/{linkId}/pay` through `payouts.Create` on the personal org
  (Idempotency-Key + step-up, sent without approval) to the person's CVU at the company in the
  amount's currency; `installmentNumber` must exist and goes in the concept (`Cuota N - …`).
  AC: step-up required; replay returns the same payout; unknown installment 404; no CVU in the
  currency 422; settles end to end (fake PSP and bind-mock).
- [x] P6 Seed + smoke: persona's DNI is a payer of the demo org (DNI) and of the ERP-linked
  Constructora Andina SRL (`andina@lenapay.app`, CUIT buyer in the fixtures, 12 installments);
  personal wallet funded; `scripts/smoke.mjs` step "Mis pagos".
- [x] P5 On-us payments (R-MP-5, owner's decision 2026-11-07: never through the bank). A payout
  whose destination CVU/alias is a Lenapay one (payers lookup BEFORE the PSP) is `rail=internal`:
  hold + 4-eyes as usual, then settled at once (personal create / company approval) as ONE ledger
  entry payer −(amount+fee) → receiver +amount (+ fees_revenue), fee from the plan's new on-us
  rates (`fee_plan_rates.internal_*`, default 0); the receiver gets a credited payin
  (`payins.rail=internal`, `payout_id`, `provider_transfer_id = internal:<payout>`), notification,
  realtime `payin.credited`. Reconciliation lists only `rail=bank`. Destination disabled before the
  approval → `failed` `destination_unavailable`, hold released. Migration
  `20261110100000_internal_rail.sql`. The bind-mock also credits its own CVUs on an outgoing
  transfer (with `origin_debit`), as a fallback for bank-rail payouts that still reach one.
  AC: person→company, company→company, company→person, concurrency (8 at a balance for 3 → 3),
  same key twice → one payout and one payin, the PSP is never called; smoke step.
- [x] P8 Personal CVUs on the account (R-CVU-4): `Account.cvus` + `PATCH
  /orgs/{org}/accounts/{accountId}/cvus/{cvuId}` (alias); seed/smoke read the CVU from the API.
- [x] P9 Personal account by default (R-ACC-5): signup creates the personal org (KYC draft) and its
  personal account in the signup transaction when `PERSONAL_MODE_ENABLED` (default on in compose,
  off in the binary for production); personal CVUs ARS+USD issued on the person KYC approval.
- [ ] P7 Mobile "Mis pagos" screens (lane M) + es-AR copy for the new error codes
  `payer_link.not_found` and `installment.not_found` in `apps/mobile/src/shared/api/errors.ts`.

## L — Mobile app in live mode (expo) · 8 pts
Deps: I1–I10. The app runs fully against the Go API (`EXPO_PUBLIC_API_MODE=live`) and keeps mock.
- [x] L1 Contract types: `src/shared/api/types.ts` derives from `@lenapay/api-types` (tsconfig path,
  type-only, ESLint guard); DTO → view model in `live/mappers.ts`; mock produces the same view
  models. AC: typecheck exit 0; mapper specs pin money, nulls and joined names.
- [x] L2 Mismatches of the integration pass: reconciliation runs + items + on-demand run, ERP
  candidate/link, KYC company/person data + `kind`/`representativeDni` uploads (DNI front, back,
  selfie), movement detail by id, payout attachments (multipart) and attachments on the payout,
  push token platform, member role change/remove, resend code, verify-email without tokens,
  nullable fields, every app-reachable error code with es-AR + en-US copy (R-ID-1, R-ORG-1,
  R-KYC-*, R-MOV-1, R-OUT-*, R-REC-*, R-ERP-*).
- [x] L3 Realtime: unnamed SSE frames `{type, orgId, occurredAt, data}` for `payin.credited`,
  `payout.updated`, `approval.requested`, `balance.changed` → query invalidation + payin banner;
  refetch after a reconnect; token refresh on a 401 stream (R-IN-3).
- [x] L4 Step-up: password fallback and Ed25519 device key (registered with `POST /devices`
  after sign-in on phones with biometrics; signature over the challenge nonce) (R-ID-3).
- [x] L5 "Modo demo" badge only in mock; Settings → Desarrollador (dev builds) switches mock/live
  and the API URL at runtime (persisted); default URL = Metro host on port 18080.
- [x] L6 `pnpm test:live` against the running stack; live web tour screenshots in
  `docs/screens/live/` (closes the E9 screenshots item for the live app).
