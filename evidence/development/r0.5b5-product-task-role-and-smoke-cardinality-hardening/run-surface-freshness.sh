#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/../../../" && pwd)"
cd "$repo"
evidence="$repo/evidence/development/r0.5b5-product-task-role-and-smoke-cardinality-hardening"

bash scripts/go.sh test ./internal/provider -run TestR05B4CurrentProductRuntimeSurfaceHasExactV2Identity -count=1 -v 2>&1 | tee "$evidence/surface-freshness-test.log"
