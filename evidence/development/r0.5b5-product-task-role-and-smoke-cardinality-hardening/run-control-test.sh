#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/../../../" && pwd)"
cd "$repo"
evidence="$repo/evidence/development/r0.5b5-product-task-role-and-smoke-cardinality-hardening"
read -r POLIS_R05B5_TEST_DSN < "$evidence/runtime-test-dsn.txt"
export POLIS_R05B5_TEST_DSN
bash scripts/go.sh test ./internal/control -run "$1" -count=1
