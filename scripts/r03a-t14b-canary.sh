#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
evidence="evidence/development/r0.3a-t14b/luna-1"
test ! -e "$evidence/allowance.json" || { echo "R0.3A-T14B allowance already exists; refusing retry/reset"; exit 1; }
test ! -e "$evidence/preflight.json" || { echo "R0.3A-T14B preflight already exists; refusing retry/reset"; exit 1; }
for name in POLIS_NATIVE_PROXY HTTP_PROXY HTTPS_PROXY ALL_PROXY NO_PROXY; do
  test -z "${!name:-}" || { echo "T14B requires $name to be absent"; exit 1; }
done
test -r /mnt/c/Users/chyinan/.codex/auth.json || { echo "authorized Codex auth file is unavailable"; exit 1; }
export POLIS_CODEX_AUTH_FILE="/mnt/c/Users/chyinan/.codex/auth.json"
bash scripts/go.sh test ./... -count=1
bash scripts/go.sh vet ./...
bash scripts/go.sh build ./cmd/...
bash scripts/go.sh run ./cmd/polis-r03a-t14b --preflight
bash scripts/go.sh run ./cmd/polis-r03a-t14b
