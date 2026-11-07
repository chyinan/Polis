#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
evidence="evidence/development/r0.3a-t16"
test ! -e "$evidence/qualification.json" || { echo "R0.3A-T16 evidence already exists; refusing rerun"; exit 1; }
for name in POLIS_NATIVE_PROXY HTTP_PROXY HTTPS_PROXY ALL_PROXY NO_PROXY; do
  unset "$name"
done
test -r /mnt/c/Users/chyinan/.codex/auth.json || { echo "authorized Codex auth file is unavailable"; exit 1; }
export POLIS_CODEX_AUTH_FILE="/mnt/c/Users/chyinan/.codex/auth.json"
bash scripts/go.sh test ./... -count=1
bash scripts/go.sh test -race ./internal/probe ./internal/runner ./internal/codex -count=1
bash scripts/go.sh vet ./...
bash scripts/go.sh build ./cmd/...
bash scripts/go.sh run ./cmd/polis-r03a-t16
