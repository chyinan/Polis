#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
export LD_LIBRARY_PATH="$repo/.tools/pg/usr/lib/x86_64-linux-gnu"
pg="$repo/.tools/pg/usr/lib/postgresql/18/bin"
dsn="$(cat "$repo/evidence/development/r0.5b-real-provider-product-smoke-live-5/runtime-test-dsn.txt")"
evidence="$repo/evidence/development/r0.5b-real-provider-product-smoke-live-5"
exec > "$evidence/live5-state-query.txt"

echo '--- missions ---'
"$pg/psql" "$dsn" -X -P pager=off -c "SELECT id,state,title FROM missions WHERE company_id='r05b-live-5';"
echo '--- tasks ---'
"$pg/psql" "$dsn" -X -P pager=off -c "SELECT id,kind,owner,state,mission_id FROM tasks WHERE company_id='r05b-live-5' ORDER BY id;"
echo '--- task validation bindings ---'
"$pg/psql" "$dsn" -X -P pager=off -c "SELECT task_id,mission_id,configuration_digest,acceptance_revision,runner_kind,runner_revision FROM task_validation_bindings WHERE company_id='r05b-live-5';"
echo '--- worker sessions ---'
"$pg/psql" "$dsn" -X -P pager=off -c "SELECT id,task_id,employee_id,state,profile,epoch,process_pid,tool_calls_used FROM worker_sessions WHERE company_id='r05b-live-5';"
echo '--- worker workspaces ---'
"$pg/psql" "$dsn" -X -P pager=off -c "SELECT task_id,revision,digest FROM worker_workspaces WHERE company_id='r05b-live-5';"
echo '--- worker checks ---'
"$pg/psql" "$dsn" -X -P pager=off -c "SELECT * FROM worker_checks WHERE company_id='r05b-live-5';"
echo '--- worker checkpoints ---'
"$pg/psql" "$dsn" -X -P pager=off -c "SELECT * FROM worker_checkpoints WHERE company_id='r05b-live-5';"
echo '--- artifacts ---'
"$pg/psql" "$dsn" -X -P pager=off -c "SELECT * FROM artifacts WHERE company_id='r05b-live-5';"
echo '--- artifact staging ---'
"$pg/psql" "$dsn" -X -P pager=off -c "SELECT * FROM artifact_staging WHERE company_id='r05b-live-5';"
echo '--- artifact qualifications ---'
"$pg/psql" "$dsn" -X -P pager=off -c "SELECT * FROM task_validation_artifact_qualifications WHERE company_id='r05b-live-5';"
echo '--- events ---'
"$pg/psql" "$dsn" -X -P pager=off -c "SELECT kind,company_seq,payload FROM events WHERE company_id='r05b-live-5' ORDER BY company_seq;"
echo '--- worker observations ---'
"$pg/psql" "$dsn" -X -P pager=off -c "SELECT session_id,reason,data FROM worker_observations WHERE company_id='r05b-live-5' ORDER BY id;"
