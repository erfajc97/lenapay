# Lenapay — Backlog (MVP)

Tracking: this file is the plan; `docs/progress.md` is the log. Points: 1 = half a day.
Lanes run in parallel once E0 is done; a lane only touches its own packages/folders.

## E0 — Foundation (sequential, blocks everything) · 8 pts
- S0.1 Monorepo, Makefile, docker-compose (postgres, api, bind-mock), `.env.example`.
- S0.2 `contracts/openapi.yaml` for the whole MVP surface (architecture §5) + codegen (Go strict
  server, TS types) wired in `make gen`.
- S0.3 All goose migrations for architecture §3 + River migrations; sqlc config per feature.
- S0.4 Platform packages: config, db (pool, WithTx, serializable retry), httpx (error writer,
  middleware, SSE helper), money, idempotency store, audit, clock, auth (JWT/JWKS verify,
  step-up check, role guard with default-deny route table).
- S0.5 `main.go` wiring every feature with stub handlers returning `501 not_implemented`, so
  lanes only fill their packages.
- S0.6 Expo app skeleton: router, providers (Query, theme tokens from brand, i18n es-AR), API
  client from the generated types, secure session store, UI kit primitives.

## Lane A — Identity, orgs, KYC (api) · 14 pts
- A1 Signup, email code, login, refresh rotation, logout, `/me` (R-ID-1, R-ID-2).
- A2 Local IdP + JWKS, device registration, step-up challenge/verify (R-ID-3, R-ID-4).
- A3 Orgs, memberships, roles, invites (R-ORG-1, R-ORG-2).
- A4 KYC cases company/person, documents, submit, fake provider, internal decision (R-KYC-*).

## Lane B — Ledger, accounts, fees, movements (api) · 14 pts
- B1 Ledger: postings, balances with holds, invariants, posting API used by other features
  (R-ACC-2, R-NF-2).
- B2 Accounts (main/project/personal), balances endpoint, internal transfers (R-ACC-1/3/4).
- B3 Fee plans and fee calculator (R-FEE-1).
- B4 Movements read model + cursor pagination + filters (R-MOV-1).

## Lane C — PSP: bind-mock, payers/CVU, payins, realtime (api + bind-mock) · 16 pts
- C1 `apps/bind-mock`: BIND-shaped endpoints (CVU create/alias/disable, transfers, transfer by
  id, destination resolve, master balance, statement) + `/_mock` control + signed webhooks.
- C2 `psp.Provider` port + HTTP adapter (timeouts, retries, per-attempt origin id) + fake.
- C3 Payers and CVUs (R-CVU-*).
- C4 Webhook intake, payin processing job, ledger posting, notifications, SSE (R-IN-*,
  R-NOT-1).

## Lane D — Payouts, approvals, reconciliation, ERP link (api) · 22 pts
- D1 Destination resolve + quote (R-OUT-1, R-OUT-2).
- D2 Create payout with idempotency + hold; approvals 4-eyes; cancel/reject (R-OUT-3/4).
- D3 Dispatch job, sweep for uncertain states, settle/fail postings, proof PDF (R-OUT-5/6).
- D4 Reconciliation job + runs endpoints (R-REC-*).
- D5 ERP identity index (fixtures), link with fake credentials check, import job (R-ERP-*).

## Lane M — Mobile app (expo) · 24 pts
- M1 Auth: signup, verify, login, biometric unlock + device key, step-up sheet.
- M2 Onboarding/KYC company + person, org switcher, team & invites.
- M3 Home (balances ARS/USD, held, recent movements, pending approvals), accounts.
- M4 Payers + CVU create/share (QR, link), live payins (SSE + push).
- M5 Payout flow (destination resolve, quote, confirm with step-up), approvals inbox.
- M6 Movements list/detail + proof, reconciliation runs, ERP link screens, settings.

## E9 — Demo hardening · 6 pts
- Seed script (demo org linked to ERP fixture, payers, payins), end-to-end smoke script driving
  the bind-mock, README with the demo path, screenshots.
