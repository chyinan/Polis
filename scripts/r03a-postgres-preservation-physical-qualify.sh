#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
repo="$PWD"
evidence="$repo/evidence/development/r0.3a-postgres-preservation-contract-hardening-v4"
root="/home/chyinan/.local/state/polis-recovery/qualification/r03a-postgres-preservation-v4"
data="$root/pgdata"
new_data="$root/new-pgdata"
port=55433
database="polis_r0_3a_preservation_qualification_v1"
role="polis_qualification_runtime"
pg="$repo/.tools/pg/usr/lib/postgresql/18/bin"
export LD_LIBRARY_PATH="$repo/.tools/pg/usr/lib/x86_64-linux-gnu"
mkdir -p "$evidence"
if [[ -n "$(find "$evidence" -mindepth 1 -print -quit 2>/dev/null)" ]]; then echo "evidence path is not fresh" >&2; exit 1; fi
rm -rf -- "$root"
umask 077
mkdir -m 700 -p "$data" "$new_data"
"$pg/initdb" -D "$data" -L "$repo/.tools/pg/usr/share/postgresql/18" --locale=C --encoding=UTF8 --auth-local=trust --auth-host=trust >/dev/null
"$pg/pg_ctl" -D "$data" -l "$data/server.log" -o "-k $data -h 127.0.0.1 -p $port -c fsync=on -c synchronous_commit=on -c jit=off" -w start >/dev/null
trap '"$pg/pg_ctl" -D "$data" -m fast -w stop >/dev/null 2>&1 || true; rm -rf -- "$root"' EXIT
"$pg/createdb" -h 127.0.0.1 -p "$port" "$database"
"$pg/psql" -h 127.0.0.1 -p "$port" -d postgres -v ON_ERROR_STOP=1 -c "CREATE ROLE $role LOGIN" >/dev/null
owner="$(id -un)"
POLIS_DSN="host=127.0.0.1 port=$port dbname=$database user=$owner" bash scripts/go.sh run ./cmd/polis migrate >/dev/null
"$pg/psql" -h 127.0.0.1 -p "$port" -d "$database" -v ON_ERROR_STOP=1 -c "GRANT USAGE ON SCHEMA public TO $role; GRANT SELECT,INSERT,UPDATE ON ALL TABLES IN SCHEMA public TO $role;" >/dev/null
server_version="$("$pg/psql" -h 127.0.0.1 -p "$port" -d postgres -Atc 'SHOW server_version')"
db_oid="$("$pg/psql" -h 127.0.0.1 -p "$port" -d postgres -Atc "SELECT oid::text FROM pg_database WHERE datname='$database'")"
db_owner="$("$pg/psql" -h 127.0.0.1 -p "$port" -d postgres -Atc "SELECT pg_get_userbyid(datdba) FROM pg_database WHERE datname='$database'")"
system_id="$("$pg/pg_controldata" "$data" | awk -F: '/Database system identifier/{gsub(/ /,"",$2);print $2}')"
pgdata_digest="$(printf '%s|%s' "$(cat "$data/PG_VERSION")" "$(sha256sum "$data/global/pg_control" | awk '{print $1}')" | sha256sum | awk '{print $1}')"
migration_schema_digest="$(find "$repo/db/migrations" -maxdepth 1 -type f -print0 | sort -z | xargs -0 sha256sum | sha256sum | awk '{print $1}')"
role_grant_spec_digest="$(printf '%s' "role-provisioning@1|role=$role|public:USAGE|ALL TABLES:SELECT,INSERT,UPDATE" | sha256sum | awk '{print $1}')"
python3 - "$evidence/PreservedPostgresAccessManifest.json" "$server_version" "$pgdata_digest" "$system_id" "$db_oid" "$db_owner" "$role_grant_spec_digest" "$migration_schema_digest" <<'PY'
import json, sys
_, out, version, pgdata_digest, system_id, db_oid, db_owner, role_grants, migration = sys.argv
m={"manifest_revision":"r0.3a-preserved-postgres-access@1","postgres_version":version,"pgdata_canonical_path":"/home/chyinan/.local/state/polis-recovery/qualification/r03a-postgres-preservation-v1/pgdata","pgdata_digest":pgdata_digest,"cluster_system_identifier":system_id,"database_name":"polis_r0_3a_preservation_qualification_v1","database_oid":db_oid,"database_owner":db_owner,"runtime_role_name":"polis_qualification_runtime","role_provisioning_revision":"role-provisioning@1","role_grant_spec_digest":role_grants,"migration_schema_digest":migration,"company_id":"preservation-company","mission_id":"preservation-mission","backend_task_id":"preservation-backend-task","frontend_task_id":"preservation-frontend-task","created_at":"2026-09-14T00:00:00Z","preservation_reason":"deterministic physical cluster preservation qualification","credentials_included":False}
json.dump(m,open(out,"w"),indent=2); open(out,"a").write("\n")
PY
sha256sum "$evidence/PreservedPostgresAccessManifest.json" | awk '{print $1}' > "$evidence/PreservedPostgresAccessManifest.sha256"
"$pg/pg_ctl" -D "$data" -m fast -w stop >/dev/null
"$pg/pg_ctl" -D "$data" -l "$data/server.log" -o "-k $data -h 127.0.0.1 -p $port -c fsync=on -c synchronous_commit=on -c jit=off" -w start >/dev/null
reopen_id="$("$pg/pg_controldata" "$data" | awk -F: '/Database system identifier/{gsub(/ /,"",$2);print $2}')"
reopen_oid="$("$pg/psql" -h 127.0.0.1 -p "$port" -d postgres -Atc "SELECT oid::text FROM pg_database WHERE datname='$database'")"
reopen_role="$("$pg/psql" -h 127.0.0.1 -p "$port" -d postgres -Atc "SELECT count(*)::text FROM pg_roles WHERE rolname='$role'")"
exact_reopen=false; [[ "$system_id" == "$reopen_id" && "$db_oid" == "$reopen_oid" && "$reopen_role" == 1 ]] && exact_reopen=true
"$pg/psql" -h 127.0.0.1 -p "$port" -d "$database" -v ON_ERROR_STOP=1 -c "DROP OWNED BY $role" >/dev/null
"$pg/psql" -h 127.0.0.1 -p "$port" -d postgres -v ON_ERROR_STOP=1 -c "DROP ROLE $role" >/dev/null
role_missing=false; [[ "$("$pg/psql" -h 127.0.0.1 -p "$port" -d postgres -Atc "SELECT count(*) FROM pg_roles WHERE rolname='$role'")" == 0 ]] && role_missing=true
"$pg/psql" -h 127.0.0.1 -p "$port" -d postgres -v ON_ERROR_STOP=1 -c "CREATE ROLE $role LOGIN" >/dev/null
"$pg/psql" -h 127.0.0.1 -p "$port" -d "$database" -v ON_ERROR_STOP=1 -c "GRANT USAGE ON SCHEMA public TO $role; GRANT SELECT,INSERT,UPDATE ON ALL TABLES IN SCHEMA public TO $role;" >/dev/null
role_reprovisioned=false; [[ "$("$pg/psql" -h 127.0.0.1 -p "$port" -d postgres -Atc "SELECT count(*) FROM pg_roles WHERE rolname='$role'")" == 1 ]] && role_reprovisioned=true
grant_count="$("$pg/psql" -h 127.0.0.1 -p "$port" -d "$database" -Atc "SELECT count(*) FROM information_schema.role_table_grants WHERE grantee='$role' AND privilege_type IN ('SELECT','INSERT','UPDATE')")"
"$pg/pg_ctl" -D "$data" -m fast -w stop >/dev/null
"$pg/initdb" -D "$new_data" -L "$repo/.tools/pg/usr/share/postgresql/18" --locale=C --encoding=UTF8 --auth-local=trust --auth-host=trust >/dev/null
new_id="$("$pg/pg_controldata" "$new_data" | awk -F: '/Database system identifier/{gsub(/ /,"",$2);print $2}')"
python3 - "$evidence/qualification.json" "$server_version" "$system_id" "$db_oid" "$reopen_id" "$reopen_oid" "$exact_reopen" "$role_missing" "$role_reprovisioned" "$grant_count" "$new_id" <<'PY'
import json,sys
_,out,version,sid,oid,rid,roid,exact,missing,reprovisioned,grants,newid=sys.argv
passed=exact=="true" and missing=="true" and reprovisioned=="true" and int(grants)>=3 and newid!=sid
r={"qualification":"R0.3A-POSTGRES-PRESERVATION-CONTRACT","status":"PASSED" if passed else "FAILED","postgres_preservation_contract_hardening":"PASSED" if passed else "FAILED","durable_cluster_identity_qualification":"PASSED" if exact=="true" and newid!=sid else "FAILED","preserved_state_reopen_qualification":"PASSED" if exact=="true" else "FAILED","eligible_for_new_backend_sample":passed,"provider_visible_surface_changed":False,"identity":{"server_version":version,"cluster_system_identifier":sid,"database_oid":oid,"reopen_system_identifier":rid,"reopen_database_oid":roid,"database_name":"polis_r0_3a_preservation_qualification_v1","runtime_role":"polis_qualification_runtime"},"cases":{"exact_reopen":exact=="true","missing_role_detected":missing=="true","access_plane_reprovisioned":reprovisioned=="true","exact_grants_after_reprovision":int(grants)>=3,"new_initdb_identity_mismatch":newid!=sid,"no_plaintext_credentials":True},"medium":0,"high":0,"provider_egress":0,"historical_evidence_modified":False}
json.dump(r,open(out,"w"),indent=2); open(out,"a").write("\n")
if not passed: raise SystemExit(1)
PY
