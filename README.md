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

## Demo en 5 minutos

Guion para presentar Lenapay en una PC con Windows (PowerShell). Los comandos funcionan igual en
Git Bash, macOS o Linux salvo donde se indica.

### 1. Requisitos

- Docker Desktop corriendo (Postgres, API, worker y bind-mock van en contenedores; no hace falta Go).
- Node.js 22 o superior (para `scripts/smoke.mjs` y para la app) y pnpm (`corepack enable`).
- Para ver la app: un teléfono con Expo Go del SDK 58 en la misma red Wi-Fi que la PC, o un
  development build (`eas build --profile development`, cuando haya acceso a EAS), o el navegador.

### 2. Levantar el backend, cargar datos y verificarlo

Desde la raíz del repo (`C:\dev\lenapay`), un solo comando:

```powershell
pnpm dev:up      # abre Docker Desktop si está cerrado, levanta el stack, espera que esté sano y carga el seed
pnpm dev:smoke   # opcional: recorrido punta a punta, 16/16 PASS
```

Otros atajos de la raíz: `pnpm dev:down` (apaga sin borrar datos), `pnpm dev:reset` (borra la base y
vuelve a sembrar), `pnpm dev:logs`, `pnpm dev:ps`, `pnpm dev:mobile`, `pnpm dev:mobile:clear`
(Metro limpiando caché) y `pnpm dev:web`. Paso a paso, lo mismo es:

```powershell
docker compose up -d --build --wait     # postgres, api (:18080), worker, bind-mock (:8090); espera a que estén sanos
curl http://localhost:18080/healthz      # {"status":"ok"}
./scripts/seed.ps1                       # datos de demo (idempotente); en Git Bash/macOS/Linux: make seed
node scripts/smoke.mjs                   # recorrido punta a punta con usuarios nuevos: tabla PASS/FAIL, exit 0
```

`docker compose up` funciona sin `.env` (todas las variables tienen valor de desarrollo). El seed
necesita el worker arriba: los cobros se acreditan con sus jobs.

### 3. Correr la app

En tu propia terminal (Metro queda corriendo):

```powershell
cd apps/mobile
pnpm install
pnpm dev                                 # expo start: escaneá el QR con Expo Go; "w" abre la versión web
```

**Mock o live.** Sin `.env` la app arranca en **mock**: un backend en memoria con datos propios y
el distintivo "Modo demo". Para usar la API real, copiá `apps/mobile/.env.example` a
`apps/mobile/.env` y poné `EXPO_PUBLIC_API_MODE=live`; con `EXPO_PUBLIC_API_URL` vacío la app usa
`http://<host de Metro>:18080`, que en el teléfono es la IP de la PC. Reiniciá `pnpm dev` después
de cambiar el `.env`.

**Cambiar en caliente (builds de desarrollo).** Menú del avatar en Inicio → **Configuración** →
**Desarrollador** (antes de ingresar: el link "Desarrollador" de la pantalla de bienvenida). Ahí
se elige "Demo (mock)" o "API real (live)", la URL de la API, "Probar conexión" y "Aplicar y cerrar
sesión". El cambio queda guardado en el teléfono; un build de release lo ignora.

**Teléfono contra la PC (live).**

1. IP de la PC en la red local: `ipconfig` → "Dirección IPv4" del adaptador Wi-Fi (por ejemplo
   `192.168.68.53`). La URL de la API es `http://192.168.68.53:18080`.
2. Abrí los puertos de la API y de Metro en el firewall de Windows (PowerShell como
   administrador, una sola vez; perfil de red "Privada"):

   ```powershell
   New-NetFirewallRule -DisplayName "Lenapay API 18080" -Direction Inbound -Protocol TCP -LocalPort 18080 -Action Allow -Profile Private
   New-NetFirewallRule -DisplayName "Expo Metro 8081" -Direction Inbound -Protocol TCP -LocalPort 8081 -Action Allow -Profile Private
   ```

3. Desde el navegador del teléfono, `http://192.168.68.53:18080/healthz` tiene que responder
   `{"status":"ok"}`; si no, revisá que la red sea "Privada" y que el teléfono esté en la misma Wi-Fi.

En la web (`pnpm web` o "w") la API solo acepta los orígenes `http://localhost:8081` y
`http://localhost:19006` (`CORS_ORIGINS`).

### 4. Usuarios de demo y qué mostrar con cada uno

Contraseña de todos: `Lenapay2026!` (solo demo). El código de verificación por email siempre es
`123456`. La confirmación de operaciones (step-up) pide la huella o Face ID en el teléfono y la
contraseña en la web.

| Usuario | Quién es | Qué mostrar |
|---|---|---|
| `demo@lenapay.app` | Ricardo Benítez, **propietario** (owner) de Desarrollos Del Sur SA | Inicio con saldos ARS/USD y cuentas de proyecto. Más → **Política de aprobación**: umbral de ARS 500.000 (solo el propietario lo edita, con step-up). **Pagar** menos de ARS 500.000: la revisión dice "Sale sin aprobación (debajo de ARS 500.000)" y el pago se envía sin esperar a nadie. Más → **Aprobaciones**: los 2 pagos que cargó Carolina esperan su aprobación (aprobar con step-up). Más → **Equipo**: distintivo de propietario y "Transferir la propiedad". |
| `approver@lenapay.app` | Carolina Méndez, aprobadora de la misma empresa | Cuatro ojos: un pago de ARS 500.000 o más queda "Enviado para aprobación" y ella no puede aprobar el suyo. La política de aprobación se ve en solo lectura. |
| `persona@lenapay.app` | Lucía Fernández, billetera personal (DNI 30.123.456) | Inicio → **Mi CVU** (ARS y USD). **Mis pagos**: las 2 empresas que la registraron como pagadora. Constructora Andina → **Cuotas** (CAC) → **Pagar cuota** → step-up: pago on-us, "Lenapay instantánea", sin pasar por el banco. Ocultar y volver a mostrar una empresa. |
| `andina@lenapay.app` | Hernán Quiroga, propietario de Constructora Andina SRL | Más → **Vincular con Lebane ERP**: empresa vinculada y resumen de la importación (usuarios del ERP invitados). Más → **Equipo**: los usuarios importados con su rol. `tesoreria@constructoraandina.com.ar` ya entró con solo registrarse y verificar su email. |

### 5. Qué está simulado

| Pieza | En la demo | En producción |
|---|---|---|
| Banco / PSP (BIND) | `apps/bind-mock`: contenedor que emite CVU, recibe transferencias y avisa con webhooks firmados, como BIND | API de BIND (mismo adaptador HTTP, otras URLs y credenciales) |
| KYC (RENAPER) | Validador falso dentro de la API: la KYC de empresa se aprueba sola a los 8 s (`DEV_KYC_AUTO_APPROVE_SECONDS`); el DNI `11111111` se rechaza | Proveedor de validación de identidad |
| Lebane ERP | Fixtures JSON en `apps/api/fixtures/erp/` (empresas, compradores, cuotas, usuarios; contraseña ERP de demo `lebane123`) | Réplica CDC o API del ERP |
| Email y push | No se envía nada: el código siempre es `123456` (`DEV_FIXED_EMAIL_CODE`) y la "bandeja de desarrollo" es el log de la API (`docker compose logs api`, atributo `dev_inbox`, `DEV_INBOX_ENABLED`) | SES y el servicio de push de Expo |
| Identidad | Proveedor de identidad local (tokens EdDSA) | Cognito compartido con el ERP |
| Dinero | **Real dentro de Lenapay**: ledger de doble entrada en Postgres, retenciones, comisiones e idempotencia | Igual |

Las opciones de demo (`DEV_*`, `SEED_DEMO_PASSWORD`) se rechazan o ignoran en producción.

## Demo

`make seed` (or `./scripts/seed.ps1`; `./scripts/seed.ps1 -OnHost` runs `go run ./cmd/seed` on
the host) loads, through the real services and the bind-mock's signed webhooks, a demo dataset
into the running stack (it needs the worker: payins are credited by its jobs). Running it again
changes nothing. The demo password is `SEED_DEMO_PASSWORD` (default `Lenapay2026!` from
`.env.example`, `docker-compose.yml` and `scripts/seed.ps1`); the seed refuses to run without it.

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
