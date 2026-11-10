<#
.SYNOPSIS
    SwiftDrop All-in-One Live Stack Launcher
.DESCRIPTION
    Starts the complete SwiftDrop messaging and API ecosystem in one command:
    - Docker Infrastructure (PostgreSQL 16, RabbitMQ 3.12, Mailpit)
    - Core API & Swagger UI (port 8080)
    - Outbox Relay Daemon (port 8082)
    - Event Worker Daemon (port 8083)
    - Webhook Test Receiver (port 8088)
    - Provides interactive controls to emit live events, run reconciler audits, and view stats.
#>

[CmdletBinding()]
param (
    [switch]$NoEmitOnStart
)

$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $ScriptDir

Write-Host "==================================================================" -ForegroundColor Cyan
Write-Host "         🚀 SwiftDrop All-in-One Live Stack Launcher" -ForegroundColor Cyan
Write-Host "==================================================================" -ForegroundColor Cyan

# ------------------------------------------------------------------------------
# 1. Load Environment Variables (.env support)
# ------------------------------------------------------------------------------
$EnvFile = Join-Path $ScriptDir ".env"
if (Test-Path $EnvFile) {
    Write-Host "[1/5] Loading environment variables from .env..." -ForegroundColor Yellow
    Get-Content $EnvFile | ForEach-Object {
        $line = $_.Trim()
        if ($line -and -not ($line.StartsWith("#"))) {
            $parts = $line.Split("=", 2)
            if ($parts.Length -eq 2) {
                $k = $parts[0].Trim()
                $v = $parts[1].Trim()
                [Environment]::SetEnvironmentVariable($k, $v, "Process")
            }
        }
    }
} else {
    Write-Host "[1/5] No .env found. Using sensible local defaults..." -ForegroundColor Yellow
}

# Set Default Variables if not already set
function Set-DefaultEnv($name, $defaultValue) {
    $current = [Environment]::GetEnvironmentVariable($name, "Process")
    if (-not $current) {
        [Environment]::SetEnvironmentVariable($name, $defaultValue, "Process")
    }
}

Set-DefaultEnv "APP_ENV" "local"
Set-DefaultEnv "LOG_LEVEL" "info"
Set-DefaultEnv "DATABASE_URL" "postgres://swiftdrop_user:swiftdrop_password@localhost:5432/swiftdrop_db?sslmode=disable"
Set-DefaultEnv "RABBITMQ_URL" "amqp://guest:guest@localhost:5672/"
Set-DefaultEnv "WEBHOOK_SECRET_ENCRYPTION_KEY" "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
Set-DefaultEnv "WEBHOOK_SIGNING_SECRET" "whsec_test_secret_for_local_merchant_123"
Set-DefaultEnv "JWT_SECRET" "swiftdrop-development-secret-key-change-in-production-32chars"
Set-DefaultEnv "SMTP_HOST" "localhost"
Set-DefaultEnv "SMTP_PORT" "1025"
Set-DefaultEnv "SMTP_SECURITY" "none"
Set-DefaultEnv "SMTP_FROM_EMAIL" "notifications@swiftdrop.io"
Set-DefaultEnv "SMTP_FROM_NAME" "SwiftDrop Deliveries"
Set-DefaultEnv "PORT" "8080"
Set-DefaultEnv "RELAY_PROBE_PORT" "8082"
Set-DefaultEnv "WORKER_PROBE_PORT" "8083"
Set-DefaultEnv "RECEIVER_PORT" "8088"
Set-DefaultEnv "SWAGGER_ENABLED" "true"

# ------------------------------------------------------------------------------
# 2. Check Docker Daemon & Start Infrastructure Containers
# ------------------------------------------------------------------------------
Write-Host "[2/5] Checking Docker Desktop status..." -ForegroundColor Yellow
try {
    $dockerCheck = docker ps 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "Docker is not running."
    }
} catch {
    Write-Host "`n[ERROR] Docker Desktop is not running or not responding." -ForegroundColor Red
    Write-Host "Please start Docker Desktop and run this script again.`n" -ForegroundColor Red
    exit 1
}

Write-Host "      Starting PostgreSQL, RabbitMQ, and Mailpit containers..." -ForegroundColor Green
docker compose -f "services/messaging/docker-compose.dev.yml" up -d | Out-Null

# Wait briefly for Postgres & RabbitMQ connectivity
Write-Host "      Waiting for PostgreSQL & RabbitMQ readiness..." -NoNewline
for ($i = 0; $i -lt 15; $i++) {
    Start-Sleep -Milliseconds 800
    Write-Host "." -NoNewline
    $pgReady = docker exec swiftdrop-dev-postgres pg_isready -U swiftdrop_user -d swiftdrop_db 2>&1
    if ($pgReady -match "accepting connections") {
        break
    }
}
Write-Host " Ready!" -ForegroundColor Green

# ------------------------------------------------------------------------------
# 3. Launch Services in Background
# ------------------------------------------------------------------------------
Write-Host "[3/5] Launching Go services concurrently..." -ForegroundColor Yellow
$processes = @()
$logDir = Join-Path $ScriptDir ".logs"
if (-not (Test-Path $logDir)) {
    New-Item -ItemType Directory -Path $logDir | Out-Null
}

function Start-ServiceProcess($name, $workingDir, $cmdArgs, $outLog, $errLog) {
    Write-Host "      -> Starting $name..." -ForegroundColor Cyan
    $pinfo = New-Object System.Diagnostics.ProcessStartInfo
    $pinfo.FileName = "go"
    $pinfo.Arguments = $cmdArgs
    $pinfo.WorkingDirectory = $workingDir
    $pinfo.UseShellExecute = $false
    $pinfo.RedirectStandardOutput = $true
    $pinfo.RedirectStandardError = $true
    $pinfo.CreateNoWindow = $true

    # Pass Process environment variables
    foreach ($de in [Environment]::GetEnvironmentVariables("Process").GetEnumerator()) {
        $pinfo.EnvironmentVariables[$de.Key] = [string]$de.Value
    }

    $proc = New-Object System.Diagnostics.Process
    $proc.StartInfo = $pinfo
    $null = $proc.Start()

    # Async log redirection
    $outStream = [System.IO.File]::CreateText((Join-Path $logDir $outLog))
    $errStream = [System.IO.File]::CreateText((Join-Path $logDir $errLog))

    $proc.BeginOutputReadLine()
    $proc.BeginErrorReadLine()

    Register-ObjectEvent -InputObject $proc -EventName OutputDataReceived -Action {
        if ($EventArgs.Data) {
            $Event.MessageData.WriteLine($EventArgs.Data)
            $Event.MessageData.Flush()
        }
    } -MessageData $outStream | Out-Null

    Register-ObjectEvent -InputObject $proc -EventName ErrorDataReceived -Action {
        if ($EventArgs.Data) {
            $Event.MessageData.WriteLine($EventArgs.Data)
            $Event.MessageData.Flush()
        }
    } -MessageData $errStream | Out-Null

    return @{ Name = $name; Process = $proc; OutStream = $outStream; ErrStream = $errStream }
}

try {
    # 1. Webhook Test Receiver (:8088)
    $procReceiver = Start-ServiceProcess `
        "Webhook Receiver (:8088)" `
        (Join-Path $ScriptDir "services/messaging") `
        "run cmd/fake-receiver/main.go -port 8088" `
        "receiver.log" "receiver.err.log"
    $processes += $procReceiver

    # 2. Outbox Relay Daemon (:8082)
    $procRelay = Start-ServiceProcess `
        "Outbox Relay Daemon (:8082)" `
        (Join-Path $ScriptDir "services/messaging") `
        "run cmd/relay/main.go" `
        "relay.log" "relay.err.log"
    $processes += $procRelay

    # 3. Event Worker Daemon (:8083)
    $procWorker = Start-ServiceProcess `
        "Event Worker Daemon (:8083)" `
        (Join-Path $ScriptDir "services/messaging") `
        "run cmd/worker/main.go" `
        "worker.log" "worker.err.log"
    $processes += $procWorker

    # 4. Core API & Swagger UI (:8080)
    $procCore = Start-ServiceProcess `
        "Core API & Swagger (:8080)" `
        (Join-Path $ScriptDir "core-api") `
        "run ./cmd/api" `
        "core-api.log" "core-api.err.log"
    $processes += $procCore

    # Brief delay for servers to bind ports
    Start-Sleep -Seconds 2

    # --------------------------------------------------------------------------
    # 4. Display Live Dashboard & Links
    # --------------------------------------------------------------------------
    Write-Host "`n[4/5] All systems operational! Active Web Endpoints:" -ForegroundColor Green
    Write-Host "------------------------------------------------------------------" -ForegroundColor DarkGray
    Write-Host "  📖 Swagger UI (Events REST API): " -NoNewline; Write-Host "http://localhost:8080/swagger/" -ForegroundColor Yellow
    Write-Host "  ✉️  Mailpit (Customer Emails):   " -NoNewline; Write-Host "http://localhost:8025" -ForegroundColor Yellow
    Write-Host "  🐰 RabbitMQ Management UI:       " -NoNewline; Write-Host "http://localhost:15672" -ForegroundColor Yellow -NoNewline; Write-Host " (guest / guest)" -ForegroundColor DarkGray
    Write-Host "  🔗 Webhook Receiver Endpoint:    " -NoNewline; Write-Host "http://localhost:8088/webhook" -ForegroundColor Yellow
    Write-Host "  📊 Outbox Relay Health & Metrics:" -NoNewline; Write-Host "http://localhost:8082/metrics" -ForegroundColor Yellow
    Write-Host "  📊 Event Worker Health & Metrics:" -NoNewline; Write-Host "http://localhost:8083/metrics" -ForegroundColor Yellow
    Write-Host "------------------------------------------------------------------" -ForegroundColor DarkGray
    Write-Host "  Service logs stored in: $logDir" -ForegroundColor DarkGray

    # --------------------------------------------------------------------------
    # 5. Optional Initial Live Event Emission
    # --------------------------------------------------------------------------
    if (-not $NoEmitOnStart) {
        Write-Host "`n[5/5] Emitting initial live milestone event..." -ForegroundColor Yellow
        Push-Location "services/messaging"
        try {
            go run cmd/devemit/main.go -type parcel.booked.v1 -email kasun.perera@example.lk
            Write-Host "      -> Check Mailpit (http://localhost:8025) to view the rendered email!" -ForegroundColor Green
        } finally {
            Pop-Location
        }
    }

    # --------------------------------------------------------------------------
    # Interactive Command Loop
    # --------------------------------------------------------------------------
    Write-Host "`n==================================================================" -ForegroundColor Cyan
    Write-Host "  🎮 Interactive Controls:" -ForegroundColor Cyan
    Write-Host "     [e] Emit a live test event (parcel.delivered.v1)"
    Write-Host "     [r] Run background Reconciler audit pass"
    Write-Host "     [s] Query live Outbox & Delivery statistics"
    Write-Host "     [q] Stop all services and quit"
    Write-Host "==================================================================" -ForegroundColor Cyan

    while ($true) {
        $canReadKey = $false
        try {
            $canReadKey = [System.Console]::KeyAvailable
        } catch {
            # Non-interactive terminal (e.g. CI or redirected stdin)
            $canReadKey = $false
        }

        if ($canReadKey) {
            $key = [System.Console]::ReadKey($true)
            switch ($key.KeyChar) {
                'e' {
                    Write-Host "`n[ACTION] Emitting live parcel.delivered.v1 event..." -ForegroundColor Cyan
                    Push-Location "services/messaging"
                    try {
                        go run cmd/devemit/main.go -type parcel.delivered.v1 -email kasun.perera@example.lk
                    } finally {
                        Pop-Location
                    }
                    Write-Host "[SUCCESS] Event staged! Watch Relay publish it and Worker email it to Mailpit." -ForegroundColor Green
                }
                'r' {
                    Write-Host "`n[ACTION] Running Reconciler audit pass..." -ForegroundColor Cyan
                    Push-Location "services/messaging"
                    try {
                        go run cmd/reconcile/main.go
                    } finally {
                        Pop-Location
                    }
                }
                's' {
                    Write-Host "`n[ACTION] Fetching live database counts..." -ForegroundColor Cyan
                    $outboxCount = docker exec swiftdrop-dev-postgres psql -U swiftdrop_user -d swiftdrop_db -t -c "SELECT COUNT(*) FROM outbox_events WHERE status = 'PUBLISHED';"
                    $notifCount = docker exec swiftdrop-dev-postgres psql -U swiftdrop_user -d swiftdrop_db -t -c "SELECT COUNT(*) FROM notification_deliveries WHERE status = 'SUCCESS';"
                    $dedupCount = docker exec swiftdrop-dev-postgres psql -U swiftdrop_user -d swiftdrop_db -t -c "SELECT COUNT(*) FROM processed_events;"
                    Write-Host "  -> Published Outbox Events: $($outboxCount.Trim())" -ForegroundColor Green
                    Write-Host "  -> Delivered Emails (250 OK): $($notifCount.Trim())" -ForegroundColor Green
                    Write-Host "  -> Processed Dedup Entries:  $($dedupCount.Trim())" -ForegroundColor Green
                }
                'q' {
                    Write-Host "`nQuitting..." -ForegroundColor Yellow
                    break
                }
            }
        }
        Start-Sleep -Milliseconds 500
    }

} finally {
    Write-Host "`n[CLEANUP] Stopping background Go services..." -ForegroundColor Yellow
    foreach ($p in $processes) {
        try {
            if ($p.Process -and -not $p.Process.HasExited) {
                Write-Host "          Stopping $($p.Name)..." -ForegroundColor DarkGray
                $null = $p.Process.Kill()
                $null = $p.Process.WaitForExit(2000)
            }
            if ($p.OutStream) { $p.OutStream.Close() }
            if ($p.ErrStream) { $p.ErrStream.Close() }
        } catch {
            # Ignore cleanup errors
        }
    }
    Write-Host "[CLEANUP] All Go background services stopped cleanly." -ForegroundColor Green
    Write-Host "Note: Docker containers remain running. To stop them, run:" -ForegroundColor DarkGray
    Write-Host "  docker compose -f services/messaging/docker-compose.dev.yml down`n" -ForegroundColor DarkGray
}
