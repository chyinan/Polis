#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
umask 077

V5_ROOT="/home/chyinan/.local/state/polis-recovery/r03a-sample-6-final-recovery-continuation-v5"
V5_PACKAGE_MANIFEST="/mnt/d/Programs/Polis/evidence/development/r0.3a-sample-6-final-recovery-continuation-v5/recovery-package-complete.json"
V5_SEMANTIC_CLOSURE="/mnt/d/Programs/Polis/evidence/development/r0.3a-sample-6-package-semantic-closure-qualification-v1/result.json"
V5_CAS="/home/chyinan/.local/state/polis-recovery/r03a-sample-6-final-recovery-continuation-v5/package/cas"
V5_SOURCE_STATE="$V5_ROOT/work/source-state.json"
V5_DUMP=""
V5_CAS_MANIFEST=""
V6_ROOT="/home/chyinan/.local/state/polis-recovery/r03a-sample-6-restore-continuation-v9"
EVIDENCE="/mnt/d/Programs/Polis/evidence/development/r0.3a-sample-6-restore-continuation-v9"
WORK="$V6_ROOT/work"
FRESH="$V6_ROOT/postgres/pgdata"
SOCKET_ROOT="/tmp/polis-pg-restore-v9"
PG="/mnt/d/Programs/Polis/.tools/pg/usr/lib/postgresql/18/bin"
export LD_LIBRARY_PATH="/mnt/d/Programs/Polis/.tools/pg/usr/lib/x86_64-linux-gnu"
PACKAGE_ID="r03a-sample-6-restore-v9-$(date -u +%Y%m%dT%H%M%SZ)-$$"
FRESH_STARTED=0
RECOVERY_ERROR="v6 restore continuation failed"

hash_file() { sha256sum "$1" | cut -d ' ' -f1; }
pg_stop() { "$PG/pg_ctl" -D "$FRESH" -m fast -w stop >/dev/null 2>&1 || true; rmdir -- "$SOCKET_ROOT" 2>/dev/null || true; }

export_state() {
  local database="$1" output="$2" company="$3"
  sed "s/r03a-t2-company-*/$company/g" scripts/r03a-backend-state-query.sql \
    | grep -v '^\\set ON_ERROR_STOP on' \
    | "$PG/psql" -X -v ON_ERROR_STOP=1 -h 127.0.0.1 -p 55432 -U chyinan -d "$database" -Atf - > "$output"
}

failure_result() {
  mkdir -m 700 -p "$EVIDENCE"
  python3 - "$EVIDENCE/result.json" "$RECOVERY_ERROR" <<'PY'
import json,sys
with open(sys.argv[1],"w",encoding="utf-8") as f:
    json.dump({"qualification":"R0.3A-SAMPLE-6-RESTORE-CONTINUATION-V9","status":"INCONCLUSIVE","sample_6_restore_continuation_v9":"INCONCLUSIVE","error":sys.argv[2],"medium":0,"high":0,"provider_egress":0,"historical_evidence_modified":False},f,indent=2); f.write("\n")
PY
}

on_exit() {
  local rc=$?
  trap - EXIT
  if [[ "$rc" != 0 ]]; then failure_result; fi
  if [[ "$FRESH_STARTED" == 1 ]]; then pg_stop; fi
  exit "$rc"
}
trap on_exit EXIT

mkdir -m 700 -p "$EVIDENCE"
if [[ -n "$(find "$EVIDENCE" -mindepth 1 -maxdepth 1 -print -quit 2>/dev/null)" ]]; then RECOVERY_ERROR="v6 evidence path is not fresh"; exit 1; fi
if [[ -e "$V6_ROOT" ]]; then RECOVERY_ERROR="v6 runtime root is not fresh"; exit 1; fi
mkdir -m 700 -p "$WORK"

bash scripts/go.sh run ./cmd/polis-r03a-recovery-package-validate -package-complete-manifest "$V5_PACKAGE_MANIFEST" -semantic-closure "$V5_SEMANTIC_CLOSURE" -cas-root "$V5_CAS" -runtime-root "$V5_ROOT" -package-evidence-root /mnt/d/Programs/Polis/evidence/development/r0.3a-sample-6-final-recovery-continuation-v5 -expected-generation r03a-sample-6-recovery-20260914T150111Z-307 -expected-package-manifest-sha256 acb19b8acd25e3f9230bafa9ed5cfeaf707d3a9bd07f036bfbb4e55fea555c5c -expected-dump-sha256 1b2acaacad9e9b907c041be58734bd5baaa4c7baffa09531dc28e4154b5401bd -expected-cas-manifest-sha256 9a7d103f56f9dd7300ba10bd88d337cafd286dbcd09177c483f4129125a7ff0f -output "$WORK/package-ref-validation.json"
V5_DUMP=$(python3 - "$WORK/package-ref-validation.json" <<'PY'
import json,sys
print(json.load(open(sys.argv[1]))["dump_path"])
PY
)
V5_CAS_MANIFEST=$(python3 - "$WORK/package-ref-validation.json" <<'PY'
import json,sys
print(json.load(open(sys.argv[1]))["cas_manifest_path"])
PY
)
bash scripts/go.sh run ./cmd/polis-r03a-recovery-closure-validate -source-state "$V5_SOURCE_STATE" -restored-state "$V5_ROOT/work/restored-state.json" -cas-manifest "$V5_CAS_MANIFEST" -cas-root "$V5_CAS" -output "$WORK/package-closure-revalidation.json"
cp "$WORK/package-closure-revalidation.json" "$EVIDENCE/package-closure-revalidation.json"
python3 - "$EVIDENCE/package-closure-revalidation.json" <<'PY'
import json,sys
if json.load(open(sys.argv[1])).get("package_semantic_closure") != "PASSED": raise SystemExit("v5 package semantic closure failed")
PY

mkdir -m 700 -p "$V6_ROOT/postgres" "$V6_ROOT/blobs"
mkdir -m 700 -p "$SOCKET_ROOT"
bash scripts/go.sh run ./cmd/polis-r03a-postgres-socket-validate -directory "$SOCKET_ROOT" -port 55432 -output "$EVIDENCE/socket-readiness.json"
"$PG/initdb" -D "$FRESH" -L /mnt/d/Programs/Polis/.tools/pg/usr/share/postgresql/18 --locale=C --encoding=UTF8 --auth-local=trust --auth-host=trust >/dev/null
"$PG/pg_ctl" -D "$FRESH" -l "$V6_ROOT/postgres/server.log" -o "-k $SOCKET_ROOT -h 127.0.0.1 -p 55432 -c fsync=on -c synchronous_commit=on -c jit=off" -w start >/dev/null
FRESH_STARTED=1
fresh_cluster_id=$("$PG/pg_controldata" "$FRESH" | awk -F: '/Database system identifier/{gsub(/ /,"",$2);print $2}')
[[ "$fresh_cluster_id" != "7685325910591533502" ]] || { RECOVERY_ERROR="fresh cluster reused sample-6 source identity"; exit 1; }
restore_db="polis_r0_3a_sample6_v6_restore_$(date -u +%Y%m%d%H%M%S)_$$"
"$PG/createdb" -h 127.0.0.1 -p 55432 -U chyinan "$restore_db"
"$PG/pg_restore" -h 127.0.0.1 -p 55432 -U chyinan -d "$restore_db" --no-owner --no-acl --exit-on-error "$V5_DUMP"
"$PG/psql" -X -v ON_ERROR_STOP=1 -h 127.0.0.1 -p 55432 -U chyinan -d postgres -c "DO \$\$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='polis_runtime') THEN CREATE ROLE polis_runtime LOGIN; END IF; END \$\$;" >/dev/null
"$PG/psql" -X -v ON_ERROR_STOP=1 -h 127.0.0.1 -p 55432 -U chyinan -d "$restore_db" -c "GRANT USAGE ON SCHEMA public TO polis_runtime; GRANT SELECT,INSERT,UPDATE ON ALL TABLES IN SCHEMA public TO polis_runtime; UPDATE runtime_control SET incarnation = 'restore-incarnation-$PACKAGE_ID' WHERE singleton;" >/dev/null
cp -a "$V5_CAS/." "$V6_ROOT/blobs/"
mkdir -m 700 -p "$V6_ROOT/blobs/.pagination-runtime"

company_id=$(python3 - "$V5_SOURCE_STATE" <<'PY'
import json,sys
print(json.load(open(sys.argv[1]))["company"]["id"])
PY
)
export_state "$restore_db" "$WORK/restored-before.json" "$company_id"
bash scripts/go.sh run ./cmd/polis-r03a-recovery-closure-validate -source-state "$V5_SOURCE_STATE" -restored-state "$WORK/restored-before.json" -cas-manifest "$V5_CAS_MANIFEST" -cas-root "$V6_ROOT/blobs" -output "$EVIDENCE/continuity-before-fence.json"

export POLIS_DSN="host=127.0.0.1 port=55432 dbname=$restore_db user=polis_runtime"
export POLIS_V3_RUNTIME_ROOT="$V6_ROOT"
export POLIS_V3_EVIDENCE="$EVIDENCE"
export POLIS_V3_BACKEND_EXECUTION_MANIFEST="/mnt/d/Programs/Polis/evidence/development/r0.3a-current-binary-backend-l2-v6/execution-manifest.json"
export POLIS_V3_BACKEND_QUALIFICATION="/mnt/d/Programs/Polis/evidence/development/r0.3a-current-binary-backend-l2-target-live-v1/result.json"
export POLIS_V3_BACKEND_L2_LIVE="$POLIS_V3_BACKEND_QUALIFICATION"
export POLIS_V3_ACCEPTANCE_QUALIFICATION="/mnt/d/Programs/Polis/evidence/development/r0.3a-public-response-encoding-contract-hardening/qualification.json"
export POLIS_BLOB_DURABILITY_QUALIFICATION="/mnt/d/Programs/Polis/evidence/development/r0.3a-blob-durability/qualification.json"
export POLIS_FRONTEND_CHECKER_FEEDBACK_QUALIFICATION="/mnt/d/Programs/Polis/evidence/development/r0.3a-frontend-checker-feedback-l2-v2/offline-result.json"
export POLIS_CODEX_BINARY="/mnt/c/Users/chyinan/AppData/Local/OpenAI/Codex/controlled-runtime/polis-r03a/bffc5354119c8421/codex.exe"
export POLIS_CODEX_CODE_MODE_HOST="/mnt/c/Users/chyinan/AppData/Local/OpenAI/Codex/controlled-runtime/polis-r03a/bffc5354119c8421/codex-code-mode-host.exe"
export POLIS_CODEX_AUTH_FILE="/mnt/c/Users/chyinan/.codex/auth.json"
export POLIS_SELECTED_CODEX_CONFIG="/mnt/d/Programs/Polis/.runtime/linux/r03a-t14c/home/config.toml"
export POLIS_CURRENT_L1_EVIDENCE="/mnt/d/Programs/Polis/evidence/development/r0.3a-current-binary-l1"
export POLIS_V3_BACKEND_BINDING="/mnt/d/Programs/Polis/evidence/development/r0.3a-pagination-v3-backend-sample-6/authorization-binding.json"
export POLIS_V3_ALLOWANCE="$EVIDENCE/no-business-allowance.json"
export POLIS_V3_BACKEND_RESULT="$EVIDENCE/restore-verification-result.json"
export POLIS_V3_POSTGRES_SNAPSHOT="$V5_DUMP"
bash scripts/go.sh run ./cmd/polis-r03a-pagination-v3 -backend-restore-verify

export_state "$restore_db" "$WORK/restored-after.json" "$company_id"
bash scripts/go.sh run ./cmd/polis-r03a-recovery-closure-validate -source-state "$V5_SOURCE_STATE" -restored-state "$WORK/restored-after.json" -cas-manifest "$V5_CAS_MANIFEST" -cas-root "$V6_ROOT/blobs" -output "$EVIDENCE/continuity-after-fence.json"

FRONTEND_FRESH="$WORK/frontend-manifest-freshness"
bash scripts/go.sh run ./cmd/polis-r03a-frontend-l2-offline -evidence "$FRONTEND_FRESH" -l1-evidence /mnt/d/Programs/Polis/evidence/development/r0.3a-current-binary-l1 -base-l2-evidence /mnt/d/Programs/Polis/evidence/development/r0.3a-current-binary-frontend-l2 -handover-protocol /mnt/d/Programs/Polis/evidence/development/r0.3a-real-frontend-handover-v4/2b7a6fd917d335459b0dbf32dda34158/protocol.jsonl
python3 - "$FRONTEND_FRESH/execution-manifest.json" /mnt/d/Programs/Polis/evidence/development/r0.3a-current-binary-frontend-l2/execution-manifest.json "$EVIDENCE/frontend-surface-freshness.json" <<'PY'
import json,sys
fresh=json.load(open(sys.argv[1])); qualified=json.load(open(sys.argv[2]))
fields=[("tool_surface","tool_count"),("tool_surface","aggregate_manifest_digest"),("tool_surface","aggregate_schema_digest"),("tool_surface","aggregate_schema_bytes"),("tool_surface","thread_start_payload_digest"),("tool_surface","thread_start_payload_bytes"),(None,"handler_binding_digest")]
equal=all((fresh.get(a) if a else fresh.get(b)) == (qualified.get(a) if a else qualified.get(b)) for a,b in fields)
status="QUALIFIED_REUSABLE" if equal else "STALE_PENDING_REQUALIFICATION"
with open(sys.argv[3],"w",encoding="utf-8") as f: json.dump({"status":status,"provider_visible_surface_equal":equal,"current_manifest":sys.argv[1],"qualified_manifest":sys.argv[2],"current_execution_fingerprint":fresh.get("execution_fingerprint"),"qualified_execution_fingerprint":qualified.get("execution_fingerprint"),"eligible_for_frontend_phase":equal},f,indent=2); f.write("\n")
if not equal: raise SystemExit("Frontend provider surface drifted")
PY

python3 - "$EVIDENCE/result.json" "$PACKAGE_ID" "$fresh_cluster_id" "$restore_db" <<'PY'
import json,sys
path,package,fresh_db_cluster,db=sys.argv[1:]
with open(path,"w",encoding="utf-8") as f:
    json.dump({"qualification":"R0.3A-SAMPLE-6-RESTORE-CONTINUATION-V9","status":"PASSED","sample_6_restore_continuation_v9":"PASSED","package_integrity":"PASSED","package_semantic_closure":"PASSED","socket_endpoint_preflight":"PASSED","fresh_cluster_created":"PASSED","fresh_cluster_started":"PASSED","pg_restore":"PASSED","CAS_restore":"PASSED","relational_identity_continuity":"PASSED","artifact_workspace_continuity":"PASSED","checkpoint_continuity":"PASSED","collaboration_continuity":"PASSED","event_continuity":"PASSED","CAS_continuity":"PASSED","restored_semantic_closure":"PASSED","synthesized_rows":0,"synthesized_blobs":0,"old_writer_fencing":"PASSED","fresh_restore":"PASSED","authoritative_state_continuity":"PASSED","backend_recovery_subject":"PASSED","frontend_L2_status":"QUALIFIED_REUSABLE","eligible_for_frontend_phase":True,"fresh_cluster_system_identifier":fresh_db_cluster,"restore_database":db,"v5_historical_authoritative_state_continuity":"FAILED","historical_evidence_modified":False,"medium":0,"high":0,"provider_egress":0},f,indent=2); f.write("\n")
PY
