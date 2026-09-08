#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
database="$(cat evidence/development/r0.1/probe-database.txt)"
[[ "$database" =~ ^polis_r0_probe_[0-9]+_[0-9]+$ ]] || exit 1
export LD_LIBRARY_PATH="$PWD/.tools/pg/usr/lib/x86_64-linux-gnu"
dsn="host=$PWD/.runtime/linux/pg-socket port=55432 dbname=$database user=polis_runtime"
.tools/pg/usr/lib/postgresql/18/bin/psql "$dsn" -X -tA -v ON_ERROR_STOP=1 -c "SELECT json_build_object(
 'sessions',(SELECT json_agg(row_to_json(s)) FROM worker_sessions s),
 'tasks',(SELECT json_agg(row_to_json(t)) FROM tasks t),
 'obligations',(SELECT json_agg(row_to_json(o)) FROM obligations o),
 'workspace',(SELECT json_agg(row_to_json(w)) FROM worker_workspaces w),
 'checkpoint_count',(SELECT count(*) FROM worker_checkpoints),
 'artifact_count',(SELECT count(*) FROM artifacts),
 'accepted_tool_receipts',(SELECT count(*) FROM receipts WHERE key LIKE 'tool-%')
);" > evidence/development/r0.1/probe-db-snapshot.json
