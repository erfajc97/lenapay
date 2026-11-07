# Lenapay monorepo tasks. Pinned tools: oapi-codegen, goose, go-test-coverage and govulncheck
# are `tool` directives in apps/api/go.mod (`go tool …`); sqlc and golangci-lint run as pinned
# Docker images (they need cgo / a fixed release); openapi-typescript runs with pnpm dlx.
SQLC_IMAGE     := sqlc/sqlc:1.31.1
GOLANGCI_IMAGE := golangci/golangci-lint:v2.15.0
API            := apps/api
BINDMOCK       := apps/bind-mock

# Host runs (make api / worker / migrate / bindmock) read .env through the environment.
ifneq (,$(wildcard .env))
include .env
export
endif

.PHONY: gen test test-integration cover lint up down reset seed migrate api worker bindmock env

## gen: regenerate everything from the contract and the migrations (P1)
gen:
	cp contracts/openapi.yaml $(API)/api/openapi.yaml
	cd $(API) && go generate ./internal/api
	docker run --rm -v "$(CURDIR)/$(API):/src" -w /src $(SQLC_IMAGE) generate
	cd packages/api-types && pnpm dlx openapi-typescript@7.13.0 ../../contracts/openapi.yaml --output src/index.ts

## test: unit + router tests of both Go modules (add -race where cgo is available: CI, Linux, macOS)
test:
	cd $(API) && go vet ./... && go test ./...
	cd $(BINDMOCK) && go vet ./... && go test ./...

## test-integration: everything, including real Postgres through testcontainers (Docker up)
test-integration:
	cd $(API) && go vet -tags=integration ./... && go test -tags=integration ./...

## cover: the G2 coverage gate on the full suite
cover:
	cd $(API) && go test -tags=integration -coverpkg=./internal/... -coverprofile=cover.out ./... \
		&& go tool go-test-coverage --config=.testcoverage.yml

## lint: golangci-lint (pinned image) on both modules
lint:
	docker run --rm -v "$(CURDIR)/$(API):/src" -w /src $(GOLANGCI_IMAGE) golangci-lint run ./...
	docker run --rm -v "$(CURDIR)/$(BINDMOCK):/src" -w /src $(GOLANGCI_IMAGE) golangci-lint run ./...

env:
	test -f .env || cp .env.example .env

## up: build and boot postgres, api (migrates at start), worker and bind-mock; wait until healthy
up: env
	docker compose up -d --build --wait

## down: stop the stack, keep the data
down:
	docker compose down

## reset: wipe the database volume and boot again
reset: env
	docker compose down -v
	docker compose up -d --build --wait

## seed: demo data (E9) against the compose database
seed: env
	docker compose run --rm --entrypoint /app/seed api

## migrate: apply migrations against DATABASE_URL from .env (host run)
migrate: env
	cd $(API) && go run ./cmd/migrate

## api / worker / bindmock: run one process on the host (postgres from `make up`)
api: env
	cd $(API) && go run ./cmd/api

worker: env
	cd $(API) && go run ./cmd/worker

bindmock: env
	cd $(BINDMOCK) && go run ./cmd/bind-mock
