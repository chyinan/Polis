#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/../../../../../" && pwd)"
cd "$repo"
evidence="$repo/evidence/development/r0.5b-real-provider-product-smoke/live-2/preflight-tests"
read -r POLIS_R05B5_TEST_DSN < "$evidence/runtime-test-dsn.txt"
export POLIS_R05B5_TEST_DSN

{
  bash scripts/go.sh test ./internal/provider -run TestLive2AuthorizationRequiresQualifiedSurfaceAndTaskSnapshot -count=1
  bash scripts/go.sh test ./internal/provider -run TestLive2RuntimeBindingMatchesPurposeAndQualification -count=1
  bash scripts/go.sh test ./internal/provider -run TestCodexRuntimeReadinessRequiresStructuredChatGPTCredentials -count=1
  bash scripts/go.sh test ./internal/provider -run TestCodexRuntimeReserveRejectsAuthRevisionDriftBeforeAllowance -count=1
  bash scripts/go.sh test ./internal/provider -run TestCodexRuntimeReadinessRejectsLIVE2BinaryAndHelperDrift -count=1
  bash scripts/go.sh test ./internal/provider -run TestValidateLive2RuntimePinsRequiresB4Identity -count=1
  bash scripts/go.sh test ./internal/provider -run TestCodexRuntimeReserveRejectsBinaryHashDriftBeforeAllowance -count=1
  bash scripts/go.sh test ./internal/provider -run TestCodexBusinessStartRequiresOneUseAuthorizationReservation -count=1
  bash scripts/go.sh test ./cmd/polis -run TestServeRealAdapterRejectsLive2PurposeAndQualificationDrift -count=1
  bash scripts/go.sh test ./internal/provider -run TestExecutionAuthorizationBindsProductIdentity -count=1
  bash scripts/go.sh test ./internal/provider -run TestExecutionAuthorizationDeniesMissionInternalTask -count=1
  bash scripts/go.sh test ./internal/provider -run TestCodexReservationDeniesInternalTaskBeforeCreatingAllowance -count=1
  bash scripts/go.sh test ./internal/kernel -run TestProductTaskValidationBindingMustMatchTaskMissionAndDigest -count=1
  bash scripts/go.sh test ./internal/kernel -run TestProductWorkspaceCASRequiresSha256DigestAndPositiveRevision -count=1
  bash scripts/go.sh test ./internal/kernel -run TestProviderWorkerSessionRejectsMissionBootstrapTask -count=1
  bash scripts/go.sh test ./internal/kernel -run TestProductProviderWorkerSessionCapturesValidationAndWorkspaceBinding -count=1
  bash scripts/go.sh test ./internal/kernel -run TestProductProviderAuthorizationBindingRejectsWorkspaceDrift -count=1
  bash scripts/go.sh test ./internal/kernel -run TestProductProviderWorkerSessionRequiresTaskValidationBinding -count=1
  bash scripts/go.sh test ./internal/kernel -run TestDatabasePreventsDuplicateProductTaskKindWithinMission -count=1
  bash scripts/go.sh test ./internal/kernel -run TestProviderWorkerSessionIsOneShotForExecutableTask -count=1
  bash scripts/go.sh test ./internal/kernel -run TestTaskValidationBindingCannotBeMutatedOrDeleted -count=1
  bash scripts/go.sh test ./internal/control -run TestRealProductStartHasOneProviderExecutableTaskAndOneBoundWorker -count=1
  bash scripts/go.sh test ./internal/control -run TestRealProductStartWithoutTaskValidationBindingStopsBeforeProviderAuthorization -count=1
  bash scripts/go.sh test ./internal/control -run TestRealProductStartRejectsMissionWithMultipleExecutableTasksBeforeReserve -count=1
  bash scripts/go.sh test ./internal/control -run TestRealProductStartRejectsWorkspaceDriftBeforeReserve -count=1
  bash scripts/go.sh test ./internal/control -run TestRealProductStartFailureCannotReserveAgainOnIdempotentReplay -count=1
  bash scripts/go.sh test ./internal/workbench -run TestWorkbenchDisplaysInternalAndProviderTaskKinds -count=1
  bash scripts/go.sh test ./internal/provider -run TestR05B4CurrentProductRuntimeSurfaceHasExactV2Identity -count=1
} 2>&1 | tee "$evidence/targeted-validation.log"
