#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
umask 077
REPO="/mnt/d/Programs/Polis"
EVIDENCE="$REPO/evidence/development/r0.3a-frontend-backend-artifact-anchor-forensics"
CLEAN_ROOT="/home/chyinan/.local/state/polis-recovery/r03a-frontend-clean-baseline-v1"
PGDATA="$CLEAN_ROOT/postgres/pgdata"
SOCKET="/tmp/polis-pg-r03a-artifact-anchor-forensics"
PG="$REPO/.tools/pg/usr/lib/postgresql/18/bin"
export LD_LIBRARY_PATH="$REPO/.tools/pg/usr/lib/x86_64-linux-gnu"
DB="polis_r0_3a_frontend_clean_baseline_v1"
COMPANY="r03a-pagination-v3-company-1789379324974307400"
TASK="dff11354ffe0afc1b93e8565dc5bb026"
ARTIFACT="f73981340227622e9c16dbf45e961a3f"
PACKAGE_DUMP="/home/chyinan/.local/state/polis-recovery/r03a-sample-6-final-recovery-continuation-v5/package/sample-6-authoritative.dump"
V5_SOURCE_STATE="/home/chyinan/.local/state/polis-recovery/r03a-sample-6-final-recovery-continuation-v5/work/source-state.json"
mkdir -m 700 -p "$EVIDENCE"
if [[ -n "$(find "$EVIDENCE" -mindepth 1 -maxdepth 1 -print -quit 2>/dev/null)" ]]; then echo "forensic evidence path is not fresh" >&2; exit 1; fi
started=0
cleanup(){ if [[ "$started" == 1 ]]; then "$PG/pg_ctl" -D "$PGDATA" -m fast -w stop >/dev/null 2>&1 || true; fi; rmdir -- "$SOCKET" 2>/dev/null || true; }
trap cleanup EXIT
mkdir -m 700 -p "$SOCKET"
"$PG/pg_ctl" -D "$PGDATA" -l "$CLEAN_ROOT/postgres/artifact-forensics.log" -o "-k $SOCKET -h 127.0.0.1 -p 55432 -c synchronous_commit=on" -w start >/dev/null
started=1

RO=(env PGOPTIONS=-c\ default_transaction_read_only=on "$PG/psql" -X -h 127.0.0.1 -p 55432 -U chyinan -d "$DB" -v ON_ERROR_STOP=1)
"${RO[@]}" -At -F $'\t' -c "SELECT row_to_json(a),row_to_json(t),row_to_json(aq),row_to_json(cr) FROM artifacts a JOIN tasks t ON t.company_id=a.company_id AND t.id=a.task_id LEFT JOIN artifact_qualifications aq ON aq.company_id=a.company_id AND aq.artifact_id=a.id LEFT JOIN contract_revisions cr ON cr.company_id=a.company_id AND cr.id=aq.contract_revision_id WHERE a.company_id='$COMPANY' AND a.task_id='$TASK' ORDER BY a.id" > "$EVIDENCE/current-artifacts.tsv"
"${RO[@]}" -At -F $'\t' -c "SELECT row_to_json(x) FROM (SELECT t.id AS task_id,t.owner,t.kind,t.state,t.generation,a.id AS artifact_id,a.author,a.state AS artifact_state,a.verdict,a.digest,a.bytes,a.contract,aq.checkpoint_id,aq.workspace_revision,aq.workspace_digest,aq.contract_revision_id,cr.state AS contract_state FROM tasks t LEFT JOIN artifacts a ON a.company_id=t.company_id AND a.task_id=t.id LEFT JOIN artifact_qualifications aq ON aq.company_id=a.company_id AND aq.artifact_id=a.id LEFT JOIN contract_revisions cr ON cr.company_id=aq.company_id AND cr.id=aq.contract_revision_id WHERE t.company_id='$COMPANY' AND t.id='$TASK') x" > "$EVIDENCE/anchor-binding.jsonl"
"${RO[@]}" -At -F $'\t' -c "SELECT row_to_json(x) FROM (SELECT cp.id,cp.session_id,cp.data FROM worker_checkpoints cp JOIN worker_sessions s ON s.company_id=cp.company_id AND s.id=cp.session_id WHERE cp.company_id='$COMPANY' AND s.task_id='$TASK' ORDER BY cp.id) x" > "$EVIDENCE/backend-checkpoints.jsonl"
"${RO[@]}" -At -F $'\t' -c "SELECT row_to_json(x) FROM (SELECT c.id,c.company_seq,(SELECT count(*) FROM worker_sessions s WHERE s.company_id=c.id AND s.employee_id='emp-frontend') AS frontend_sessions,(SELECT count(*) FROM worker_sessions s WHERE s.company_id=c.id AND s.employee_id='emp-frontend' AND s.state!='stopped') AS live_frontend_writers,(SELECT json_agg(e ORDER BY e.company_seq) FROM (SELECT company_seq,kind,payload FROM events WHERE company_id=c.id ORDER BY company_seq DESC LIMIT 12) e) AS event_tail FROM companies c WHERE c.id='$COMPANY') x" > "$EVIDENCE/control-plane-current.jsonl"
"${RO[@]}" -At -F $'\t' -c "SELECT row_to_json(x) FROM (SELECT e.epoch,(SELECT incarnation FROM runtime_control WHERE singleton) AS runtime_incarnation,(SELECT count(*) FROM events WHERE company_id='$COMPANY') AS event_count,(SELECT company_seq FROM companies WHERE id='$COMPANY') AS company_seq FROM employees e WHERE e.company_id='$COMPANY' AND e.id='emp-frontend') x" > "$EVIDENCE/epoch-runtime-current.jsonl"

mkdir -p "$EVIDENCE/package-artifact-inspection"
"$PG/pg_restore" --data-only --table=public.artifacts -f - "$PACKAGE_DUMP" > "$EVIDENCE/package-artifact-inspection/artifacts.sql"
"$PG/pg_restore" --list "$PACKAGE_DUMP" > "$EVIDENCE/package-artifact-inspection/toc.txt"
cp -- "$V5_SOURCE_STATE" "$EVIDENCE/package-source-state.json"
printf '%s\n' "$ARTIFACT" > "$EVIDENCE/expected-artifact-id.txt"
printf '%s\n' "$TASK" > "$EVIDENCE/expected-task-id.txt"
date -u +%Y-%m-%dT%H:%M:%SZ > "$EVIDENCE/forensic-finished-at.txt"
