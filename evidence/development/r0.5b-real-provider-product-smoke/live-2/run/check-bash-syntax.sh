#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/../../../../../" && pwd)"
files=(
  evidence/development/r0.5b-real-provider-product-smoke/live-2/run/setup-postgres.sh
  evidence/development/r0.5b-real-provider-product-smoke/live-2/run/stop-postgres.sh
  evidence/development/r0.5b-real-provider-product-smoke/live-2/run/bootstrap-company.sh
  evidence/development/r0.5b-real-provider-product-smoke/live-2/run/capture-draft-state.sh
  evidence/development/r0.5b-real-provider-product-smoke/live-2/run/capture-current-state.sh
  evidence/development/r0.5b-real-provider-product-smoke/live-2/preflight-tests/run-auth-tests.sh
  evidence/development/r0.5b-real-provider-product-smoke/live-2/preflight-tests/run-go-validation.sh
)
for relative in "${files[@]}"; do
  bash -n "$repo/$relative"
done
printf '{"result":"PASS","script_count":%s}\n' "${#files[@]}"
