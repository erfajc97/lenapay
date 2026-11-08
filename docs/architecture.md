# Lenapay — Architecture (MVP)

Requirements: `docs/prd.md`. Rules: `CLAUDE.md` (kit blocks G, A, AG, M, ME, addons MO, T, OB).

## 1. Shape

```
apps/mobile (Expo)  ──HTTPS/JSON + SSE──►  apps/api (Go)  ──►  Postgres 17 (lenapay)
                                              │  ▲
                              PSP port (HTTP) │  │ signed webhooks
                                              ▼  │
                                         apps/bind-mock (Go)   ← simulates BIND COBRO + CVUCOLLECT
Fakes inside apps/api (adapters): KYC, ERP, notifications (email/push), clock.
```

- One Go module per app: `lenapay/apps/api` and `lenapay/apps/bind-mock` (a workspace in
  `go.work`; the paths have no host on purpose — the code is not published as a library); Go 1.27.
- One contract: `contracts/openapi.yaml` (OpenAPI 3.0.3). Generates the Go strict server
  (`oapi-codegen`) and the TS client types (`openapi-typescript`) used by the app.
- Local run: `docker compose up` → postgres:17, api (host :18080 → container :8080), bind-mock (:8090). The app runs
  with `npx expo start` in its own terminal (ME14) pointing at `EXPO_PUBLIC_API_URL`.

## 2. Monorepo layout

```
contracts/openapi.yaml              single source of the API contract
apps/api/
  cmd/api/main.go                   wiring only: config → pool → river → adapters → router
  internal/platform/                config, db (pool, WithTx, serializable retry), httpx (errors,
                                    middleware, SSE), auth (JWT/JWKS, step-up), idempotency,
                                    money (Amount{Minor int64, Currency}, split), audit, clock
  internal/gen/                     oapi-codegen output (do not edit)
  internal/<feature>/               handler.go · service.go · store.go (+ queries.sql → sqlc gen
                                    in internal/<feature>/db) · service_test.go · fake_*.go
     identity  orgs  kyc  accounts  ledger  payers  payins  payouts  fees  reconciliation
     erplink  movements  notifications  realtime  payerview ("Mis pagos")
     providers/psp (port + bind http adapter)
     providers/kyc  providers/erp  providers/notify
  migrations/                       goose SQL, numbered, hand-written
  sqlc.yaml                         one sql entry per feature → its own package
apps/bind-mock/                     fake PSP: REST endpoints shaped like BIND + webhook sender
                                    + `/_mock/*` control endpoints (simulate incoming transfer,
                                    force payout failure/timeout, statement)
apps/mobile/                        Expo Router app (kit M/ME): src/app routes, src/features/*,
                                    src/shared (api client, queries, ui kit, i18n, theme)
docs/  prd.md · architecture.md · backlog.md · progress.md · brand.md · decisions.md
docker-compose.yml · Makefile (gen, test, lint, up, seed)
```

Feature packages never import each other's internals; cross-feature calls go through the
exported service interface of the other feature (wired in `main.go`). The ledger is the only
writer of balances.

## 3. Data model (Postgres)

All ids are UUIDv7; all money columns `bigint` minor units + `currency char(3)`; timestamps
`timestamptz` UTC. Tenancy: every org-owned row carries `org_id`; queries filter by it (T).

| Table | Purpose |
|---|---|
| `users` | email, password hash (argon2id), email_verified_at, totp_secret?, status, full_name? (1–120) |
| `sessions` / `refresh_tokens` | rotating refresh tokens (hashed), device id |
| `devices` | device public key for biometric step-up |
| `email_codes` | verification codes (hashed, expiry) |
| `organizations` | kind `company`/`personal`, legal name, cuit, kyc_status, fee_plan_id, erp_link |
| `memberships` | user × org × role (`viewer` < `operator` < `approver` < `admin` < `owner`); exactly one `owner` per org: unique partial index `memberships_one_owner_key` + deferred constraint trigger `memberships_keep_owner` (demoting or deleting the owner must be compensated in the same transaction) |
| `invites` | email, role (never owner), token hash, expires_at, `source` `manual`/`erp_import` (the import's invites are auto-accepted for a verified email) |
| `approval_policies` | org_id PK, mode `always`/`threshold`, `threshold_ars_minor`/`threshold_usd_minor` (≥ 1, null = no threshold), `allow_owner_self_approval`, updated_by/at; no row = defaults |
| `user_preferences` | user_id PK, default_org_id, default_account_id (needs the org), locale `es-AR`/`en-US`, theme `system`/`light`/`dark`, updated_at |
| `kyc_cases` / `kyc_documents` | onboarding case per org/person, status, provider ref, files |
| `accounts` | org_id, kind `main`/`project`/`personal`/`system`, name, archived_at |
| `ledger_accounts` | (account_id, currency) → ledger account; system accounts: psp_master, fees_revenue, payout_clearing, suspense |
| `ledger_transactions` | id, kind, ref_type/ref_id, idempotency key, created_by |
| `ledger_postings` | transaction_id, ledger_account_id, amount (signed); CHECK sum per tx = 0 (deferred trigger) |
| `ledger_balances` | cached posted + held per ledger account, updated in the same tx with `FOR UPDATE` |
| `holds` | amount held per payout, released/captured |
| `payers` | org_id, account_id, name, doc, email, erp_ref? |
| `cvus` | payer_id or account_id, currency, cvu, alias, psp_ref, status |
| `psp_webhook_events` | raw body, signature ok, provider event id UNIQUE, processed_at |
| `payins` | cvu_id, amount, fee, net, origin name/cuit/cbu, provider transfer id UNIQUE, status |
| `payouts` | org, account, currency, amount, fee, destination snapshot, concept, status, created_by, approved_by, `self_approved` (owner approval under the policy; `payouts_four_eyes_check` keeps approved_by ≠ created_by otherwise), provider attempt id, idempotency key |
| `payout_events` | status history |
| `fee_plans` | payin_bps, payout_fixed_minor, payout_bps per currency |
| `reconciliation_runs` / `reconciliation_items` | per currency/day, counts, difference |
| `erp_identity_index` | doc_type, doc_number PK → erp_org_id, erp_client_id, display name |
| `erp_links` | org ↔ erp org, linked_by, linked_at; imported snapshot refs |
| `payer_links` | "Mis pagos": personal_org_id (tenant) ↔ a company's payer (payer_id, org_id), matched_doc_type `dni`/`cuit`/`cuil`, status `active`/`hidden`, matched_at; UNIQUE (personal_org_id, payer_id); payers get the expression index `payers_person_dni_idx` (the 8-digit DNI a payer document carries) |
| `idempotency_keys` | (scope, key) PK, request hash, response status/body, expires_at |
| `audit_log` | actor, action, target, request id, idempotency key, payload hash |
| `notifications` / `push_tokens` | outbox of user notifications, Expo push tokens |
| River tables | job queue (`river migrate`) |

## 4. Key flows

- **Payin**: bind-mock POSTs `/webhooks/psp/transfers` with `X-Signature: sha256=<hmac(raw)>` →
  verify → insert event (dedupe) → enqueue `process_payin` (same tx) → job re-reads the
  transfer at the PSP (`GET /transfers/{id}`), finds the CVU, posts ledger (psp_master −gross →
  account +net, fees_revenue +fee) → payin row → notify (SSE via `LISTEN/NOTIFY` + push outbox).
- **Payout**: `POST /orgs/{org}/payouts` (Idempotency-Key + step-up) → resolve destination at
  PSP → compute fee → tx: lock balance, hold amount+fee, insert payout `pending_approval`
  (company) or `approved` (personal) → approver `POST /payouts/{id}/approve` (step-up, not
  creator) → enqueue `dispatch_payout` → job calls PSP with a per-attempt origin id →
  settled: capture hold (account −, psp_master +, fees_revenue +) / failed: release hold /
  timeout or 5xx: status `sending`, `sweep_payouts` re-queries.
- **Reconciliation**: job `reconcile_master(currency, day)` pulls the PSP statement
  (`GET /accounts/master/statement`) and matches by provider transfer id against payins and
  settled payouts.
- **Approval policy** (R-OUT-7): `orgs.ApprovalRules(orgID)` = the stored policy (or the defaults)
  + the number of members besides the owner with role approver/admin. Payout create asks
  `AutoApproves(amount)` (threshold mode, a threshold for the currency, amount strictly below it) →
  `approved` + dispatch in the same transaction, event reason `policy_threshold`; approve asks
  `OwnerMaySelfApprove()` when the caller is the owner and the creator (allowed and nobody else can
  approve) → `self_approved`, audit `payout.self_approval`. Everything else is 4-eyes.
- **ERP link**: `GET /orgs/{org}/erp-link/candidate` looks up `erp_identity_index` by CUIT →
  `POST /orgs/{org}/erp-link` with ERP credentials → ERP adapter verifies (fake) → import job
  copies projects/buyers from the ERP adapter (fake reads `apps/api/fixtures/erp/*.json`), then
  invites the ERP users (below) and stores the summary on the link (`importSummary`).
- **Mis pagos (payer view, B2C bridge, `internal/payerview`)**: when a person's personal org is
  KYC-approved, the approval copies the verified DNI onto the org and the `payerview` approval
  hook (same transaction) links every company payer whose document carries it: a DNI equal to
  it (7-digit DNIs left-padded) or an 11-digit CUIT/CUIL whose middle 8 digits are it. Every
  `GET /me/payments/companies` links new matches again (payers created later). Only verified
  data matches (never the KYC case in draft or a company's self-declared CUIT). The person sees
  only the company's legal name, the project account, their own active CVUs, their own payins
  and, when the company is ERP-linked, their installments from `erplink.Directory.Installments`
  (by the payer's `erp_ref`). Paying reuses `payouts.Create` on the person's personal org
  (Idempotency-Key + step-up; personal payouts are sent without approval) towards their CVU at
  the company, with `Cuota N - …` in the concept when an installment is named. The read side is
  a read model like movements (it never writes payers, CVUs or payins). Known gap: a payout to a
  Lenapay CVU goes out through the PSP and the bind-mock does not loop it back as a payin, so it
  does not show in the company's history yet (backlog P5).

## 5. API surface (contract summary)

Auth: `POST /auth/signup` (optional `fullName`), `/auth/verify-email`, `/auth/resend-code`, `/auth/login`, `/auth/refresh`, `/auth/logout`,
`/auth/step-up/challenge`, `/auth/step-up/verify`, `POST /devices`, `GET /.well-known/jwks.json`,
`GET/PATCH /me` (`{fullName}`; `/me` carries `preferences`), `GET/PATCH /me/preferences`
(`{defaultOrgId, defaultAccountId, locale, theme}`, absent keeps, null clears).
Orgs: `GET/POST /orgs`, `GET /orgs/{org}`, `GET/POST /orgs/{org}/members`,
`PATCH/DELETE /orgs/{org}/members/{userId}`, `POST /orgs/{org}/invites`,
`POST /invites/{token}/accept`, `POST /orgs/{org}/ownership-transfer` (owner + step-up),
`GET/PUT /orgs/{org}/approval-policy` (any member reads; owner + step-up writes).
Roles on responses are `MemberRole` (adds `owner`); requests keep `Role` (grantable, no owner).
KYC: `GET/PUT /orgs/{org}/kyc`, `POST /orgs/{org}/kyc/documents`, `POST /orgs/{org}/kyc/submit`,
`POST /internal/kyc/{case}/decision` (ops, API key).
Accounts: `GET/POST /orgs/{org}/accounts`, `PATCH /orgs/{org}/accounts/{id}`, `GET /orgs/{org}/balances`,
`POST /orgs/{org}/internal-transfers`.
Payers/CVU: `GET/POST /orgs/{org}/payers`, `GET/PATCH /orgs/{org}/payers/{id}`,
`POST /orgs/{org}/payers/{id}/cvus`, `PATCH/DELETE /orgs/{org}/cvus/{id}`.
Payins: `POST /webhooks/psp/transfers`, `GET /orgs/{org}/payins/{id}`.
Payouts: `POST /orgs/{org}/payouts/quote`, `POST /orgs/{org}/payouts`, `GET /orgs/{org}/payouts/{id}`,
`POST /orgs/{org}/payouts/{id}/approve|reject|cancel`, `GET /orgs/{org}/approvals`,
`GET /orgs/{org}/payouts/{id}/proof.pdf`, `POST /orgs/{org}/payouts/{id}/attachments`,
`GET /orgs/{org}/destinations/resolve?to=`.
Movements: `GET /orgs/{org}/movements?account=&type=&status=&currency=&from=&to=&cursor=`,
`GET /orgs/{org}/movements/{id}`.
Reconciliation: `GET/POST /orgs/{org}/reconciliation/runs`, `GET /orgs/{org}/reconciliation/runs/{id}/items`,
`POST /internal/reconciliation/run`.
ERP: `GET /orgs/{org}/erp-link/candidate`, `POST /orgs/{org}/erp-link`, `GET /orgs/{org}/erp-link`.
Mis pagos (authenticated; the service resolves the caller's own verified personal org, another
person's link is `payer_link.not_found` 404, no verified personal org is `kyc.not_approved` 403):
`GET /me/payments/companies?includeHidden=` → `{items: [{linkId, companyName, projectName, cvus[{currency, cvu, alias}],
totalPaid[Money], lastPaymentAt, erpLinked, hidden}]}`, `PATCH /me/payments/companies/{linkId}` `{hidden}`,
`GET /me/payments/companies/{linkId}/payins?cursor=&limit=` → `{items: [{id, cvu, amount, status, createdAt, creditedAt}], nextCursor}`,
`GET /me/payments/companies/{linkId}/installments` → `{erpLinked, items: [{number, dueDate, amount, status pending|paid|overdue, adjustmentIndex CAC|UVA|null}]}`,
`POST /me/payments/companies/{linkId}/pay` (Idempotency-Key + step-up) `{amount, installmentNumber?, concept?}` → `Payout`
(`installment.not_found` 404, `money.currency_mismatch` 422 without a CVU in that currency).
Realtime/push: `GET /orgs/{org}/events` (SSE: unnamed `message` frames whose data is
`{type, orgId, occurredAt, data, ...data copied top-level}`; types `payin.credited`, `payout.updated`
(carries the `payout`), `approval.requested`, `balance.changed`), `POST /push-tokens`.
Errors: `{ "error": { "code": "payout.insufficient_funds", "message": "...", "details": {} } }`.

## 6. Auth and security

- Local IdP issues EdDSA JWT (15 min) with `sub`, `sid`; JWKS published. Org role checked per
  request from `memberships` (default-deny route table). Step-up token: 5-minute JWT with
  `amr=["stepup"]`, bound to `sid`, required by money mutations.
- Device key (Ed25519, generated on device, private key in SecureStore behind biometrics)
  signs the step-up challenge; fallback password.
- Roles: viewer < operator < approver < admin < owner (`auth.Roles()`, the ONE ladder). Member
  doors: nobody acts above their own role; an admin's row is changed only by the owner; the owner's
  row never (`org.last_admin`); `owner` is not grantable. Owner-only routes: ownership transfer and
  approval policy (both with step-up).
- CORS allows the app's headers plus `Cache-Control`, `X-Requested-With`, `Last-Event-ID` (the web
  EventSource polyfill on the SSE stream).
- Webhooks: HMAC-SHA256 over the raw body with a shared secret, constant-time compare,
  timestamp tolerance 5 min.
- Secrets only from env; `.env.example` documents them.

## 7. Mobile app

Expo SDK (latest stable), Expo Router (`src/app`), TanStack Query (+ persist in MMKV),
Zustand for client state, uniwind + Tailwind v4 tokens (Lebane palette, `docs/brand.md`),
FlashList, expo-secure-store, expo-local-authentication, expo-crypto, expo-notifications,
react-native-sse (EventSource), i18n es-AR, Vitest. Features: `auth`, `onboarding`, `orgs`,
`home`, `accounts`, `payers`, `payouts`, `approvals`, `movements`, `reconciliation`,
`erp-link`, `settings`. Money is formatted with `Intl.NumberFormat('es-AR')` from minor units.

**Contract types in the app.** `apps/mobile` imports `@lenapay/api-types` (generated by `make gen`)
through a TypeScript path (`tsconfig.json` → `../../packages/api-types/src/index.ts`) with
`import type` only, enforced by ESLint (`@typescript-eslint/no-restricted-imports`,
`allowTypeImports`). Babel erases type imports, so Metro, the web export and EAS builds never
resolve the package; no `file:`/`workspace:` dependency, no copy step, nothing to install
(the app keeps its own lockfile and `node-linker=hoisted`). `src/shared/api/types.ts` re-exports
the contract shapes and adds the few view models the screens use (money as minor units, names
joined in); `src/shared/api/live/mappers.ts` is the only DTO → view-model translation.

**Mock and live.** `EXPO_PUBLIC_API_MODE` (`mock` default, `live`) and `EXPO_PUBLIC_API_URL`
(empty = Metro's host on port 18080) pick the API at build time; in development builds
Settings → Desarrollador overrides both on the device (persisted, signs out on apply; a release
build ignores the override). `pnpm test:live` (in `apps/mobile`) drives the live client against
the running stack: login, reads, payout with password step-up, approval with device-key step-up
and SSE events. Step-up: an Ed25519 device key (`@noble/ed25519`, secret from `expo-crypto`, kept
in SecureStore) is registered with `POST /devices` after sign-in on phones with biometrics; the
password stays as fallback.

## 8. Swapping fakes for real services

| Port | Fake (MVP) | Real (later) |
|---|---|---|
| `psp.Provider` | HTTP adapter → `apps/bind-mock` | same HTTP adapter → BIND base URLs + Entra OAuth |
| `kyc.Provider` | in-process fake | RENAPER validation vendor |
| `erp.Directory` (identity index, projects, buyers, buyer installments) | JSON fixtures / dump import | CDC (Debezium) read model or ERP API |
| `idp` | local EdDSA IdP | Cognito user pool shared with the ERP |
| `notify.Sender` | log + dev inbox endpoint | SES / Expo push service |

### ERP identity index: loading it

`erp_identity_index` is filled by the worker's periodic `erp_index_refresh` job (on start, then
every 6 h), which upserts every entry the `erplink.Directory` port returns. Today the port is
the fixture directory (`apps/api/fixtures/erp/*.json`, `ERP_FIXTURES_DIR`; any ERP user
accepts the demo password `lebane123`). Loading the ERP dump needs no new command: a
`Directory` adapter that reads the dump (or the CDC read model) replaces the fixture one in
`app.FakePorts`, and the same job loads it. Until that adapter exists there is no dump loader.

### ERP users import

The fixture of each ERP org lists its users (`username`, `email`, `fullName`, `admin`,
`permissions`) with Lebane's real permission codes (`PermisoUsuario` in lebane-back; payments
codes exist for the developer vertical and as `CONSTRUCTORA_*` twins for builders, grouped in
`PermisosPaymentsPorVertical`). The import maps them with `erplink.RoleForERPUser` (one function,
spec `model_roles_test.go`):

| ERP user | Lenapay role |
|---|---|
| ERP admin flag, `CONFIGURACION_ORGANIZACION` or `PAYMENTS_RECONCILIATIONS_MASTER` | `admin` |
| `PAYMENTS_PAYOUTS`, `PAYMENTS_CREATE_ACCOUNTS`, `PAYMENTS_CREATE_CVU` or `PAYMENTS_ACCOUNT_MOVEMENTS` (or their `CONSTRUCTORA_` twins) | `operator` |
| only `PAYMENTS_RECONCILIATIONS` (read-only) | `viewer` |
| no payments permission (only `CONTABILIDAD`, `COMPRAS`, …) | not invited (counted in `usersWithoutPaymentsAccess`) |

Nobody maps to `approver`: Lebane has no payments approval permission (`APROBACIONES` /
`GENERAL_APROBACIONES` approve purchase documents, not payments), so ERP payers come in as
operators and the owner or an admin promotes approvers in Lenapay (proposal, G11). The role is
capped at the linking admin's (an owner linker caps at `admin`) and is never `owner`; users who are
already members are skipped and a pending invite is never duplicated (River retries). The invites
(`source = erp_import`) are emailed like any invite and accepted automatically: inside the email
verification transaction for a new account, and at login for an account that already existed.
Only these invites auto-accept; a manual invite still needs its token (proposal: auto-accepting
every invite would let an admin add any registered email to an org without their action). A link
imported before this existed has no users in its summary (the import does not run again).
