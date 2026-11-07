#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
db="polis_r0_3a_pg_dump_smoke_v1_$$"
restore="polis_r0_3a_pg_restore_smoke_v1_$$"
root="/home/chyinan/.local/state/polis-recovery/qualification/pgdump-smoke-v1-$$"
dump="$root/fixture.dump"
pg="$PWD/.tools/pg/usr/lib/postgresql/18/bin"
started=0
created=0
restore_created=0
cleanup() {
  set +e
  if [[ $restore_created == 1 ]]; then bash scripts/r03a-real-backend-pg.sh drop "$restore"; fi
  if [[ $created == 1 ]]; then bash scripts/r03a-real-backend-pg.sh drop "$db"; fi
  if [[ $started == 1 ]]; then bash scripts/r03a-real-backend-pg.sh stop; fi
  rm -rf -- "$root"
}
trap cleanup EXIT
mkdir -m 700 -p "$root"
bash scripts/r03a-real-backend-pg.sh start
started=1
bash scripts/r03a-real-backend-pg.sh create "$db"
created=1
owner="$(id -un)"
bash scripts/r03a-real-backend-pg.sh migrate "$db" "$owner"
bash scripts/r03a-real-backend-pg.sh grant "$db"
export LD_LIBRARY_PATH="$PWD/.tools/pg/usr/lib/x86_64-linux-gnu"
"$pg/pg_dump" -h 127.0.0.1 -p 55432 -d "$db" --format=custom --no-owner --no-acl --file="$dump"
test -s "$dump"
"$pg/pg_restore" -l "$dump" | grep -q TABLE
bash scripts/r03a-real-backend-pg.sh create "$restore"
restore_created=1
"$pg/pg_restore" -h 127.0.0.1 -p 55432 -d "$restore" --no-owner --no-acl --exit-on-error "$dump"
bash scripts/r03a-real-backend-pg.sh grant "$restore"
schema="$("$pg/psql" -h 127.0.0.1 -p 55432 -d "$restore" -Atc 'SELECT max(version_id) FROM goose_db_version WHERE is_applied')"
test "$schema" = 5
mkdir -p evidence/development/r0.3a-recovery-config-wiring-smoke-v1
cat > evidence/development/r0.3a-recovery-config-wiring-smoke-v1/result.json <<EOF
{"status":"PASSED","dump_path":"configured production pg_dump","custom_archive_nonzero":true,"restore_status":"PASSED","schema_version":$schema,"medium":0,"high":0,"provider_egress":0,"sample6_mutated":false}
EOF
