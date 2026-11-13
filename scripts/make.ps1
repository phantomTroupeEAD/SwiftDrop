# ==============================================================================
# SwiftDrop Windows PowerShell Make Automation Helper
# Author: Member 9 (DevOps, Kubernetes & Database Coordinator)
# Mirror of root Makefile targets for native Windows environments
# ==============================================================================

param (
    [Parameter(Position=0)]
    [ValidateSet("help", "up", "down", "restart", "ps", "logs", "migrate-up", "seed", "test", "swagger", "test-e2e", "reset")]
    [string]$Target = "help"
)

$ComposeFile = "deploy/docker-compose.yml"

switch ($Target) {
    "help" {
        Write-Host "SwiftDrop PowerShell Automation Helper" -ForegroundColor Cyan
        Write-Host "---------------------------------------" -ForegroundColor Cyan
        Write-Host ".\scripts\make.ps1 up          - Start all containers in background"
        Write-Host ".\scripts\make.ps1 down        - Stop all running containers"
        Write-Host ".\scripts\make.ps1 restart     - Restart all containers"
        Write-Host ".\scripts\make.ps1 ps          - Display container health & status"
        Write-Host ".\scripts\make.ps1 logs        - Follow logs across all containers"
        Write-Host ".\scripts\make.ps1 migrate-up  - Run database migrations"
        Write-Host ".\scripts\make.ps1 seed        - Load deploy/seeds/*.sql into the running database"
        Write-Host ".\scripts\make.ps1 test        - Run Go test suite with race detector"
        Write-Host ".\scripts\make.ps1 swagger     - Regenerate unified Swagger/OpenAPI docs in core-api"
        Write-Host ".\scripts\make.ps1 test-e2e    - Run the Playwright (TypeScript) E2E suite"
        Write-Host ".\scripts\make.ps1 reset       - Wipe volumes and stop containers"
    }
    "up" {
        docker compose -f $ComposeFile up -d
    }
    "down" {
        docker compose -f $ComposeFile down
    }
    "restart" {
        docker compose -f $ComposeFile restart
    }
    "ps" {
        docker compose -f $ComposeFile ps
    }
    "logs" {
        docker compose -f $ComposeFile logs -f
    }
    "migrate-up" {
        docker compose -f $ComposeFile run --rm core-api-migrate
    }
    "seed" {
        # Load every deploy/seeds/*.sql into the running postgres container (seeds are idempotent)
        Get-ChildItem -Path "deploy/seeds" -Filter "*.sql" | Sort-Object Name | ForEach-Object {
            Write-Host "Seeding $($_.Name)"
            Get-Content -Raw $_.FullName | docker compose -f $ComposeFile exec -T postgres sh -c 'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB"'
            if ($LASTEXITCODE -ne 0) { throw "Seed failed: $($_.Name)" }
        }
    }
    "test" {
        Push-Location core-api
        try {
            go test -v -race ./...
        } finally {
            Pop-Location
        }
    }
    "swagger" {
        Push-Location core-api
        try {
            go run github.com/swaggo/swag/cmd/swag@v1.16.3 init -g cmd/api/main.go -o docs --ot json,yaml --parseDependency --parseInternal
            Copy-Item -Path "docs/swagger.json" -Destination "internal/events/swagger/swagger.json" -Force
        } finally {
            Pop-Location
        }
    }
    "test-e2e" {
        Push-Location tests/e2e
        try {
            npx playwright test
        } finally {
            Pop-Location
        }
    }
    "reset" {
        docker compose -f $ComposeFile down -v
    }
}

