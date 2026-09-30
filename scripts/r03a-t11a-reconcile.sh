#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
test -r /mnt/c/Users/chyinan/.codex/auth.json || { echo "authorized Codex auth file is unavailable"; exit 1; }
export POLIS_CODEX_AUTH_FILE="/mnt/c/Users/chyinan/.codex/auth.json"
bash scripts/go.sh run ./cmd/polis-r03a-t11a
