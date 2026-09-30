#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
evidence="evidence/development/r0.3a-t9/luna-1"
test ! -e "$evidence/allowance.json" || { echo "R0.3A-T9 allowance already exists; refusing retry/reset"; exit 1; }
for name in POLIS_NATIVE_PROXY HTTP_PROXY HTTPS_PROXY ALL_PROXY NO_PROXY; do
  test -z "${!name:-}" || { echo "T9 requires $name to be absent"; exit 1; }
done
test -r /mnt/c/Users/chyinan/.codex/auth.json || { echo "authorized Codex auth file is unavailable"; exit 1; }
export POLIS_CODEX_AUTH_FILE="/mnt/c/Users/chyinan/.codex/auth.json"
mkdir -p "$evidence"
if test ! -e "$evidence/preflight.json"; then
  bash scripts/go.sh run ./cmd/polis-r03a-t9 --preflight
fi
bash scripts/go.sh run ./cmd/polis-r03a-t9
