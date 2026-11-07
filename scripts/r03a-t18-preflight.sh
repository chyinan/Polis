#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
evidence="evidence/development/r0.3a-t18"
test ! -e "$evidence/qualification.json" || { echo "R0.3A-T18 evidence already exists; refusing rerun"; exit 1; }
for name in POLIS_NATIVE_PROXY HTTP_PROXY HTTPS_PROXY ALL_PROXY NO_PROXY; do
  unset "$name"
done
bash scripts/go.sh test ./... -count=1
bash scripts/go.sh test -race ./internal/probe ./internal/runner ./internal/codex -count=1
bash scripts/go.sh vet ./...
bash scripts/go.sh build ./cmd/...
bash scripts/go.sh run ./cmd/polis-r03a-t18
