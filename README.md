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
curl http://localhost:8080/healthz          # {"status":"ok"}
curl http://localhost:8090/healthz          # bind-mock
make down      # stop (keeps the database volume); `make reset` wipes it
```

Without `make`: `cp .env.example .env` then `docker compose up -d --build --wait`. Host ports
come from `.env` (`API_PORT`, `POSTGRES_PORT`, `BINDMOCK_PORT`); if 8080 is taken:
`API_PORT=18080 docker compose up -d --build --wait`.

The API applies migrations at boot in compose (`MIGRATE_ON_START=true`); production runs
`cmd/migrate` as a deploy step. Without `JWT_PRIVATE_KEY` the local identity provider uses an
ephemeral key in development (tokens die on restart): set one with `openssl rand -base64 32`.

Run a process on the host against the compose database: `make api`, `make worker`,
`make bindmock`, `make migrate`, `make seed`.

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
