#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/../../.." && pwd)"
evidence="$repo/evidence/development/r1-r3-implementation-validation-20260925-slice-26"
export POLIS_TEST_DSN="$(cat "$evidence/runtime-test-dsn.txt")"
bash "$repo/scripts/go.sh" test ./internal/workbench -run '^TestQualifiedArtifactDeliveryManifestAndPackageReadFromPostgresAndCAS$' -count=1 > "$evidence/artifact-package-pg-test.log" 2>&1 || {
  cat "$evidence/artifact-package-pg-test.log"
  exit 1
}
cat "$evidence/artifact-package-pg-test.log"
