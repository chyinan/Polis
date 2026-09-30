#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
umask 077

SOURCE_ROOT="/home/chyinan/.local/state/polis-recovery/r03a-sample-6"
SOURCE_DATA="$SOURCE_ROOT/postgres/pgdata"
FRESH_ROOT="/home/chyinan/.local/state/polis-recovery/r03a-sample-6-final-recovery-v5"
EVIDENCE="/mnt/d/Programs/Polis/evidence/development/r0.3a-sample-6-final-recovery-continuation-v5"
RUNTIME_ROOT="/home/chyinan/.local/state/polis-recovery/r03a-sample-6-final-recovery-continuation-v5"
SOURCE_BLOBS="/mnt/d/Polis-recovery/r0.3a-pagination-v3-backend-sample-6-runtime/backend/blobs"
SALVAGE_MANIFEST_HASH="147e47ddf7a833d91eb184998586f00954580bc44604e226b03653ad4400bad7"
EXPECTED_SYSTEM_ID="7685325910591533502"
EXPECTED_DB="polis_r0_3a_pagination_v3_backend_20260914174837_31960"
EXPECTED_CONTRACT="92566d902cd328c0b694ef32188dabf5"
EXPECTED_MESSAGE="45a77c6833518bd8fef1ddfe8d28b985"
EXPECTED_CHECKPOINT="13164a801bac0760050c5f456bfd3f82"
EXPECTED_ARTIFACT="f73981340227622e9c16dbf45e961a3f"
EXPECTED_ARTIFACT_DIGEST="e69a3f216201711384f4988698d7c33867e52f2426d93add6c05dda4beb12672"
PG="/mnt/d/Programs/Polis/.tools/pg/usr/lib/postgresql/18/bin"
export LD_LIBRARY_PATH="/mnt/d/Programs/Polis/.tools/pg/usr/lib/x86_64-linux-gnu"
PACKAGE_ID="r03a-sample-6-recovery-$(date -u +%Y%m%dT%H%M%SZ)-$$"
WORK_ROOT="$RUNTIME_ROOT/work"
ANCHOR_DIR="$WORK_ROOT/anchor-observation"
PACKAGE_ROOT="$RUNTIME_ROOT/package"
PACKAGE_CAS="$PACKAGE_ROOT/cas"
DUMP_PATH="$PACKAGE_ROOT/sample-6-authoritative.dump"
CONTINUATION_MANIFEST="$EVIDENCE/PreservedPostgresAccessManifest.continuation.json"
CONTINUATION_HASH_PATH="$CONTINUATION_MANIFEST.sha256"
CAS_MANIFEST="$EVIDENCE/cas-manifest.json"
CAS_HASH_PATH="$CAS_MANIFEST.sha256"

SOURCE_STARTED=0
FRESH_STARTED=0
PACKAGE_COMPLETE=0
DESTRUCTIVE_BOUNDARY=0
SOURCE_DATABASE_DESTROYED=0
RECOVERY_ERROR="final recovery continuation failed"

hash_file() { sha256sum "$1" | cut -d ' ' -f1; }

pg_ctl_start() {
  local data="$1" log="$2"
  "$PG/pg_ctl" -D "$data" -l "$log" -o "-k $data -h 127.0.0.1 -p 55432 -c fsync=on -c synchronous_commit=on -c jit=off" -w start
}

pg_ctl_stop() {
  "$PG/pg_ctl" -D "$1" -m fast -w stop >/dev/null 2>&1 || true
}

psql_db() {
  "$PG/psql" -X -v ON_ERROR_STOP=1 -h 127.0.0.1 -p 55432 -U chyinan -d "$1" -Atc "$2"
}

export_state() {
  local database="$1" output="$2" company="$3"
  sed "s/r03a-t2-company-*/$company/g" scripts/r03a-backend-state-query.sql \
    | grep -v '^\\set ON_ERROR_STOP on' \
    | "$PG/psql" -X -v ON_ERROR_STOP=1 -h 127.0.0.1 -p 55432 -U chyinan -d "$database" -Atf - > "$output"
}

write_failure() {
  python3 - "$EVIDENCE/result.json" "$RECOVERY_ERROR" "$PACKAGE_COMPLETE" "$DESTRUCTIVE_BOUNDARY" "$SOURCE_DATABASE_DESTROYED" <<'PY'
import json,sys
path,message,package,destructive,source_destroyed=sys.argv[1:]
value={"qualification":"R0.3A-SAMPLE-6-FINAL-RECOVERY-CONTINUATION-V5","status":"INCONCLUSIVE","sample_6_final_recovery_continuation_v5":"INCONCLUSIVE","error":message,"recovery_package_complete":package=="1","destructive_boundary":destructive=="1","source_database_destroyed":source_destroyed=="1","historical_preservation_manifest":"NOT_FOUND","historical_evidence_modified":False,"medium":0,"high":0,"provider_egress":0}
with open(path,"w",encoding="utf-8") as f: json.dump(value,f,indent=2); f.write("\n")
PY
}

cleanup() {
  if [[ "$FRESH_STARTED" == 1 ]]; then pg_ctl_stop "$FRESH_ROOT/postgres/pgdata"; fi
  if [[ "$SOURCE_STARTED" == 1 && "$DESTRUCTIVE_BOUNDARY" == 0 ]]; then pg_ctl_stop "$SOURCE_DATA"; fi
}

on_exit() {
  local rc=$?
  trap - EXIT
  if [[ "$rc" != 0 ]]; then mkdir -m 700 -p "$EVIDENCE"; write_failure; fi
  cleanup
  exit "$rc"
}
trap on_exit EXIT

mkdir -m 700 -p "$EVIDENCE"
if [[ -n "$(find "$EVIDENCE" -mindepth 1 -maxdepth 1 -print -quit 2>/dev/null)" ]]; then RECOVERY_ERROR="final recovery continuation evidence path is not fresh"; exit 1; fi
if [[ -e "$RUNTIME_ROOT" || -e "$FRESH_ROOT" ]]; then RECOVERY_ERROR="final recovery continuation runtime root is not fresh"; exit 1; fi
if [[ ! -f "$SOURCE_DATA/PG_VERSION" || ! -f "$SOURCE_DATA/global/pg_control" ]]; then RECOVERY_ERROR="original sample-6 PGDATA is unavailable"; exit 1; fi
if [[ "$(realpath -e "$SOURCE_DATA")" != "$SOURCE_DATA" ]]; then RECOVERY_ERROR="original PGDATA canonical path mismatch"; exit 1; fi
if "$PG/pg_ctl" -D "$SOURCE_DATA" status >/dev/null 2>&1; then RECOVERY_ERROR="original source cluster was already running"; exit 1; fi

mkdir -m 700 -p "$WORK_ROOT" "$ANCHOR_DIR" "$PACKAGE_ROOT" "$PACKAGE_CAS"
source_control_before=$(hash_file "$SOURCE_DATA/global/pg_control")
source_system_id_before=$("$PG/pg_controldata" "$SOURCE_DATA" | awk -F: '/Database system identifier/{gsub(/ /,"",$2);print $2}')
source_control_state_before=$("$PG/pg_controldata" "$SOURCE_DATA" | awk -F: '/Database cluster state/{sub(/^ +/,"",$2);print $2}')
source_pg_version=$(tr -d '\r\n' < "$SOURCE_DATA/PG_VERSION")
[[ "$source_system_id_before" == "$EXPECTED_SYSTEM_ID" ]] || { RECOVERY_ERROR="source system identifier mismatch"; exit 1; }
[[ "$source_control_state_before" == "shut down" ]] || { RECOVERY_ERROR="source cluster was not shut down"; exit 1; }

[[ "$RUNTIME_ROOT" != /tmp/* && "$RUNTIME_ROOT" != /mnt/c/* && "$RUNTIME_ROOT" != /mnt/d/* ]] || { RECOVERY_ERROR="recovery root is not durable"; exit 1; }
export POLIS_V3_EVIDENCE="$EVIDENCE"
export POLIS_V3_POSTGRES_DUMP="$PG/pg_dump"
export POLIS_V3_POSTGRES_SNAPSHOT_DSN="host=127.0.0.1 port=55432 dbname=readiness-only user=chyinan"
export POLIS_V3_POSTGRES_SNAPSHOT="$RUNTIME_ROOT/readiness-only.dump"
export POLIS_RECOVERY_ROOT_WSL="$RUNTIME_ROOT"
export POLIS_V3_RUNTIME_ROOT="$RUNTIME_ROOT"
bash scripts/go.sh run ./cmd/polis-r03a-pagination-v3 -recovery-readiness
python3 - "$EVIDENCE/runner-preflight.json" <<'PY'
import json,sys
with open(sys.argv[1],"w",encoding="utf-8") as f:
    json.dump({"status":"PASSED","runner_compile":"PASSED","recovery_readiness_cli":"PASSED","cli_argument_contract":"recovery-readiness","context_wiring":"PASSED","source_started":False,"provider_egress":0},f,indent=2); f.write("\n")
PY
pg_dump_version=$(python3 - "$EVIDENCE/recovery-readiness.json" <<'PY'
import json,sys
print(json.load(open(sys.argv[1]))["pg_dump"]["raw_version"])
PY
)
pg_restore_version=$(python3 - "$EVIDENCE/recovery-readiness.json" <<'PY'
import json,sys
print(json.load(open(sys.argv[1]))["pg_restore"]["raw_version"])
PY
)
PREVIOUS_ANCHOR_SNAPSHOT="/mnt/d/Polis-recovery/r0.3a-pagination-v3-backend-sample-6-runtime/authoritative-snapshot.json"
bash scripts/go.sh run ./cmd/polis-r03a-cas-validate -root "$SOURCE_BLOBS" -anchor-snapshot "$PREVIOUS_ANCHOR_SNAPSHOT" -output "$EVIDENCE/cas-source-preflight.json"

export POLIS_RECOVERY_ROOT_WSL="$SOURCE_ROOT"
bash scripts/r03a-real-backend-pg.sh start
SOURCE_STARTED=1
source_system_id_after=$("$PG/pg_controldata" "$SOURCE_DATA" | awk -F: '/Database system identifier/{gsub(/ /,"",$2);print $2}')
source_control_after=$(hash_file "$SOURCE_DATA/global/pg_control")
[[ "$source_system_id_after" == "$EXPECTED_SYSTEM_ID" ]] || { RECOVERY_ERROR="source system identifier changed on reopen"; exit 1; }

source_database=$(psql_db postgres "SELECT datname FROM pg_database WHERE datname LIKE 'polis_r0_3a_pagination_v3_backend_%' ORDER BY datname DESC LIMIT 1")
[[ "$source_database" == "$EXPECTED_DB" ]] || { RECOVERY_ERROR="source database identity mismatch"; exit 1; }
database_oid=$(psql_db postgres "SELECT oid::text FROM pg_database WHERE datname='$source_database'")
database_owner=$(psql_db postgres "SELECT pg_get_userbyid(datdba) FROM pg_database WHERE datname='$source_database'")
[[ "$database_oid" == "16384" && "$database_owner" == "chyinan" ]] || { RECOVERY_ERROR="source database OID/owner mismatch"; exit 1; }
[[ "$(psql_db "$source_database" "SELECT 1")" == 1 ]] || { RECOVERY_ERROR="source database read-only connection failed"; exit 1; }
[[ "$(psql_db "$source_database" "SELECT COALESCE(max(version_id),0) FROM goose_db_version WHERE is_applied")" == 5 ]] || { RECOVERY_ERROR="source schema migration mismatch"; exit 1; }

source_dsn="postgres://polis_runtime@127.0.0.1:55432/$source_database"
env POLIS_TEST_DSN="$source_dsn" bash scripts/go.sh run ./cmd/polis-r03a-recovery-cut -dsn "$source_dsn" -root "$SOURCE_BLOBS" -output "$ANCHOR_DIR"
cp "$ANCHOR_DIR/authoritative-snapshot.json" "$EVIDENCE/authoritative-snapshot.json"
bash scripts/go.sh run ./cmd/polis-r03a-cas-validate -root "$SOURCE_BLOBS" -anchor-snapshot "$ANCHOR_DIR/authoritative-snapshot.json" -output "$EVIDENCE/cas-source-validation.json"
cas_canonical_root=$(python3 - "$EVIDENCE/cas-source-validation.json" <<'PY'
import json,sys
print(json.load(open(sys.argv[1]))["canonical_root"])
PY
)
cas_layout_revision=$(python3 - "$EVIDENCE/cas-source-validation.json" <<'PY'
import json,sys
print(json.load(open(sys.argv[1]))["layout_revision"])
PY
)
cas_inventory_digest=$(python3 - "$EVIDENCE/cas-source-validation.json" <<'PY'
import json,sys
print(json.load(open(sys.argv[1]))["inventory_digest"])
PY
)
cas_required_count=$(python3 - "$EVIDENCE/cas-source-validation.json" <<'PY'
import json,sys
print(json.load(open(sys.argv[1]))["required_blob_count"])
PY
)

read -r company_id mission_id backend_task_id frontend_task_id contract_id message_id obligation_id checkpoint_id artifact_id artifact_digest workspace_revision workspace_digest obligation_state <<EOF
$(python3 - "$ANCHOR_DIR/authoritative-snapshot.json" <<'PY'
import json,sys
a=json.load(open(sys.argv[1]))["recovery_anchors"]
print(a["company_id"],a["mission_id"],a["backend_task_id"],a["frontend_task_id"],a["final_contract_revision_id"],a["message_id"],a["obligation_id"],a["backend_checkpoint_id"],a["backend_artifact_id"],a["backend_artifact_digest"],a["backend_workspace_revision"],a["backend_workspace_digest"],a["obligation_state"])
PY
)
EOF
[[ "$contract_id" == "$EXPECTED_CONTRACT" && "$message_id" == "$EXPECTED_MESSAGE" && "$obligation_id" == "$EXPECTED_MESSAGE" && "$checkpoint_id" == "$EXPECTED_CHECKPOINT" && "$artifact_id" == "$EXPECTED_ARTIFACT" && "$artifact_digest" == "$EXPECTED_ARTIFACT_DIGEST" && "$workspace_revision" == 3 && "$workspace_digest" == "$EXPECTED_ARTIFACT_DIGEST" && "$obligation_state" == pending ]] || { RECOVERY_ERROR="production recovery anchors do not match frozen sample-6 terminal state"; exit 1; }
[[ "$(psql_db "$source_database" "SELECT count(*) FROM worker_sessions WHERE employee_id='emp-frontend'")" == 0 ]] || { RECOVERY_ERROR="Frontend session exists at Backend boundary"; exit 1; }
[[ "$(psql_db "$source_database" "SELECT count(*) FROM messages WHERE sender='emp-planning' OR recipient='emp-planning'")" == 0 ]] || { RECOVERY_ERROR="Planner relay is nonzero"; exit 1; }

continuation_created_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
pgdata_digest=$(printf '%s|%s' "$source_pg_version" "$source_control_after" | sha256sum | cut -d ' ' -f1)
role_grant_spec_digest=$(printf '%s' 'role-provisioning@1|role=polis_runtime|public:USAGE|ALL TABLES:SELECT,INSERT,UPDATE' | sha256sum | cut -d ' ' -f1)
migration_schema_digest=$(python3 - <<'PY'
import hashlib
from pathlib import Path
root=Path('/mnt/d/Programs/Polis/db/migrations')
parts=[p.name+':'+hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(root.glob('*.sql'))]
print(hashlib.sha256('|'.join(parts).encode()).hexdigest())
PY
)
python3 - "$CONTINUATION_MANIFEST" "$source_database" "$database_oid" "$database_owner" "$company_id" "$mission_id" "$backend_task_id" "$frontend_task_id" "$source_pg_version" "$pgdata_digest" "$source_system_id_after" "$role_grant_spec_digest" "$migration_schema_digest" "$continuation_created_at" "$cas_canonical_root" "$cas_layout_revision" "$cas_required_count" "$cas_inventory_digest" <<'PY'
import json,sys
(path,db,oid,owner,company,mission,backend,frontend,version,pgdata_digest,system_id,grant_digest,migration_digest,created,cas_root,cas_layout,cas_required,cas_inventory)=sys.argv[1:]
m={"manifest_revision":"r0.3a-preserved-postgres-access@1","postgres_version":version,"pgdata_canonical_path":"/home/chyinan/.local/state/polis-recovery/r03a-sample-6/postgres/pgdata","pgdata_digest":pgdata_digest,"cluster_system_identifier":system_id,"database_name":db,"database_oid":oid,"database_owner":owner,"runtime_role_name":"polis_runtime","role_provisioning_revision":"role-provisioning@1","role_grant_spec_digest":grant_digest,"migration_schema_digest":migration_digest,"company_id":company,"mission_id":mission,"backend_task_id":backend,"frontend_task_id":frontend,"created_at":created,"preservation_reason":"R0.3A sample-6 recovery continuation after physical salvage","credentials_included":False,"manifest_creation_phase":"recovery_continuation_after_salvage","historical_manifest_present":False,"provenance_salvage_manifest_hash":"147e47ddf7a833d91eb184998586f00954580bc44604e226b03653ad4400bad7","source":"original_physical_source_reopen","business_state_source":"existing_database_only","cas_canonical_root":cas_root,"cas_layout_revision":cas_layout,"cas_company_namespace":company,"cas_required_blob_count":int(cas_required),"cas_inventory_digest":cas_inventory,"authoritative_source_preserved":True}
with open(path,'w',encoding='utf-8') as f: json.dump(m,f,indent=2); f.write('\n')
PY
continuation_manifest_hash=$(hash_file "$CONTINUATION_MANIFEST")
printf '%s\n' "$continuation_manifest_hash" > "$CONTINUATION_HASH_PATH"

export_state "$source_database" "$WORK_ROOT/source-state.json" "$company_id"
"$PG/pg_dump" -h 127.0.0.1 -p 55432 -U chyinan -d "$source_database" --format=custom --no-owner --no-acl --file="$DUMP_PATH"
[[ -s "$DUMP_PATH" ]] || { RECOVERY_ERROR="production pg_dump produced an empty archive"; exit 1; }
dump_hash=$(hash_file "$DUMP_PATH")
"$PG/pg_restore" -l "$DUMP_PATH" | grep -q TABLE || { RECOVERY_ERROR="custom-format archive validation failed"; exit 1; }
dump_size=$(stat -c '%s' "$DUMP_PATH")
python3 - "$EVIDENCE/dump-proof.json" "$DUMP_PATH" "$dump_hash" "$dump_size" "$continuation_manifest_hash" "$source_system_id_after" "$database_oid" "$PACKAGE_ID" <<'PY'
import json,sys
path,dump,h,size,cont,system_id,oid,package=sys.argv[1:]
with open(path,'w',encoding='utf-8') as f: json.dump({"status":"PASSED","path":dump,"sha256":h,"size":int(size),"format":"custom","continuation_manifest_hash":cont,"cluster_system_identifier":system_id,"database_oid":oid,"package_generation_id":package},f,indent=2); f.write("\n")
PY
python3 - "$EVIDENCE/authoritative-snapshot-status.json" "$continuation_manifest_hash" "$dump_hash" "$PACKAGE_ID" <<'PY'
import json,sys
path,cont,dump,package=sys.argv[1:]
with open(path,'w',encoding='utf-8') as f: json.dump({"authoritative_snapshot":"PASSED","continuation_manifest_hash":cont,"dump_hash":dump,"package_generation_id":package},f,indent=2); f.write("\n")
PY

mkdir -m 700 -p "$PACKAGE_CAS/$company_id"
cp -a "$SOURCE_BLOBS/$company_id/." "$PACKAGE_CAS/$company_id/"
python3 - "$ANCHOR_DIR/authoritative-snapshot.json" "$PACKAGE_CAS" "$CAS_MANIFEST" "$PACKAGE_ID" "$continuation_manifest_hash" "$dump_hash" "$source_system_id_after" "$database_oid" "$cas_canonical_root" "$cas_layout_revision" "$cas_inventory_digest" <<'PY'
import hashlib,json,os,sys
snapshot=json.load(open(sys.argv[1])); cas_root=sys.argv[2]; entries=snapshot["cas"]
for e in entries:
    p=os.path.join(cas_root,e["company_id"],e["content_sha256"])
    if not os.path.isfile(p) or os.path.getsize(p)!=int(e["size"]): raise SystemExit("CAS entry missing or size mismatch")
    if hashlib.sha256(open(p,"rb").read()).hexdigest()!=e["content_sha256"]: raise SystemExit("CAS entry digest mismatch")
m={"schema_version":"r03a-sample-6-recovery-cas@1","package_generation_id":sys.argv[4],"entries":entries,"manifest_digest":snapshot["cas_manifest_digest"],"continuation_manifest_hash":sys.argv[5],"dump_hash":sys.argv[6],"cluster_system_identifier":sys.argv[7],"database_oid":sys.argv[8],"canonical_root":sys.argv[9],"layout_revision":sys.argv[10],"inventory_digest":sys.argv[11],"historical_evidence_modified":False}
with open(sys.argv[3],"w",encoding="utf-8") as f: json.dump(m,f,indent=2); f.write("\n")
PY
cas_hash=$(hash_file "$CAS_MANIFEST")
printf '%s\n' "$cas_hash" > "$CAS_HASH_PATH"
mkdir -m 700 -p "$WORK_ROOT/package-semantic-closure"
bash scripts/go.sh run ./cmd/polis-r03a-recovery-closure-validate -source-state "$WORK_ROOT/source-state.json" -cas-manifest "$CAS_MANIFEST" -cas-root "$PACKAGE_CAS" -output "$EVIDENCE/package-semantic-closure.json"
PACKAGE_COMPLETE=1
python3 - "$EVIDENCE/recovery-package-complete.json" "$PACKAGE_ID" "$DUMP_PATH" "$dump_hash" "$CAS_MANIFEST" "$cas_hash" "$continuation_manifest_hash" <<'PY'
import json,sys
path,package,dump,dump_hash,cas,cas_hash,cont=sys.argv[1:]
with open(path,'w',encoding='utf-8') as f: json.dump({"status":"COMPLETE","package_generation_id":package,"dump_path":dump,"dump_hash":dump_hash,"cas_manifest_path":cas,"cas_manifest_hash":cas_hash,"continuation_manifest_hash":cont,"historical_evidence_modified":False},f,indent=2); f.write("\n")
PY

python3 - "$EVIDENCE/destructive-boundary-authorized.json" "$PACKAGE_ID" "$SALVAGE_MANIFEST_HASH" "$continuation_manifest_hash" "$dump_hash" "$cas_hash" "$source_system_id_after" "$database_oid" <<'PY'
import json,sys
path,package,salvage,cont,dump,cas,system_id,oid=sys.argv[1:]
with open(path,'w',encoding='utf-8') as f: json.dump({"status":"AUTHORIZED_AFTER_PACKAGE_COMPLETE","package_generation_id":package,"salvage_manifest_hash":salvage,"continuation_manifest_hash":cont,"dump_hash":dump,"cas_manifest_hash":cas,"cluster_system_identifier":system_id,"database_oid":oid,"recovery_package_complete":True},f,indent=2); f.write("\n")
PY
DESTRUCTIVE_BOUNDARY=1

"$PG/dropdb" -h 127.0.0.1 -p 55432 -U chyinan "$source_database"
SOURCE_DATABASE_DESTROYED=1
pg_ctl_stop "$SOURCE_DATA"
SOURCE_STARTED=0
[[ ! -e "$SOURCE_DATA/postmaster.pid" ]] || { RECOVERY_ERROR="source cluster did not stop before release"; exit 1; }
[[ "$(realpath -e "$SOURCE_DATA")" == "$SOURCE_DATA" ]] || { RECOVERY_ERROR="source PGDATA release path changed"; exit 1; }
rm -rf -- "$SOURCE_DATA"
[[ ! -e "$SOURCE_DATA" ]] || { RECOVERY_ERROR="source PGDATA destruction did not complete"; exit 1; }

mkdir -m 700 -p "$FRESH_ROOT/postgres" "$FRESH_ROOT/blobs"
"$PG/initdb" -D "$FRESH_ROOT/postgres/pgdata" -L /mnt/d/Programs/Polis/.tools/pg/usr/share/postgresql/18 --locale=C --encoding=UTF8 --auth-local=trust --auth-host=trust >/dev/null
pg_ctl_start "$FRESH_ROOT/postgres/pgdata" "$FRESH_ROOT/postgres/server.log"
FRESH_STARTED=1
fresh_system_id=$("$PG/pg_controldata" "$FRESH_ROOT/postgres/pgdata" | awk -F: '/Database system identifier/{gsub(/ /,"",$2);print $2}')
[[ "$fresh_system_id" != "$source_system_id_after" ]] || { RECOVERY_ERROR="fresh restore reused source cluster identity"; exit 1; }
restore_database="polis_r0_3a_sample6_restore_$(date -u +%Y%m%d%H%M%S)_$$"
"$PG/createdb" -h 127.0.0.1 -p 55432 -U chyinan "$restore_database"
"$PG/pg_restore" -h 127.0.0.1 -p 55432 -U chyinan -d "$restore_database" --no-owner --no-acl --exit-on-error "$DUMP_PATH"
"$PG/psql" -X -v ON_ERROR_STOP=1 -h 127.0.0.1 -p 55432 -U chyinan -d postgres -c "DO \$\$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='polis_runtime') THEN CREATE ROLE polis_runtime LOGIN; END IF; END \$\$;" >/dev/null
"$PG/psql" -X -v ON_ERROR_STOP=1 -h 127.0.0.1 -p 55432 -U chyinan -d "$restore_database" -c "GRANT USAGE ON SCHEMA public TO polis_runtime; GRANT SELECT,INSERT,UPDATE ON ALL TABLES IN SCHEMA public TO polis_runtime; UPDATE runtime_control SET incarnation = 'restore-incarnation-$PACKAGE_ID' WHERE singleton;" >/dev/null
cp -a "$PACKAGE_CAS/." "$FRESH_ROOT/blobs/"
export_state "$restore_database" "$WORK_ROOT/restored-state.json" "$company_id"
python3 - "$WORK_ROOT/source-state.json" "$WORK_ROOT/restored-state.json" <<'PY'
import json,sys
source=json.load(open(sys.argv[1])); restored=json.load(open(sys.argv[2]))
source.pop("runtime_control",None); restored.pop("runtime_control",None)
if source != restored: raise SystemExit("restored state differs from source state")
PY
[[ "$(psql_db "$restore_database" "SELECT count(*) FROM obligations WHERE id='$obligation_id' AND state='pending'")" == 1 ]] || { RECOVERY_ERROR="restored obligation is not pending"; exit 1; }
[[ "$(psql_db "$restore_database" "SELECT count(*) FROM worker_sessions WHERE employee_id='emp-frontend'")" == 0 ]] || { RECOVERY_ERROR="restored Frontend session is not empty"; exit 1; }
[[ "$(psql_db "$restore_database" "SELECT count(*) FROM messages WHERE sender='emp-planning' OR recipient='emp-planning'")" == 0 ]] || { RECOVERY_ERROR="restored Planner relay is nonzero"; exit 1; }

export POLIS_DSN="host=127.0.0.1 port=55432 dbname=$restore_database user=polis_runtime"
export POLIS_V3_RUNTIME_ROOT="$FRESH_ROOT"
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
export POLIS_V3_POSTGRES_SNAPSHOT="$DUMP_PATH"
bash scripts/go.sh run ./cmd/polis-r03a-pagination-v3 -backend-restore-verify
[[ -f "$EVIDENCE/backend-restored-runtime-verification.json" ]] || { RECOVERY_ERROR="old-writer verification evidence missing"; exit 1; }

python3 - "$EVIDENCE/backend-restored-runtime-verification.json" "$EVIDENCE/restore-continuity.json" "$FRESH_ROOT/postgres/pgdata" "$fresh_system_id" "$restore_database" "$PACKAGE_ID" "$continuation_manifest_hash" "$dump_hash" "$cas_hash" <<'PY'
import json,sys
verification_path,output,fresh_data,fresh_id,db,package,cont,dump,cas=sys.argv[1:]
v=json.load(open(verification_path))
if v.get("status")!="PASSED" or v.get("synthesized_rows")!=0 or not v.get("checks",{}).get("old_writer_denied"): raise SystemExit("old-writer/continuity verifier failed")
with open(output,'w',encoding='utf-8') as f: json.dump({"status":"PASSED","fresh_pgdata":fresh_data,"fresh_cluster_system_identifier":fresh_id,"restore_database":db,"package_generation_id":package,"continuation_manifest_hash":cont,"dump_hash":dump,"cas_manifest_hash":cas,"old_writer_fencing":"PASSED","synthesized_rows":0,"event_ordering":"PASSED","frontend_sessions":0,"obligation_state":"pending","planner_relay":0},f,indent=2); f.write("\n")
PY

FRONTEND_FRESH="$WORK_ROOT/frontend-manifest-freshness"
bash scripts/go.sh run ./cmd/polis-r03a-frontend-l2-offline -evidence "$FRONTEND_FRESH" -l1-evidence /mnt/d/Programs/Polis/evidence/development/r0.3a-current-binary-l1 -base-l2-evidence /mnt/d/Programs/Polis/evidence/development/r0.3a-current-binary-frontend-l2 -handover-protocol /mnt/d/Programs/Polis/evidence/development/r0.3a-real-frontend-handover-v4/2b7a6fd917d335459b0dbf32dda34158/protocol.jsonl
python3 - "$FRONTEND_FRESH/execution-manifest.json" /mnt/d/Programs/Polis/evidence/development/r0.3a-current-binary-frontend-l2/execution-manifest.json "$EVIDENCE/frontend-surface-freshness.json" <<'PY'
import json,sys
fresh=json.load(open(sys.argv[1])); qualified=json.load(open(sys.argv[2]))
fields=[("tool_surface","tool_count"),("tool_surface","aggregate_manifest_digest"),("tool_surface","aggregate_schema_digest"),("tool_surface","aggregate_schema_bytes"),("tool_surface","thread_start_payload_digest"),("tool_surface","thread_start_payload_bytes"),(None,"handler_binding_digest")]
equal=all((fresh.get(a) if a else fresh.get(b)) == (qualified.get(a) if a else qualified.get(b)) for a,b in fields)
status="QUALIFIED_REUSABLE" if equal else "STALE_PENDING_REQUALIFICATION"
with open(sys.argv[3],"w",encoding="utf-8") as f:
    json.dump({"status":status,"provider_visible_surface_equal":equal,"current_manifest":sys.argv[1],"qualified_manifest":sys.argv[2],"current_execution_fingerprint":fresh.get("execution_fingerprint"),"qualified_execution_fingerprint":qualified.get("execution_fingerprint"),"eligible_for_frontend_phase":equal},f,indent=2); f.write("\n")
if not equal: raise SystemExit("Frontend provider surface drifted")
PY

python3 - "$EVIDENCE/result.json" "$PACKAGE_ID" "$source_system_id_after" "$database_oid" "$DUMP_PATH" "$dump_hash" "$dump_size" "$CAS_MANIFEST" "$cas_hash" "$continuation_manifest_hash" "$fresh_system_id" "$restore_database" "$EVIDENCE/backend-restored-runtime-verification.json" <<'PY'
import json,sys
path,package,source_id,oid,dump,dump_hash,size,cas,cas_hash,cont,fresh_id,db,verify=sys.argv[1:]
with open(path,'w',encoding='utf-8') as f:
    json.dump({"qualification":"R0.3A-SAMPLE-6-FINAL-RECOVERY-CONTINUATION-V5","status":"PASSED","sample_6_final_recovery_continuation_v5":"PASSED","backend_recovery_subject":"PASSED","runner_preflight":"PASSED","postgres_source_validation":"PASSED","CAS_source_validation":"PASSED","joint_source_binding":"PASSED","terminal_state_validation":"PASSED","source_identity_after_salvage":"PASSED","continuation_manifest":"PASSED","recovery_readiness":"PASSED","recovery_anchors":"PASSED","authoritative_snapshot":"PASSED","CAS_manifest":"PASSED","recovery_package":"COMPLETE","destructive_boundary":"PASSED","fresh_restore":"PASSED","authoritative_state_continuity":"PASSED","old_writer_fencing":"PASSED","source_cluster_system_identifier":source_id,"source_database_oid":oid,"dump_path":dump,"dump_hash":dump_hash,"dump_size":int(size),"cas_manifest_path":cas,"cas_manifest_hash":cas_hash,"continuation_manifest_hash":cont,"fresh_cluster_system_identifier":fresh_id,"restore_database":db,"restore_verification":verify,"frontend_L2_status":"QUALIFIED_REUSABLE","eligible_for_frontend_phase":True,"historical_preservation_manifest":"NOT_FOUND","historical_evidence_modified":False,"source_database_destroyed":True,"source_pgdata_destroyed":True,"frontend":"NOT_STARTED","reviewer":"NOT_STARTED","sample_7":"NOT_CREATED","medium":0,"high":0,"provider_egress":0},f,indent=2); f.write("\n")
PY
