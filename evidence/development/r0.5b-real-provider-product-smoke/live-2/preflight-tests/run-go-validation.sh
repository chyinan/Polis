#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/../../../../../" && pwd)"
cd "$repo"
evidence="$repo/evidence/development/r0.5b-real-provider-product-smoke/live-2/preflight-tests"
read -r POLIS_R05B5_TEST_DSN < "$evidence/runtime-test-dsn.txt"
export POLIS_R05B5_TEST_DSN

mode="${1:?usage: run-go-validation.sh test|race|vet|linux-build|windows-build}"
case "$mode" in
  test) command=(env -u POLIS_R05B5_TEST_DSN bash scripts/go.sh test ./...) ;;
  race) command=(env -u POLIS_R05B5_TEST_DSN bash scripts/go.sh test -race ./...) ;;
  vet) command=(bash scripts/go.sh vet ./...) ;;
  linux-build) command=(bash scripts/go.sh build ./cmd/...) ;;
  windows-build) command=(env GOOS=windows GOARCH=amd64 bash scripts/go.sh build ./cmd/...) ;;
  *) echo "unknown validation mode: $mode" >&2; exit 2 ;;
esac

"${command[@]}" 2>&1 | tee "$evidence/go-$mode.log"
