#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
umask 077
V9_ROOT="/home/chyinan/.local/state/polis-recovery/r03a-sample-6-restore-continuation-v9"
PGDATA="$V9_ROOT/postgres/pgdata"
SOCKET="/tmp/polis-pg-r03a-v9-session-forensics"
PG="/mnt/d/Programs/Polis/.tools/pg/usr/lib/postgresql/18/bin"
export LD_LIBRARY_PATH="/mnt/d/Programs/Polis/.tools/pg/usr/lib/x86_64-linux-gnu"
EVIDENCE="/mnt/d/Programs/Polis/evidence/development/r0.3a-v9-frontend-session-provenance-forensics"
mkdir -m 700 -p "$EVIDENCE"
if [[ -n "$(find "$EVIDENCE" -mindepth 1 -maxdepth 1 -print -quit 2>/dev/null)" ]]; then
  echo "forensic evidence path is not fresh" >&2
  exit 1
fi

started=0
cleanup() {
  if [[ "$started" == 1 ]]; then
    "$PG/pg_ctl" -D "$PGDATA" -m fast -w stop >/dev/null 2>&1 || true
  fi
  rmdir -- "$SOCKET" 2>/dev/null || true
}
trap cleanup EXIT

mkdir -m 700 -p "$SOCKET"
"$PG/pg_ctl" -D "$PGDATA" -l "$V9_ROOT/postgres/session-forensics.log" -o "-k $SOCKET -h 127.0.0.1 -p 55432 -c synchronous_commit=on" -w start >/dev/null
started=1

restore_db=$(PGOPTIONS='-c default_transaction_read_only=on' "$PG/psql" -X -h 127.0.0.1 -p 55432 -U chyinan -d postgres -Atc "select datname from pg_database where datname like 'polis_r0_3a_sample6_v6_restore_%' order by datname desc limit 1")
[[ -n "$restore_db" ]] || { echo "v9 restored database missing" >&2; exit 1; }
printf '%s\n' "$restore_db" > "$EVIDENCE/database.txt"

PGOPTIONS='-c default_transaction_read_only=on' "$PG/psql" -X -h 127.0.0.1 -p 55432 -U chyinan -d "$restore_db" -v ON_ERROR_STOP=1 -P pager=off -c '\d+ worker_sessions' > "$EVIDENCE/worker-sessions-schema.txt"
PGOPTIONS='-c default_transaction_read_only=on' "$PG/psql" -X -h 127.0.0.1 -p 55432 -U chyinan -d "$restore_db" -v ON_ERROR_STOP=1 -At -F $'\t' -c "SELECT row_to_json(ws), row_to_json(t), row_to_json(e) FROM worker_sessions ws JOIN tasks t ON t.company_id=ws.company_id AND t.id=ws.task_id JOIN employees e ON e.company_id=ws.company_id AND e.id=ws.employee_id WHERE ws.employee_id='emp-frontend' ORDER BY ws.company_id, ws.id" > "$EVIDENCE/frontend-session-task-employee.tsv"

company_id=$(PGOPTIONS='-c default_transaction_read_only=on' "$PG/psql" -X -h 127.0.0.1 -p 55432 -U chyinan -d "$restore_db" -Atc "SELECT company_id FROM worker_sessions WHERE employee_id='emp-frontend' ORDER BY company_id,id LIMIT 1")
session_id=$(PGOPTIONS='-c default_transaction_read_only=on' "$PG/psql" -X -h 127.0.0.1 -p 55432 -U chyinan -d "$restore_db" -Atc "SELECT id FROM worker_sessions WHERE employee_id='emp-frontend' ORDER BY company_id,id LIMIT 1")
if [[ -n "$company_id" ]]; then
  PGOPTIONS='-c default_transaction_read_only=on' "$PG/psql" -X -h 127.0.0.1 -p 55432 -U chyinan -d "$restore_db" -v ON_ERROR_STOP=1 -At -F $'\t' -c "SELECT company_seq,kind,payload,observed FROM events WHERE company_id='$company_id' ORDER BY company_seq" > "$EVIDENCE/events.tsv"
  PGOPTIONS='-c default_transaction_read_only=on' "$PG/psql" -X -h 127.0.0.1 -p 55432 -U chyinan -d "$restore_db" -v ON_ERROR_STOP=1 -At -F $'\t' -c "SELECT actor,key,fingerprint,result FROM receipts WHERE company_id='$company_id' ORDER BY actor,key" > "$EVIDENCE/receipts.tsv"
  PGOPTIONS='-c default_transaction_read_only=on' "$PG/psql" -X -h 127.0.0.1 -p 55432 -U chyinan -d "$restore_db" -v ON_ERROR_STOP=1 -At -F $'\t' -c "SELECT row_to_json(x) FROM (SELECT * FROM worker_checks WHERE company_id='$company_id' ORDER BY id) x" > "$EVIDENCE/worker-checks.tsv"
  PGOPTIONS='-c default_transaction_read_only=on' "$PG/psql" -X -h 127.0.0.1 -p 55432 -U chyinan -d "$restore_db" -v ON_ERROR_STOP=1 -At -F $'\t' -c "SELECT row_to_json(x) FROM (SELECT * FROM worker_checkpoints WHERE company_id='$company_id' ORDER BY id) x" > "$EVIDENCE/worker-checkpoints.tsv"
  PGOPTIONS='-c default_transaction_read_only=on' "$PG/psql" -X -h 127.0.0.1 -p 55432 -U chyinan -d "$restore_db" -v ON_ERROR_STOP=1 -At -F $'\t' -c "SELECT row_to_json(x) FROM (SELECT * FROM worker_observations WHERE company_id='$company_id' ORDER BY id) x" > "$EVIDENCE/worker-observations.tsv"
  PGOPTIONS='-c default_transaction_read_only=on' "$PG/psql" -X -h 127.0.0.1 -p 55432 -U chyinan -d "$restore_db" -v ON_ERROR_STOP=1 -At -F $'\t' -c "SELECT 'companies',row_to_json(x) FROM (SELECT * FROM companies WHERE id='$company_id') x UNION ALL SELECT 'employees',row_to_json(x) FROM (SELECT * FROM employees WHERE company_id='$company_id' ORDER BY id) x UNION ALL SELECT 'tasks',row_to_json(x) FROM (SELECT * FROM tasks WHERE company_id='$company_id' ORDER BY id) x UNION ALL SELECT 'messages',row_to_json(x) FROM (SELECT * FROM messages WHERE company_id='$company_id' ORDER BY id) x UNION ALL SELECT 'obligations',row_to_json(x) FROM (SELECT * FROM obligations WHERE company_id='$company_id' ORDER BY id) x UNION ALL SELECT 'contract_revisions',row_to_json(x) FROM (SELECT * FROM contract_revisions WHERE company_id='$company_id' ORDER BY revision) x UNION ALL SELECT 'artifacts',row_to_json(x) FROM (SELECT * FROM artifacts WHERE company_id='$company_id' ORDER BY id) x UNION ALL SELECT 'workspaces',row_to_json(x) FROM (SELECT * FROM worker_workspaces WHERE company_id='$company_id' ORDER BY task_id) x" > "$EVIDENCE/business-state.tsv"
  PGOPTIONS='-c default_transaction_read_only=on' "$PG/psql" -X -h 127.0.0.1 -p 55432 -U chyinan -d "$restore_db" -v ON_ERROR_STOP=1 -At -F $'\t' -c "SELECT 'frontend_session_count',count(*) FROM worker_sessions WHERE company_id='$company_id' AND employee_id='emp-frontend' UNION ALL SELECT 'live_frontend_session_count',count(*) FROM worker_sessions WHERE company_id='$company_id' AND employee_id='emp-frontend' AND state <> 'stopped' UNION ALL SELECT 'planner_relay_count',count(*) FROM messages WHERE company_id='$company_id' AND (sender='emp-planning' OR recipient='emp-planning') UNION ALL SELECT 'event_count',count(*) FROM events WHERE company_id='$company_id'" > "$EVIDENCE/counts.tsv"
fi

find /mnt/d/Programs/Polis/evidence/development -maxdepth 1 -type d -name 'r0.3a-pagination-v3-real-frontend-single-session-*' -printf '%f\n' | sort > "$EVIDENCE/frontend-run-paths.txt"
date -u +%Y-%m-%dT%H:%M:%SZ > "$EVIDENCE/forensic-finished-at.txt"
