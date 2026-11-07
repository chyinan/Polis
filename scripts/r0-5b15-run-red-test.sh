#!/usr/bin/env bash
set -euo pipefail
repo="$(cd "$(dirname "$0")/.." && pwd)"
export POLIS_R05B15_TEST_DSN="$(cat "$repo/evidence/development/r0.5b15-business-path-provider-initialize-hardening/test-db-2/runtime-test-dsn.txt")"
exec bash "$repo/scripts/go.sh" test -v ./internal/control -run TestRealProviderWorkerAdapterInitializationFailureUsesInitializationEvent
