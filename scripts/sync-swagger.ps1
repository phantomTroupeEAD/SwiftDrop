# Synchronize embedded OpenAPI JSON specification to docs copy
$ErrorActionPreference = "Stop"

$SRC = "core-api/internal/events/transport/openapi.json"
$DST = "docs/drafts/member-08-events-messaging/swagger/events-ops.openapi.json"

if (!(Test-Path $SRC)) {
    Write-Error "Error: source OpenAPI spec $SRC not found"
    exit 1
}

$dstDir = Split-Path $DST -Parent
if (!(Test-Path $dstDir)) {
    New-Item -ItemType Directory -Path $dstDir -Force | Out-Null
}

Copy-Item -Path $SRC -Destination $DST -Force
Write-Host "Successfully synchronized $SRC -> $DST" -ForegroundColor Green

