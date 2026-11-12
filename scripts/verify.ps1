# SwiftDrop — Local Verification Script (PowerShell for Windows)
$ErrorActionPreference = "Continue"

Write-Host "=================================================" -ForegroundColor Cyan
Write-Host "  SwiftDrop: Running Automated Quality Verification" -ForegroundColor Cyan
Write-Host "=================================================" -ForegroundColor Cyan

$Failed = $false

# 1. Secret Scanning Check
Write-Host "[1/4] Checking for accidental hardcoded secrets..." -ForegroundColor Yellow
$secretPattern = "(BEGIN RSA PRIVATE KEY|BEGIN OPENSSH PRIVATE KEY|AKIA[0-9A-Z]{16}|ghp_[a-zA-Z0-9]{36})"
$foundSecrets = Get-ChildItem -Recurse -File -Exclude "*.docx","*.pdf","*.png","*.excalidraw","verify.ps1","verify.sh" | 
    Where-Object { $_.FullName -notmatch "node_modules|\.git|vendor|scripts" } | 
    Select-String -Pattern $secretPattern

if ($foundSecrets) {
    Write-Host "  [ERROR] Potential hardcoded secret detected!" -ForegroundColor Red
    $foundSecrets | ForEach-Object { Write-Host "    $($_.Path):$($_.LineNumber)" -ForegroundColor Red }
    $Failed = $true
} else {
    Write-Host "  [PASS] No hardcoded secrets detected." -ForegroundColor Green
}

# 2. Go Code Formatting Check
if ((Test-Path "core-api") -or (Test-Path "services/messaging") -or (Test-Path "go.mod")) {
    Write-Host "[2/4] Verifying Go formatting (gofmt)..." -ForegroundColor Yellow
    if (Get-Command gofmt -ErrorAction SilentlyContinue) {
        $goFiles = Get-ChildItem -Recurse -Filter *.go | Where-Object { $_.FullName -notmatch "vendor|docs" } | Select-Object -ExpandProperty FullName
        if ($goFiles) {
            $unformatted = gofmt -l $goFiles
            if ($unformatted) {
                Write-Host "  [ERROR] Unformatted Go files:" -ForegroundColor Red
                $unformatted | ForEach-Object { Write-Host "    $_" -ForegroundColor Red }
                $Failed = $true
            } else {
                Write-Host "  [PASS] All Go files formatted." -ForegroundColor Green
            }
        } else {
            Write-Host "  [PASS] All Go files formatted." -ForegroundColor Green
        }
    } else {
        Write-Host "  [NOTICE] gofmt not found in PATH." -ForegroundColor Gray
    }
} else {
    Write-Host "[2/4] Skipping Go formatting (no Go projects initialized yet)." -ForegroundColor Gray
}

# 3. Linter Execution
if (Get-Command golangci-lint -ErrorAction SilentlyContinue) {
    Write-Host "[3/4] Running golangci-lint..." -ForegroundColor Yellow
    if (Test-Path "core-api") {
        Write-Host "  Linting core-api..." -ForegroundColor Yellow
        Push-Location "core-api"
        golangci-lint run ./...
        if ($LASTEXITCODE -ne 0) { $Failed = $true }
        Pop-Location
    }
    if (Test-Path "services/messaging") {
        Write-Host "  Linting services/messaging..." -ForegroundColor Yellow
        Push-Location "services/messaging"
        golangci-lint run ./...
        if ($LASTEXITCODE -ne 0) { $Failed = $true }
        Pop-Location
    }
} else {
    Write-Host "[3/4] Notice: golangci-lint not installed locally (runs in CI)." -ForegroundColor Gray
}

# 4. Tests Execution
if (Get-Command go -ErrorAction SilentlyContinue) {
    Write-Host "[4/4] Running Go tests..." -ForegroundColor Yellow
    if (Test-Path "core-api/go.mod") {
        Write-Host "  Running core-api tests..." -ForegroundColor Yellow
        Push-Location "core-api"
        go test -v ./...
        if ($LASTEXITCODE -ne 0) { $Failed = $true }
        Pop-Location
    }
    if (Test-Path "services/messaging/go.mod") {
        Write-Host "  Running services/messaging tests..." -ForegroundColor Yellow
        Push-Location "services/messaging"
        go test -v ./...
        if ($LASTEXITCODE -ne 0) { $Failed = $true }
        Pop-Location
    }
    if (!(Test-Path "core-api/go.mod") -and !(Test-Path "services/messaging/go.mod") -and (Test-Path "go.mod")) {
        go test -v ./...
        if ($LASTEXITCODE -ne 0) { $Failed = $true }
    }

    # 5. Swagger Specification Verification
    if (Test-Path "core-api/docs/swagger.json") {
        Write-Host "  Verifying core-api Swagger specification..." -ForegroundColor Yellow
        try {
            $swaggerContent = Get-Content "core-api/docs/swagger.json" -Raw | ConvertFrom-Json
            if ($swaggerContent.swagger -or $swaggerContent.openapi) {
                Write-Host "  [PASS] Unified Swagger specification is valid JSON." -ForegroundColor Green
            } else {
                Write-Host "  [ERROR] core-api/docs/swagger.json is missing 'swagger' or 'openapi' root field." -ForegroundColor Red
                $Failed = $true
            }
        } catch {
            Write-Host "  [ERROR] Failed to parse core-api/docs/swagger.json as JSON: $_" -ForegroundColor Red
            $Failed = $true
        }
    }
} else {
    Write-Host "[4/4] Skipping Go test execution (go not found in PATH)." -ForegroundColor Gray
}

# 5. Web Frontend Quality Gate (lint:ui + build)
if (Test-Path "web") {
    Write-Host "[5/5] Running web quality verification (npm run build)..." -ForegroundColor Yellow
    Push-Location "web"
    npm run build
    if ($LASTEXITCODE -ne 0) { $Failed = $true }
    Pop-Location
}

Write-Host "=================================================" -ForegroundColor Cyan
if ($Failed) {
    Write-Host "  [FAILED] Verification checks failed. Address errors before PR." -ForegroundColor Red
    exit 1
} else {
    Write-Host "  [SUCCESS] All checks passed. Ready for review." -ForegroundColor Green
}
