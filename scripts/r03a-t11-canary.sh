#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
evidence="evidence/development/r0.3a-t11/luna-1"
test ! -e "$evidence/allowance.json" || { echo "R0.3A-T11 allowance already exists; refusing retry/reset"; exit 1; }
test ! -e "$evidence/preflight.json" || { echo "R0.3A-T11 preflight already exists; refusing retry/reset"; exit 1; }
for name in POLIS_NATIVE_PROXY HTTP_PROXY HTTPS_PROXY ALL_PROXY NO_PROXY; do
  test -z "${!name:-}" || { echo "T11 requires $name to be absent"; exit 1; }
done
test -r /mnt/c/Users/chyinan/.codex/auth.json || { echo "authorized Codex auth file is unavailable"; exit 1; }
export POLIS_CODEX_AUTH_FILE="/mnt/c/Users/chyinan/.codex/auth.json"
bash scripts/go.sh run ./cmd/polis-r03a-t11 --preflight
bash scripts/go.sh run ./cmd/polis-r03a-t11
