#!/usr/bin/env bash
# Stop the stack. ./scripts/down.sh --wipe also deletes the database volume.
set -euo pipefail
cd "$(dirname "$0")/.."
if [ "${1:-}" = "--wipe" ]; then docker compose down -v; else docker compose down; fi
