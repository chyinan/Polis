#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/../../../" && pwd)"
cd "$repo"
evidence="$repo/evidence/development/r0.5b5-product-task-role-and-smoke-cardinality-hardening"
read -r POLIS_R05B5_TEST_DSN < "$evidence/runtime-test-dsn.txt"
export POLIS_R05B5_TEST_DSN

{
  bash "$evidence/run-kernel-task-role-tests.sh"
  bash scripts/go.sh test ./internal/provider -run TestExecutionAuthorizationDeniesMissionInternalTask -count=1
  bash scripts/go.sh test ./internal/provider -run TestCodexReservationDeniesInternalTaskBeforeCreatingAllowance -count=1
  bash scripts/go.sh test ./internal/control -run TestRealProductStartHasOneProviderExecutableTaskAndOneBoundWorker -count=1 -v
  bash scripts/go.sh test ./internal/control -run TestRealProductStartRejectsMissionWithMultipleExecutableTasksBeforeReserve -count=1
  bash scripts/go.sh test ./internal/control -run TestRealProductStartFailureCannotReserveAgainOnIdempotentReplay -count=1
  bash scripts/go.sh test ./internal/workbench -run TestWorkbenchDisplaysInternalAndProviderTaskKinds -count=1
  bash scripts/go.sh test ./internal/provider -run TestR05B4CurrentProductRuntimeSurfaceHasExactV2Identity -count=1
} 2>&1 | tee "$evidence/targeted-validation.log"
