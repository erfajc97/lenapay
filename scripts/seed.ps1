# Loads the demo dataset into the running compose stack (idempotent: run it again any time).
# Usage (PowerShell, repo root):  ./scripts/seed.ps1           # inside the api image
#                                 ./scripts/seed.ps1 -OnHost     # on the host: go run ./cmd/seed
# The stack must be up first: docker compose up -d --build --wait
param([switch]$OnHost)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
try {
    if ($OnHost) {
        if (-not (Test-Path .env)) { Copy-Item .env.example .env }
        # Host runs read .env through the environment (like `make seed` does with `include .env`).
        Get-Content .env | Where-Object { $_ -match '^\s*[A-Z_][A-Z0-9_]*=' } | ForEach-Object {
            $name, $value = $_ -split '=', 2
            [Environment]::SetEnvironmentVariable($name.Trim(), $value)
        }
        # An older .env may predate SEED_DEMO_PASSWORD: fall back to the published demo password.
        if (-not $env:SEED_DEMO_PASSWORD) { $env:SEED_DEMO_PASSWORD = 'Lenapay2026!' }
        Push-Location apps/api
        try { go run ./cmd/seed } finally { Pop-Location }
    } else {
        docker compose run --rm --entrypoint /app/seed api
    }
    if ($LASTEXITCODE -ne 0) { throw "seed failed with exit code $LASTEXITCODE" }
} finally {
    Pop-Location
}
