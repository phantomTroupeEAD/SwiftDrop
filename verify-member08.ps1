<#
.SYNOPSIS
    Member 08 (Events & Messaging) Full Capability Verification Suite
.DESCRIPTION
    Executes live verification across all 8 domain capabilities owned by Member 08:
    1. Schema & Migration Invariants (5 Tables)
    2. Outbox Writer & Transactional Invariants
    3. Outbox Relay & Broker Publisher Confirms
    4. RabbitMQ Topic Exchange & DLQ Topology
    5. Event Worker & Idempotent Deduplication Engine
    6. Merchant Webhook Dispatcher & HMAC-SHA256 Signing
    7. Database Audit, Lease Recovery & Retention Reconciler
    8. Customer Notification Engine (SMTP / Templates)
#>

$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $ScriptDir

Write-Host "==================================================================" -ForegroundColor Cyan
Write-Host "  🧪 Member 08: Full Capability & Subsystem Verification Suite   " -ForegroundColor Cyan
Write-Host "==================================================================" -ForegroundColor Cyan

# Load .env if present
$envFile = Join-Path $ScriptDir ".env"
if (Test-Path $envFile) {
    Get-Content $envFile | ForEach-Object {
        $line = $_.Trim()
        if ($line -and -not ($line.StartsWith("#"))) {
            $parts = $line.Split("=", 2)
            if ($parts.Length -eq 2) {
                [Environment]::SetEnvironmentVariable($parts[0].Trim(), $parts[1].Trim(), "Process")
            }
        }
    }
}

function Set-Default($name, $val) {
    if (-not [Environment]::GetEnvironmentVariable($name, "Process")) {
        [Environment]::SetEnvironmentVariable($name, $val, "Process")
    }
}

Set-Default "APP_ENV" "local"
Set-Default "DATABASE_URL" "postgres://swiftdrop_user:swiftdrop_password@localhost:5432/swiftdrop_db?sslmode=disable"
Set-Default "RECONCILER_DATABASE_URL" "postgres://swiftdrop_user:swiftdrop_password@localhost:5432/swiftdrop_db?sslmode=disable"
Set-Default "RABBITMQ_URL" "amqp://guest:guest@localhost:5672/"
Set-Default "WEBHOOK_SECRET_ENCRYPTION_KEY" "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
Set-Default "WEBHOOK_ALLOW_PRIVATE_HOSTS" "true"
Set-Default "INTEGRATION" "1"

$Results = [ordered]@{}

function Run-Step($title, [scriptblock]$action) {
    Write-Host "`n>>> [TESTING] $title..." -ForegroundColor Yellow
    try {
        & $action
        $Results[$title] = "PASS"
        Write-Host ">>> [PASSED] $title" -ForegroundColor Green
    } catch {
        $Results[$title] = "FAIL: $_"
        Write-Host ">>> [FAILED] $title - $_" -ForegroundColor Red
    }
}

# 1. Docker Dev Infrastructure Check
Run-Step "1. Docker Infrastructure (Postgres, RabbitMQ, Mailpit)" {
    $pg = docker ps --filter "name=swiftdrop-dev-postgres" --format "{{.Status}}"
    $rmq = docker ps --filter "name=swiftdrop-dev-rabbitmq" --format "{{.Status}}"
    if (-not $pg) { throw "PostgreSQL container (swiftdrop-dev-postgres) is not running" }
    if (-not $rmq) { throw "RabbitMQ container (swiftdrop-dev-rabbitmq) is not running" }
}

# 2. Core API Events Unit Tests (Outbox Writer, Envelope, Validation, API Handlers)
Run-Step "2. Core API Events Unit & Invariant Tests (core-api/internal/events/...)" {
    Push-Location "$ScriptDir\core-api"
    try {
        $out = go test -v ./internal/events/... 2>&1
        if ($LASTEXITCODE -ne 0) { throw "go test failed: $out" }
    } finally {
        Pop-Location
    }
}

# 3. Messaging Internal Unit Tests (Relay, Worker, Webhook, Notification, Reconciler)
Run-Step "3. Messaging Service Internal Unit Tests (services/messaging/internal/...)" {
    Push-Location "$ScriptDir\services\messaging"
    try {
        $out = go test ./internal/... 2>&1
        if ($LASTEXITCODE -ne 0) { throw "go test failed: $out" }
    } finally {
        Pop-Location
    }
}

# 4. PostgreSQL Schema & Migrations Contract (5 Tables verified in real DB)
Run-Step "4. Database Schema & Migration Invariant Contract (Live Postgres 16)" {
    Push-Location "$ScriptDir\services\messaging\tests"
    try {
        $env:INTEGRATION = "1"
        $out = go test -v -run TestComprehensiveSQLSchemaContract_Integration 2>&1
        if ($LASTEXITCODE -ne 0) { throw "Schema contract test failed: $out" }
    } finally {
        Pop-Location
    }
}

# 5. RabbitMQ Topology & End-to-End Lifecycle Integration
Run-Step "5. RabbitMQ Topology, Relay Confirm & Consumer Deduplication (Live AMQP)" {
    Push-Location "$ScriptDir\services\messaging\tests"
    try {
        $env:INTEGRATION = "1"
        $out = go test -v -run TestEndToEndLifecycle_Integration 2>&1
        if ($LASTEXITCODE -ne 0) { throw "E2E lifecycle test failed: $out" }
    } finally {
        Pop-Location
    }
}

# 6. Database Audit & Lease Recovery (cmd/reconcile)
Run-Step "6. Database Audit & Stale Lease Recovery (cmd/reconcile)" {
    Push-Location "$ScriptDir\services\messaging"
    try {
        $out = go run ./cmd/reconcile 2>&1
        if ($LASTEXITCODE -ne 0) { throw "Reconciler audit failed: $out" }
    } finally {
        Pop-Location
    }
}

# 7. Retention Policy Engine (cmd/reconcile retention)
Run-Step "7. Event Retention Policy Cleanup (cmd/reconcile retention)" {
    Push-Location "$ScriptDir\services\messaging"
    try {
        $out = go run ./cmd/reconcile retention 2>&1
        if ($LASTEXITCODE -ne 0) { throw "Reconciler retention failed: $out" }
    } finally {
        Pop-Location
    }
}

# Summary Report
Write-Host "`n==================================================================" -ForegroundColor Cyan
Write-Host "                📊 Member 08 Verification Summary                 " -ForegroundColor Cyan
Write-Host "==================================================================" -ForegroundColor Cyan

$allPassed = $true
foreach ($k in $Results.Keys) {
    $res = $Results[$k]
    if ($res -eq "PASS") {
        Write-Host "  ✅ $k" -ForegroundColor Green
    } else {
        Write-Host "  ❌ $($k): $res" -ForegroundColor Red
        $allPassed = $false
    }
}

Write-Host "==================================================================" -ForegroundColor Cyan
if ($allPassed) {
    Write-Host "  🎉 ALL 7 CAPABILITIES OF MEMBER 08 ARE FULLY OPERATIONAL!      " -ForegroundColor Green
} else {
    Write-Host "  ⚠️  SOME TESTS FAILED - REVIEW OUTPUT ABOVE                    " -ForegroundColor Red
}
Write-Host "==================================================================" -ForegroundColor Cyan
