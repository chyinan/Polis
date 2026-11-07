#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
export LD_LIBRARY_PATH="$repo/.tools/pg/usr/lib/x86_64-linux-gnu"
pg="$repo/.tools/pg/usr/lib/postgresql/18/bin"
dsn="$(cat "$repo/evidence/development/r0.5b-real-provider-product-smoke-live-5/runtime-test-dsn.txt")"
evidence="$repo/evidence/development/r0.5b-real-provider-product-smoke-live-5"

"$pg/psql" "$dsn" -X -tA -v ON_ERROR_STOP=1 -c "SELECT json_build_object(
  'company', (SELECT row_to_json(c) FROM companies c WHERE c.id='r05b-live-5'),
  'missions', COALESCE((SELECT json_agg(row_to_json(m)) FROM missions m WHERE m.company_id='r05b-live-5'),'[]'::json),
  'tasks', COALESCE((SELECT json_agg(row_to_json(t)) FROM tasks t WHERE t.company_id='r05b-live-5'),'[]'::json),
  'task_validation_bindings', COALESCE((SELECT json_agg(row_to_json(b)) FROM task_validation_bindings b WHERE b.company_id='r05b-live-5'),'[]'::json),
  'worker_sessions', COALESCE((SELECT json_agg(row_to_json(s)) FROM worker_sessions s WHERE s.company_id='r05b-live-5'),'[]'::json),
  'worker_workspaces', COALESCE((SELECT json_agg(row_to_json(w)) FROM worker_workspaces w WHERE w.company_id='r05b-live-5'),'[]'::json),
  'worker_checks', COALESCE((SELECT json_agg(row_to_json(c)) FROM worker_checks c WHERE c.company_id='r05b-live-5'),'[]'::json),
  'worker_checkpoints', COALESCE((SELECT json_agg(row_to_json(c)) FROM worker_checkpoints c WHERE c.company_id='r05b-live-5'),'[]'::json),
  'artifacts', COALESCE((SELECT json_agg(row_to_json(a)) FROM artifacts a WHERE a.company_id='r05b-live-5'),'[]'::json),
  'artifact_staging', COALESCE((SELECT json_agg(row_to_json(a)) FROM artifact_staging a WHERE a.company_id='r05b-live-5'),'[]'::json),
  'task_validation_artifact_qualifications', COALESCE((SELECT json_agg(row_to_json(q)) FROM task_validation_artifact_qualifications q WHERE q.company_id='r05b-live-5'),'[]'::json),
  'worker_observations', COALESCE((SELECT json_agg(row_to_json(o)) FROM worker_observations o WHERE o.company_id='r05b-live-5'),'[]'::json),
  'events', COALESCE((SELECT json_agg(row_to_json(e)) FROM events e WHERE e.company_id='r05b-live-5'),'[]'::json)
)" > "$evidence/postgres-final-state.json"
