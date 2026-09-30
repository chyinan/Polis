#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
export LD_LIBRARY_PATH="$repo/.tools/pg/usr/lib/x86_64-linux-gnu"
pg="$repo/.tools/pg/usr/lib/postgresql/18/bin"
dsn="$(cat "$repo/evidence/development/r0.5b-real-provider-product-smoke-live-4/runtime-test-dsn.txt")"
evidence="$repo/evidence/development/r0.5b-real-provider-product-smoke-live-4"
exec > "$evidence/post-provider-state-query.txt"

echo '--- worker_sessions ---'
"$pg/psql" "$dsn" -X -P pager=off -c 'SELECT * FROM worker_sessions;'
echo '--- events ---'
"$pg/psql" "$dsn" -X -P pager=off -c "SELECT * FROM events WHERE company_id='r05b-live-4' ORDER BY company_seq;"
echo '--- provider terminal observations ---'
"$pg/psql" "$dsn" -X -P pager=off -c "SELECT session_id,reason,data FROM worker_observations WHERE company_id='r05b-live-4' ORDER BY id;"
echo '--- authoritative product rows ---'
for table in missions tasks task_validation_bindings worker_workspaces worker_checks worker_checkpoints artifacts artifact_staging task_validation_artifact_qualifications receipts; do
  echo "### $table"
  "$pg/psql" "$dsn" -X -P pager=off -c "SELECT * FROM $table WHERE company_id='r05b-live-4';" || true
done
