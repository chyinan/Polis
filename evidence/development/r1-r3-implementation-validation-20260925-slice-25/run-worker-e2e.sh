#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/../../.." && pwd)"
evidence="$repo/evidence/development/r1-r3-implementation-validation-20260925-slice-25"
dsn="$(cat "$evidence/runtime-test-dsn.txt")"
export POLIS_TEST_DSN="$dsn"
bash "$repo/scripts/go.sh" test ./internal/control -run '^TestProductWorkerReceivesFrozenGitSnapshotFromSelectedCommit$' -count=1 > "$evidence/git-worker-pg-test.log" 2>&1 || {
  cat "$evidence/git-worker-pg-test.log"
  exit 1
}
cat "$evidence/git-worker-pg-test.log"
