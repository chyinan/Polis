#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/../../../" && pwd)"
cd "$repo"
evidence="$repo/evidence/development/r0.5b5-product-task-role-and-smoke-cardinality-hardening"
read -r POLIS_R05B5_TEST_DSN < "$evidence/runtime-test-dsn.txt"
export POLIS_R05B5_TEST_DSN

bash scripts/go.sh test ./internal/core -run TestProductProviderExecutableTaskRequiresCompatKindAndBackendAssignee -count=1
bash scripts/go.sh test ./internal/kernel -run TestTaskIsProductProviderExecutableUsesPersistedKindAndAssignee -count=1
bash scripts/go.sh test ./internal/kernel -run TestSelectSingleProductProviderTask -count=1
bash scripts/go.sh test ./internal/kernel -run TestProviderWorkerSessionRejectsMissionBootstrapTask -count=1
bash scripts/go.sh test ./internal/kernel -run TestDatabasePreventsDuplicateProductTaskKindWithinMission -count=1
bash scripts/go.sh test ./internal/kernel -run TestProviderWorkerSessionIsOneShotForExecutableTask -count=1
bash scripts/go.sh test ./internal/kernel -run TestTaskValidationBindingCannotBeMutatedOrDeleted -count=1
