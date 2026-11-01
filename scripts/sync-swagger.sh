#!/usr/bin/env bash
# Synchronize embedded OpenAPI JSON specification to docs copy
set -euo pipefail

SRC="core-api/internal/events/transport/openapi.json"
DST="docs/drafts/member-08-events-messaging/swagger/events-ops.openapi.json"

if [ ! -f "$SRC" ]; then
    echo "Error: source OpenAPI spec $SRC not found"
    exit 1
fi

mkdir -p "$(dirname "$DST")"
cp "$SRC" "$DST"
echo "Successfully synchronized $SRC -> $DST"

