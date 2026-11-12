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
        Write-Host ".\scripts\make.ps1 seed        - Seed mock users & roles"
        Write-Host ".\scripts\make.ps1 test        - Run Go test suite with race detector"
        Write-Host ".\scripts\make.ps1 swagger     - Regenerate unified Swagger/OpenAPI docs in core-api"
        Write-Host ".\scripts\make.ps1 test-e2e    - Run Python Playwright E2E tests"
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
        docker compose -f $ComposeFile exec core-api /app/migrate
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
            go run github.com/swaggo/swag/cmd/swag@latest init -g cmd/api/main.go -o docs --ot json,yaml --parseDependency --parseInternal
            Copy-Item -Path "docs/swagger.json" -Destination "internal/events/swagger/swagger.json" -Force
        } finally {
            Pop-Location
        }
    }
    "test-e2e" {
        pytest tests/e2e/ -v
    }
    "reset" {
        docker compose -f $ComposeFile down -v
    }
}

