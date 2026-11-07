#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
src="/home/chyinan/.local/state/polis-recovery/r03a-sample-6/postgres/pgdata"
clone="/home/chyinan/.local/state/polis-recovery/r03a-sample-6-salvage/clone"
socket="$clone/socket"
evidence="/mnt/d/Programs/Polis/evidence/development/r0.3a-sample-6-physical-state-binding-salvage-v1"
pg="/mnt/d/Programs/Polis/.tools/pg/usr/lib/postgresql/18/bin"
export LD_LIBRARY_PATH="/mnt/d/Programs/Polis/.tools/pg/usr/lib/x86_64-linux-gnu"

if [[ ! -f "$src/PG_VERSION" || ! -f "$src/global/pg_control" ]]; then
  echo "source PGDATA physical identity missing" >&2
  exit 1
fi
if [[ -e "$clone" ]]; then
  echo "forensic clone is not fresh" >&2
  exit 1
fi

mkdir -m 700 -p "$(dirname "$clone")" "$evidence"
cp -a "$src" "$clone"
mkdir -m 700 "$socket"

source_control_hash=$(sha256sum "$src/global/pg_control" | awk '{print $1}')
clone_control_hash=$(sha256sum "$clone/global/pg_control" | awk '{print $1}')
source_system_id=$("$pg/pg_controldata" "$src" | awk -F: '/Database system identifier/{gsub(/ /,"",$2);print $2}')
clone_system_id=$("$pg/pg_controldata" "$clone" | awk -F: '/Database system identifier/{gsub(/ /,"",$2);print $2}')
file_manifest_hash=$(tar --sort=name --mtime='UTC 1970-01-01' --owner=0 --group=0 --numeric-owner -cf - -C "$clone" . | sha256sum | awk '{print $1}')

"$pg/pg_ctl" -D "$clone" -l "$clone/server.log" -o "-k $socket -h '' -p 55435 -c listen_addresses='' -c fsync=on -c synchronous_commit=on -c jit=off" -w start >/dev/null
cleanup() { "$pg/pg_ctl" -D "$clone" -m fast -w stop >/dev/null 2>&1 || true; }
trap cleanup EXIT
psql() { "$pg/psql" -X -v ON_ERROR_STOP=1 -h "$socket" -p 55435 -d "$1" -Atc "$2"; }

db=$(psql postgres "SELECT datname FROM pg_database WHERE datname LIKE 'polis_r0_3a_pagination_v3_backend_%' ORDER BY datname DESC LIMIT 1")
if [[ -z "$db" ]]; then
  echo "sample-6 source database not found" >&2
  exit 1
fi
db_exists=$(psql postgres "SELECT count(*) FROM pg_database WHERE datname='$db'")
db_oid=$(psql postgres "SELECT oid::text FROM pg_database WHERE datname='$db'")
db_owner=$(psql postgres "SELECT pg_get_userbyid(datdba) FROM pg_database WHERE datname='$db'")
role_present=$(psql postgres "SELECT count(*) FROM pg_roles WHERE rolname='polis_runtime'")
schema_version=$(psql "$db" "SELECT COALESCE(max(version_id),0)::text FROM goose_db_version WHERE is_applied")
contract_match=$(psql "$db" "SELECT count(*) FROM contract_revisions WHERE id='92566d902cd328c0b694ef32188dabf5' AND state='accepted'")
message_match=$(psql "$db" "SELECT count(*) FROM messages WHERE id='45a77c6833518bd8fef1ddfe8d28b985' AND contract_revision_id='92566d902cd328c0b694ef32188dabf5'")
obligation_match=$(psql "$db" "SELECT count(*) FROM obligations WHERE id='45a77c6833518bd8fef1ddfe8d28b985' AND state='pending'")
signal_match=$(psql "$db" "SELECT count(*) FROM peer_work_signals WHERE message_id='45a77c6833518bd8fef1ddfe8d28b985' AND obligation_id='45a77c6833518bd8fef1ddfe8d28b985' AND state='pending'")
checkpoint_match=$(psql "$db" "SELECT count(*) FROM worker_checkpoints WHERE id='13164a801bac0760050c5f456bfd3f82' AND data->>'kind'='qualified' AND data->>'workspace_revision'='3'")
artifact_match=$(psql "$db" "SELECT count(*) FROM artifacts WHERE id='f73981340227622e9c16dbf45e961a3f' AND digest='e69a3f216201711384f4988698d7c33867e52f2426d93add6c05dda4beb12672' AND state='ready' AND verdict='candidate'")
workspace_digest=$(psql "$db" "SELECT digest FROM worker_workspaces WHERE revision=3 AND task_id IN (SELECT id FROM tasks WHERE owner='emp-backend') ORDER BY task_id LIMIT 1")
workspace_match=$(psql "$db" "SELECT count(*) FROM worker_workspaces WHERE revision=3 AND task_id IN (SELECT id FROM tasks WHERE owner='emp-backend')")
frontend_sessions=$(psql "$db" "SELECT count(*) FROM worker_sessions WHERE employee_id='emp-frontend'")
frontend_task_state=$(psql "$db" "SELECT COALESCE(string_agg(state,',' ORDER BY id),'') FROM tasks WHERE owner='emp-frontend'")
planner_relay=$(psql "$db" "SELECT count(*) FROM messages WHERE sender='emp-planning' OR recipient='emp-planning'")
postgres_version=$(tr -d '\r\n' < "$clone/PG_VERSION")
company_id=$(psql "$db" "SELECT id FROM companies ORDER BY id LIMIT 1")
mission_id=$(psql "$db" "SELECT mission_id FROM tasks WHERE owner='emp-backend' ORDER BY id LIMIT 1")
backend_task_id=$(psql "$db" "SELECT id FROM tasks WHERE owner='emp-backend' ORDER BY id LIMIT 1")
frontend_task_id=$(psql "$db" "SELECT id FROM tasks WHERE owner='emp-frontend' ORDER BY id LIMIT 1")
pgdata_digest=$(printf '%s|%s' "$postgres_version" "$source_control_hash" | sha256sum | awk '{print $1}')
role_grant_spec_digest=$(printf '%s' 'role-provisioning@1|role=polis_runtime|public:USAGE|ALL TABLES:SELECT,INSERT,UPDATE' | sha256sum | awk '{print $1}')
migration_schema_digest=$(python3 - <<'PY'
import hashlib
from pathlib import Path
root = Path('/mnt/d/Programs/Polis/db/migrations')
parts = []
for path in sorted(root.glob('*.sql')):
    parts.append(path.name + ':' + hashlib.sha256(path.read_bytes()).hexdigest())
print(hashlib.sha256('|'.join(parts).encode()).hexdigest())
PY
)

business_exact=false
if [[ "$db_exists" == 1 && "$contract_match" == 1 && "$message_match" == 1 && "$obligation_match" == 1 && "$signal_match" == 1 && "$checkpoint_match" == 1 && "$artifact_match" == 1 && "$workspace_match" == 1 && "$frontend_sessions" == 0 && "$planner_relay" == 0 ]]; then
  business_exact=true
fi

if [[ "$business_exact" != true || "$source_system_id" != "$clone_system_id" || "$source_control_hash" != "$clone_control_hash" ]]; then
  python3 - "$evidence/result.json" "$db" "$source_system_id" "$clone_system_id" "$business_exact" <<'PY'
import json
import sys

result = {
    "qualification": "R0.3A-SAMPLE-6-PHYSICAL-STATE-BINDING-SALVAGE",
    "status": "FAILED",
    "physical_cluster_bound_to_sample6": False,
    "source_state_identity_reestablished": False,
    "failure": {
        "database": sys.argv[2],
        "source_system_identifier": sys.argv[3],
        "clone_system_identifier": sys.argv[4],
        "business_anchor_match": sys.argv[5] == "true",
    },
    "sample6_source_mutated": False,
    "provider_egress": 0,
}
with open(sys.argv[1], "w", encoding="utf-8") as f:
    json.dump(result, f, indent=2)
    f.write("\n")
PY
  exit 1
fi

manifest_path="$evidence/PreservedPostgresAccessManifest.salvage.json"
manifest_hash_path="$manifest_path.sha256"
export DB="$db" DB_EXISTS="$db_exists" DB_OID="$db_oid" DB_OWNER="$db_owner" ROLE_PRESENT="$role_present" SCHEMA_VERSION="$schema_version" CONTRACT_MATCH="$contract_match" MESSAGE_MATCH="$message_match" OBLIGATION_MATCH="$obligation_match" SIGNAL_MATCH="$signal_match" CHECKPOINT_MATCH="$checkpoint_match" ARTIFACT_MATCH="$artifact_match" WORKSPACE_MATCH="$workspace_match" WORKSPACE_DIGEST="$workspace_digest" FRONTEND_SESSIONS="$frontend_sessions" FRONTEND_TASK_STATE="$frontend_task_state" PLANNER_RELAY="$planner_relay" BUSINESS_EXACT="$business_exact" SOURCE_SYSTEM_ID="$source_system_id" CLONE_SYSTEM_ID="$clone_system_id" SOURCE_CONTROL_HASH="$source_control_hash" CLONE_CONTROL_HASH="$clone_control_hash" FILE_MANIFEST_HASH="$file_manifest_hash" POSTGRES_VERSION="$postgres_version" PGDATA_DIGEST="$pgdata_digest" COMPANY_ID="$company_id" MISSION_ID="$mission_id" BACKEND_TASK_ID="$backend_task_id" FRONTEND_TASK_ID="$frontend_task_id" ROLE_GRANT_SPEC_DIGEST="$role_grant_spec_digest" MIGRATION_SCHEMA_DIGEST="$migration_schema_digest" MANIFEST_PATH="$manifest_path"
python3 - "$manifest_path" <<'PY'
import json
import os
import sys
from datetime import datetime, timezone

v = os.environ
manifest = {
    "manifest_revision": "r0.3a-preserved-postgres-access@1",
    "postgres_version": v["POSTGRES_VERSION"],
    "pgdata_canonical_path": "/home/chyinan/.local/state/polis-recovery/r03a-sample-6/postgres/pgdata",
    "pgdata_digest": v["PGDATA_DIGEST"],
    "cluster_system_identifier": v["SOURCE_SYSTEM_ID"],
    "database_name": v["DB"],
    "database_oid": v["DB_OID"],
    "database_owner": v["DB_OWNER"],
    "runtime_role_name": "polis_runtime",
    "role_provisioning_revision": "role-provisioning@1",
    "role_grant_spec_digest": v["ROLE_GRANT_SPEC_DIGEST"],
    "migration_schema_digest": v["MIGRATION_SCHEMA_DIGEST"],
    "company_id": v["COMPANY_ID"],
    "mission_id": v["MISSION_ID"],
    "backend_task_id": v["BACKEND_TASK_ID"],
    "frontend_task_id": v["FRONTEND_TASK_ID"],
    "created_at": datetime.now(timezone.utc).isoformat(),
    "preservation_reason": "R0.3A sample-6 post-failure physical salvage; existing database only",
    "credentials_included": False,
    "manifest_creation_phase": "post_failure_salvage",
    "historical_manifest_present": False,
    "source": "physical_cluster_inspection",
    "business_state_source": "existing_database_only",
    "forensic_clone_path": "/home/chyinan/.local/state/polis-recovery/r03a-sample-6-salvage/clone",
    "forensic_clone_file_manifest_sha256": v["FILE_MANIFEST_HASH"],
    "source_pg_control_sha256": v["SOURCE_CONTROL_HASH"],
    "clone_pg_control_sha256": v["CLONE_CONTROL_HASH"],
}
with open(sys.argv[1], "w", encoding="utf-8") as f:
    json.dump(manifest, f, indent=2)
    f.write("\n")
PY
sha256sum "$manifest_path" | awk '{print $1}' > "$manifest_hash_path"
manifest_hash=$(tr -d '\r\n' < "$manifest_hash_path")
export MANIFEST_PATH MANIFEST_HASH="$manifest_hash"
python3 - "$evidence/result.json" <<'PY'
import json
import os
import sys

v = os.environ
physical_match = v["SOURCE_SYSTEM_ID"] == v["CLONE_SYSTEM_ID"] and v["SOURCE_CONTROL_HASH"] == v["CLONE_CONTROL_HASH"]
business_match = v["BUSINESS_EXACT"] == "true"
result = {
    "qualification": "R0.3A-SAMPLE-6-PHYSICAL-STATE-BINDING-SALVAGE",
    "status": "PASSED" if physical_match and business_match else "FAILED",
    "physical_cluster_bound_to_sample6": physical_match and business_match,
    "source_state_identity_reestablished": physical_match and business_match,
    "source": {"pgdata": "/home/chyinan/.local/state/polis-recovery/r03a-sample-6/postgres/pgdata", "system_identifier": v["SOURCE_SYSTEM_ID"], "pg_control_sha256": v["SOURCE_CONTROL_HASH"]},
    "clone": {"pgdata": "/home/chyinan/.local/state/polis-recovery/r03a-sample-6-salvage/clone", "system_identifier": v["CLONE_SYSTEM_ID"], "pg_control_sha256": v["CLONE_CONTROL_HASH"], "file_manifest_sha256": v["FILE_MANIFEST_HASH"], "method": "cp -a from shut-down source PGDATA", "network": "unix_socket_only"},
    "database": {"name": v["DB"], "exists": v["DB_EXISTS"] == "1", "oid": v["DB_OID"], "owner": v["DB_OWNER"], "schema_version": int(v["SCHEMA_VERSION"])},
    "runtime_role": {"name": "polis_runtime", "present": v["ROLE_PRESENT"] == "1"},
    "anchors": {"contract_revision": v["CONTRACT_MATCH"] == "1", "message": v["MESSAGE_MATCH"] == "1", "obligation_pending": v["OBLIGATION_MATCH"] == "1", "work_signal": v["SIGNAL_MATCH"] == "1", "checkpoint": v["CHECKPOINT_MATCH"] == "1", "artifact": v["ARTIFACT_MATCH"] == "1", "workspace": v["WORKSPACE_MATCH"] == "1", "workspace_digest": v["WORKSPACE_DIGEST"], "frontend_sessions": int(v["FRONTEND_SESSIONS"]), "frontend_task_state": v["FRONTEND_TASK_STATE"], "planner_relay": int(v["PLANNER_RELAY"])},
    "salvage_manifest": {"path": v["MANIFEST_PATH"], "sha256": v["MANIFEST_HASH"], "historical_manifest_present": False, "creation_phase": "post_failure_salvage"},
    "salvage_manifest_status": "PASSED",
    "eligible_for_sample_6_recovery_continuation": "YES",
    "sample6_source_mutated": False,
    "provider_egress": 0,
}
with open(sys.argv[1], "w", encoding="utf-8") as f:
    json.dump(result, f, indent=2)
    f.write("\n")
if result["status"] != "PASSED":
    raise SystemExit(1)
PY
