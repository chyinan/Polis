#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
evidence="evidence/development/r0.3a-t6/luna-1"
test ! -e "$evidence/allowance.json" || { echo "R0.3A-T6 allowance already exists; refusing retry/reset"; exit 1; }
test -r /mnt/c/Users/chyinan/.codex/auth.json || { echo "authorized Codex auth file is unavailable"; exit 1; }
export POLIS_CODEX_AUTH_FILE="/mnt/c/Users/chyinan/.codex/auth.json"
export POLIS_NATIVE_PROXY="http://127.0.0.1:$(cat .runtime/r01-proxy-port.txt)"
mkdir -p "$evidence"
if test ! -e "$evidence/preflight.json"; then
  bash scripts/go.sh run ./cmd/polis-r03a-t6 --preflight
fi
bash scripts/go.sh run ./cmd/polis-r03a-t6
