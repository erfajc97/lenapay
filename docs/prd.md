# Lenapay — PRD (MVP)

Status: approved scope 2026-11-07 (owner: Erick Jiménez). Requirement ids: `R-<area>-<n>`.

## 1. Product

Lenapay is a fintech wallet with CVU accounts in ARS and USD. Primary users are the companies
that use the Lebane ERP (real-estate developers and builders): they collect from their buyers
through one unique CVU per payer, keep money per project, pay suppliers, and approve payouts
from the phone. The same platform is ready from day one for individual people (B2C behind a
feature flag), like any Argentine wallet that issues CVUs.

Lenapay is independent: a company or a person can sign up, pass KYC and operate without the
ERP. When a verified identity (CUIT/DNI) exists in the ERP, Lenapay offers to link it and import
projects and buyers.

For the MVP every external dependency is simulated behind an adapter: the PSP (BIND), the KYC
provider, the ERP and push delivery. Swapping a fake for the real service must not change the
domain code.

## 2. Users and roles

- **Company member** with a role inside an organization: `viewer`, `operator` (creates payouts,
  CVUs), `approver` (approves payouts), `admin` (team, settings, onboarding), `owner` (exactly one
  per organization: its creator; ownership, approval policy, admins).
- **Person** (B2C, flag `personal_mode`): owner of a personal wallet.
- **Lenapay ops** (internal, no UI in the MVP): reviews KYC through an internal endpoint.

## 3. Requirements

### Identity — Lebane ID
- R-ID-5 Profile and preferences (owner's decision 2026-11-07): optional `fullName` at signup and
  `PATCH /me {fullName}` (null clears; the person KYC approval fills it when empty); `/me` and the
  member lists return it. `GET/PATCH /me/preferences {defaultOrgId, defaultAccountId, locale,
  theme}` per user (the account must belong to that org, the user must be a member; a default org
  the user left reads as unset); `/me` returns them.
- R-ID-1 Sign up with email + password; email verified by a 6-digit code (sent through the
  notification adapter; local fake logs it and exposes it in dev).
- R-ID-2 Login returns a short access token (15 min) + rotating refresh token; MFA by TOTP is
  optional per user.
- R-ID-3 Step-up: money-moving actions (payout create/approve, CVU create, internal transfer)
  require a fresh step-up token (biometric unlock on the device signs a challenge with a device
  key registered at login; fallback: password re-entry).
- R-ID-4 "Sign in with your Lebane account": the identity provider is behind a port; the MVP
  ships a local IdP with JWKS so Cognito (shared with the ERP) can replace it later.

### KYC and onboarding
- R-KYC-1 Company onboarding: CUIT, legal name, legal address, documents (bylaws, AFIP
  constancy), and the representatives' identity (DNI + selfie) — statuses `draft → submitted →
  approved | rejected`.
- R-KYC-2 Person onboarding: DNI + selfie validated through the KYC adapter (fake approves
  valid-looking data, rejects a configurable DNI list).
- R-KYC-3 Nothing that moves money works until the owning org/person is `approved`.

### Organizations and team
- R-ORG-1 A user can belong to several organizations and switch between them.
- R-ORG-2 Admins invite members by email and set their role; invites expire in 7 days.
- R-ORG-3 Single owner (owner's decision 2026-11-07): role ladder viewer < operator < approver <
  admin < owner; the creator is the owner and every org has EXACTLY one (DB constraint; existing
  orgs: the earliest admin became the owner). Only the owner demotes or removes admins; admins
  manage the roles below admin (and may grant up to admin, never above their own); nobody removes
  or demotes the owner through the member doors (`org.last_admin`); `owner` is never granted by
  invite or role change. `POST /orgs/{org}/ownership-transfer {userId}` (owner + step-up) makes
  another member the owner and the previous owner an admin; audited.

### Accounts and ledger
- R-ACC-1 Each approved organization has a main account and one account per project; a person
  has one personal account. Every account holds balances per currency (ARS, USD).
- R-ACC-2 Balances come from a double-entry ledger (immutable postings, sum of each posting =
  0, minor units int64). Available = posted − held.
- R-ACC-3 Create, rename and archive project accounts (archive only with zero balance).
- R-ACC-4 Internal transfer between own accounts, same currency, immediate (ledger only).
- R-ACC-5 Personal account by default: with personal mode on (`PERSONAL_MODE_ENABLED`, on in
  dev/compose, a flag in production) every user gets, at signup and in the same transaction, a
  personal organization (legal name = the email until the person KYC is approved, KYC `draft`,
  the user its admin) with its personal account. A second personal org is `conflict`.

### Payers and CVU
- R-CVU-1 Create a payer (name, CUIT/DNI, email) under a project account and issue a unique CVU
  (and alias) for that payer through the PSP adapter; one CVU per payer per currency.
- R-CVU-2 Edit alias, disable CVU, share CVU/alias (link + QR payload).
- R-CVU-3 A person in personal mode gets one CVU + alias per currency, issued automatically when
  the person KYC is approved.
- R-CVU-4 The person sees her own CVUs (currency, CVU, alias, status, share link/QR) on her personal
  account (`GET /orgs/{org}/accounts` → `cvus`; a project account lists its own receiving CVU there
  if it ever has one, never payers' CVUs) and changes their alias
  (`PATCH /orgs/{org}/accounts/{accountId}/cvus/{cvuId}`; a payer's or another org's CVU is 404).

### Payins (cash-in)
- R-IN-1 The PSP notifies incoming transfers by a signed webhook; Lenapay verifies the signature
  over the raw body, stores the raw event, deduplicates by provider id, re-reads the transfer at
  the provider and credits the right account through the ledger (net of fees).
- R-IN-2 The payer is identified by the receiving CVU; the origin CUIT/name is stored.
- R-IN-3 Realtime: the app receives the payin through SSE and a push notification.

### Payouts (cash-out / transfers)
- R-OUT-1 Create a payout from an account to a CBU/CVU/alias, ARS or USD, with concept and
  optional attachments; destination is resolved at the PSP first (holder name shown).
- R-OUT-2 Fees are computed and shown before confirming; amount + fee is held on creation.
- R-OUT-3 Client-generated Idempotency-Key is mandatory; same key + same payload replays the
  stored response; same key + different payload → 409.
- R-OUT-7 Approval policy per org (owner's decision 2026-11-07): `GET/PUT /orgs/{org}/approval-policy`
  (read by any member; changed only by the owner with step-up, audited) `{mode: always|threshold,
  thresholdMinor per currency, allowOwnerSelfApproval}`. Default: `always` (4-eyes),
  `allowOwnerSelfApproval: true` but effective only while no OTHER member has a role ≥ approver.
  `threshold`: a company payout whose amount (fee not included) is STRICTLY below its currency's
  threshold is approved and dispatched at creation (still Idempotency-Key + step-up + hold; event
  reason `policy_threshold`); a currency without a threshold always needs approval. The owner may
  approve their own payout only when self-approval is allowed and effective (audited
  `payout.self_approval`; the DB keeps 4-eyes otherwise). Personal orgs never need approval.
- R-OUT-4 Company payouts need 4-eyes approval: the creator cannot approve; approvers approve or
  reject with step-up. Personal payouts are sent directly.
- R-OUT-5 Approved payouts are dispatched to the PSP asynchronously (job queue), never inside
  the HTTP request; states `pending_approval → approved → sending → settled | failed |
  rejected | cancelled`; an uncertain PSP answer is re-checked by a sweep, never marked failed
  blindly.
- R-OUT-6 Proof of payment (PDF) for settled payouts.

### Mis pagos — payer view, B2C bridge (owner's brief 2026-11-07)
- R-MP-1 Matching: a person whose personal org passed KYC is linked to every company payer whose
  document is her verified DNI, or a CUIT **or CUIL** whose middle 8 digits are that DNI (7-digit
  DNIs left-padded). Linked at approval (same transaction) and lazily on read for payers created
  later. Never from unverified (draft) KYC data, never a company's CUIT. Another person's link is
  404 on every door.
- R-MP-2 She lists the companies (company name, project, her CVUs there, totals paid per
  currency, last payment, ERP flag, hidden) and each company's payins to her CVUs (cursor
  paginated), and can hide/show a company. Nothing else of the company is exposed.
- R-MP-3 For an ERP-linked company she sees her installment plan (number, due date, amount and
  currency, status pending|paid|overdue, CAC/UVA index); `erpLinked=false` and `[]` otherwise.
- R-MP-4 She pays a company from her personal wallet: a payout to her CVU at that company in the
  amount's currency (Idempotency-Key + step-up, no approval), optionally naming an existing
  installment in the concept (`Cuota N - …`).
- R-MP-5 On-us payments: transfers between Lenapay CVUs/aliases never go through the bank. When a
  payout's destination CVU or alias is a Lenapay one (looked up in our CVUs before the PSP), it is
  routed on the internal rail: same rules as any payout (Idempotency-Key, step-up, hold, 4-eyes
  for company payouts), then settled instantly — as soon as it is approved — as ONE ledger entry
  from the paying account to the receiving account (fee per plan, on-us rates default 0), with no
  PSP call. The payout is `settled` with `rail: internal`; the receiver gets a credited payin of
  the same rail (movements, "Mis pagos" history, realtime `payin.credited`, push). Covers
  person → company, company → company and company → person. Reconciliation ignores the internal
  rail (it never touches the master account). `rail: bank|internal` is on Payout, Quote and
  Movement.

### Fees
- R-FEE-1 Fee plans per organization (payin %, payout fixed + %), defaults per currency; fees are
  posted to Lenapay's revenue account in the same ledger transaction.

### Reconciliation (CVU)
- R-REC-1 Daily (and on demand) reconciliation of the master account: the PSP statement vs the
  ledger, per currency; each movement is `matched`, `missing_in_ledger` or `missing_at_psp`,
  with totals and the difference.
- R-REC-2 Ops/admins can see the reconciliation runs and their differences in the app.

### ERP link
- R-ERP-1 An identity index (doc type + number → ERP org / client refs) answers in
  milliseconds; it is loaded from a fixture now and from the ERP dump/CDC later.
- R-ERP-2 After KYC approval, if the CUIT exists in the index, admins can link the org: an
  extra confirmation is required (ERP credentials check through the ERP adapter, fake in MVP).
- R-ERP-4 ERP users import (owner's decision 2026-11-07): the link's import also invites the ERP
  org's users with the role their Lebane permissions map to (architecture "ERP users import"): never
  above the linking admin's role and never owner; users already members are skipped, users without
  payments access are not invited. Those invites are accepted automatically when that email
  verifies its Lenapay account or logs in. `GET /orgs/{org}/erp-link` → `importSummary` (counts and
  the invited users with their roles).
- R-ERP-3 A linked org imports ERP projects as project accounts and ERP buyers as payers
  (snapshot copy; the app never calls the ERP live).

### Movements and notifications
- R-MOV-1 Movements list per org / account with cursor pagination and filters (type, status,
  currency, dates), detail screen.
- R-NOT-1 Push notifications for payins, approvals needed and payout results (Expo push adapter,
  fake logs).

### Non-functional
- R-NF-1 p95 < 150 ms for reads on local hardware; lists are cursor-paginated.
- R-NF-2 All money is int64 minor units + ISO currency; no floats.
- R-NF-3 Every money mutation is audited (who, what, idempotency key, request id).
- R-NF-4 Everything runs with `docker compose up` (Postgres, API, BIND mock) + Expo dev client.
- R-NF-5 Code, tests and commits in English; UI copy in es-AR through i18n files.
- R-NF-6 Web realtime: CORS allows `Cache-Control`, `X-Requested-With` and `Last-Event-ID` (the web
  EventSource polyfill's headers on the SSE stream).

## 4. Out of scope (MVP)
Cash networks (Rapipago/Pago Fácil), prepaid card, investments/FCI for people, loans, bill
payments, interoperable QR merchant payments, the ERP quota imputation, real BIND/KYC/ERP/
Cognito connections.
