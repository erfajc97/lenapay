# Brings the whole local backend up with one command (pnpm dev:up from the repo root):
# opens Docker Desktop when the engine is down, starts the compose stack (postgres :5433,
# api :18080, worker, bind-mock :8090), waits until healthy and loads the demo seed (idempotent).
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
try {
    function Test-DockerEngine {
        # Windows PowerShell 5 turns a native command's stderr into an error under 'Stop'.
        $ErrorActionPreference = 'Continue'
        docker info --format '{{.ServerVersion}}' 2>&1 | Out-Null
        return $LASTEXITCODE -eq 0
    }

    if (-not (Test-DockerEngine)) {
        $dockerExe = "$env:LOCALAPPDATA\Programs\DockerDesktop\Docker Desktop.exe"
        if (-not (Test-Path $dockerExe)) { $dockerExe = 'C:\Program Files\Docker\Docker\Docker Desktop.exe' }
        Write-Host 'Starting Docker Desktop...'
        Start-Process $dockerExe
        $deadline = (Get-Date).AddMinutes(3)
        while (-not (Test-DockerEngine)) {
            if ((Get-Date) -gt $deadline) { throw 'Docker engine did not start within 3 minutes' }
            Start-Sleep -Seconds 3
        }
    }

    Write-Host 'Starting the compose stack...'
    docker compose up -d --build --wait
    if ($LASTEXITCODE -ne 0) { throw "docker compose up failed with exit code $LASTEXITCODE" }

    Write-Host 'Loading the demo seed...'
    docker compose run --rm --entrypoint /app/seed api
    if ($LASTEXITCODE -ne 0) { throw "seed failed with exit code $LASTEXITCODE" }

    docker compose ps --format 'table {{.Service}}\t{{.Status}}\t{{.Ports}}'
    Write-Host 'Ready: API http://localhost:18080/healthz  |  Postgres localhost:5433  |  bind-mock :8090'
} finally {
    Pop-Location
}
