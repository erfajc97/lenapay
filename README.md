# Lenapay

Fintech wallet with CVU accounts in ARS and USD for Lebane's clients (and, later, individuals).
Requirements: `docs/prd.md` · design: `docs/architecture.md` · plan: `docs/backlog.md` ·
rules: `CLAUDE.md`.

```
contracts/openapi.yaml   the ONE API contract (edit first, then `make gen`)
apps/api                 Go API (+ River worker, migrate, seed commands)
apps/bind-mock           Go fake of the BIND PSP (local only)
apps/mobile              Expo app
packages/api-types       TypeScript types generated from the contract (@lenapay/api-types)
```

## Quick start

Requirements: Docker, Go 1.27, pnpm (for type generation), `make` (Git Bash / WSL on Windows,
or run the commands of the Makefile by hand).

```sh
make up        # copies .env.example → .env, builds and boots postgres, api, worker, bind-mock; waits until healthy
curl http://localhost:18080/healthz         # {"status":"ok"}  (the API's host port is 18080)
curl http://localhost:8090/healthz          # bind-mock
make seed      # demo dataset (idempotent); see "Demo" below
node scripts/smoke.mjs                      # end-to-end smoke of the running stack (PASS/FAIL table)
make down      # stop (keeps the database volume); `make reset` wipes it
```

Without `make` (PowerShell or any shell): `docker compose up -d --build --wait` works with or
without a `.env` (every variable has a development default), then `./scripts/seed.ps1`.
The API listens on host port **18080** by default (8080 is often taken; the container still
listens on 8080 inside the compose network). Host ports come from `API_PORT`, `POSTGRES_PORT`
and `BINDMOCK_PORT`. In Git Bash, prefix `docker compose run ... /app/seed` with
`MSYS_NO_PATHCONV=1` (otherwise the path is rewritten to a Windows path).

The API applies migrations at boot in compose (`MIGRATE_ON_START=true`); production runs
`cmd/migrate` as a deploy step. Without `JWT_PRIVATE_KEY` the local identity provider uses an
ephemeral key in development (tokens die on restart): set one with `openssl rand -base64 32`.

Run a process on the host against the compose database: `make api`, `make worker`,
`make bindmock`, `make migrate` (the host API also listens on :18080, see `.env.example`).

## Demo

`make seed` (or `./scripts/seed.ps1`; `./scripts/seed.ps1 -OnHost` runs `go run ./cmd/seed` on
the host) loads, through the real services and the bind-mock's signed webhooks, a demo dataset
into the running stack (it needs the worker: payins are credited by its jobs). Running it again
changes nothing.

| User (password `Lenapay2026!`, **demo only**) | Role |
|---|---|
| `demo@lenapay.app` | "Ricardo Benítez", **owner** of **Desarrollos Del Sur SA** (CUIT 30-71234567-8, KYC approved, found in the ERP fixture index); its approval policy approves company payouts strictly below ARS 500.000 without a second person |
| `approver@lenapay.app` | "Carolina Méndez", approver of the same org; created the two payouts waiting for `demo@` to approve (made before the policy) and one below the threshold that settled at once |
| `persona@lenapay.app` | personal wallet "Lucía Fernández" (personal mode on, DNI 30.123.456), with ARS and USD CVUs and ARS 3.000.000 to pay with; "Mis pagos" shows 2 companies |
| `andina@lenapay.app` | "Hernán Quiroga", owner of **Constructora Andina SRL** (CUIT 30-71553201-4), linked to its ERP org (fixtures `erp-org-002`); the import invited its 4 ERP users with payments permissions |
| `tesoreria@constructoraandina.com.ar` | "Paula Sosa", an ERP user the Andina import invited as operator: she joined just by signing up and verifying her email |

Dataset: project accounts "Torre Palermo" and "Barrio Nordelta Lote 12"; 8 payers with ARS CVUs
(3 also USD), one of them Lucía by her DNI; 17 payins credited through the ledger; 4 payouts (2 pending approval, 1 settled
with its proof PDF, 1 below the ARS 500.000 threshold approved at creation and settled); an internal transfer; a reconciliation run.
Roles: every org has exactly one owner (its creator); only the owner changes the approval policy,
transfers ownership and demotes/removes admins. "Mis pagos": the ERP import
of Constructora Andina creates Lucía as a buyer by her CUIT 27-30123456-8 (12 CAC installments in
the fixture), with an ARS CVU and 2 payins, so `GET /me/payments/companies` as `persona@` lists
both companies with history, and the Andina one with installments. Email codes are always `123456`
(`DEV_FIXED_EMAIL_CODE`) and a submitted company KYC is auto-approved after 8 s
(`DEV_KYC_AUTO_APPROVE_SECONDS`); both are development-only and refused/ignored in production.
With personal mode on (the compose default) every user — including the company users above —
gets a personal org (KYC draft) and personal account at signup; the personal CVUs (ARS, USD)
appear on that account (`GET /orgs/{org}/accounts` → `cvus`) once the person KYC is approved.
A payout to a Lenapay CVU or alias is on-us (`rail: internal`): it never reaches the bind-mock and
settles at once (after the 4-eyes approval for companies), and the receiver sees a credited payin.

`node scripts/smoke.mjs` walks the whole path with fresh users (signup → KYC → CVU → payin →
SSE → payout → second approver → settle → attachment + proof → reconciliation → approval policy
threshold (an approver changing it gets 403) → ownership transfer and back → profile and preferences
→ the seeded Andina ERP users import and its auto-accepted invites → "Mis pagos": a verified person
sees, hides/shows and pays the company on-us) and exits 1 on any failure (run the seed first); `node scripts/p95.mjs` times the hot reads (R-NF-1) on the seeded stack.

## Testing

```sh
make test               # go vet + unit/router tests of both Go modules
make test-integration   # + real Postgres 17 through testcontainers (Docker up)
make cover              # full suite with the 70 % coverage gate (.testcoverage.yml)
make lint               # golangci-lint v2.15.0 (pinned image) on both modules
```

`-race` needs cgo: CI and Linux/macOS run `go test -race ./...`; on Windows without a C
toolchain run it in the Go image:
`docker run --rm -v "$PWD:/repo" -w /repo/apps/api golang:1.27 go test -race ./...`.

## Code generation

`make gen` copies the contract into `apps/api/api/openapi.yaml` (embedded for the validator,
the guard and the tests), regenerates the strict server (`go tool oapi-codegen` → 
`apps/api/internal/api`), the sqlc stores (`sqlc/sqlc:1.31.1` → `internal/<feature>/store`) and
the TypeScript types (`openapi-typescript@7.13.0` → `packages/api-types/src/index.ts`). Never
edit generated files.
