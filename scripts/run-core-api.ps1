# Run Core API locally with settings from the repo-root .env (gitignored).
# Core API does not read .env itself, so this script copies the keys below into the
# process environment first. It loads only these keys on purpose: .env also holds
# PORT=8000 (AI service), which would move Core API off :8080.
# Usage (from anywhere):  .\scripts\run-core-api.ps1
param([string]$EnvFile = (Join-Path $PSScriptRoot "..\.env"))

$allowed = @("DATABASE_URL", "APP_ENV")

if (-not (Test-Path $EnvFile)) {
    Write-Host "[ERROR] $EnvFile not found. Copy .env.example to .env first." -ForegroundColor Red
    exit 1
}

foreach ($line in Get-Content $EnvFile) {
    if ($line -match '^\s*(?<k>[A-Z_][A-Z0-9_]*)\s*=\s*(?<v>.*?)\s*$' -and $allowed -contains $Matches.k) {
        Set-Item -Path "Env:$($Matches.k)" -Value $Matches.v.Trim('"').Trim("'")
    }
}

if (-not $env:DATABASE_URL) {
    Write-Host "[ERROR] DATABASE_URL is not set in $EnvFile. Without it Core API runs in mock mode and the admin routes return 404." -ForegroundColor Red
    exit 1
}

# Show where we connect, never the password.
Write-Host ("Using DATABASE_URL = " + ($env:DATABASE_URL -replace '(://[^:]+:)[^@]*@', '$1***@')) -ForegroundColor Cyan

Push-Location (Join-Path $PSScriptRoot "..\core-api")
try { go run ./cmd/api } finally { Pop-Location }
