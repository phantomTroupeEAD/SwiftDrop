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
if ((Test-Path "core-api") -or (Test-Path "go.mod")) {
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
            Write-Host "  [PASS] No Go source files created yet." -ForegroundColor Gray
        }
    } else {
        Write-Host "  [NOTICE] gofmt not found in PATH." -ForegroundColor Gray
    }
} else {
    Write-Host "[2/4] Skipping Go formatting (core-api not initialized yet)." -ForegroundColor Gray
}

# 3. Linter Execution
if (Get-Command golangci-lint -ErrorAction SilentlyContinue) {
    Write-Host "[3/4] Running golangci-lint..." -ForegroundColor Yellow
    if (Test-Path "core-api") {
        Push-Location "core-api"
        golangci-lint run ./...
        if ($LASTEXITCODE -ne 0) { $Failed = $true }
        Pop-Location
    }
} else {
    Write-Host "[3/4] Notice: golangci-lint not installed locally (runs in CI)." -ForegroundColor Gray
}

# 4. Tests Execution
if ((Get-Command go -ErrorAction SilentlyContinue) -and ((Test-Path "core-api/go.mod") -or (Test-Path "go.mod"))) {
    Write-Host "[4/4] Running Go tests..." -ForegroundColor Yellow
    if (Test-Path "core-api/go.mod") {
        Push-Location "core-api"
        go test -v ./...
        if ($LASTEXITCODE -ne 0) { $Failed = $true }
        Pop-Location
    } elseif (Test-Path "go.mod") {
        go test -v ./...
        if ($LASTEXITCODE -ne 0) { $Failed = $true }
    }
} else {
    Write-Host "[4/4] Skipping Go test execution (go.mod not yet created)." -ForegroundColor Gray
}

Write-Host "=================================================" -ForegroundColor Cyan
if ($Failed) {
    Write-Host "  [FAILED] Verification checks failed. Address errors before PR." -ForegroundColor Red
    exit 1
} else {
    Write-Host "  [SUCCESS] All checks passed. Ready for review." -ForegroundColor Green
    exit 0
}
