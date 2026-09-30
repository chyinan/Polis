#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/../../.." && pwd)"
evidence="$repo/evidence/development/r1-r3-implementation-validation-20260925-slice-25"
export LD_LIBRARY_PATH="$repo/.tools/pg/usr/lib/x86_64-linux-gnu"
pg="$repo/.tools/pg/usr/lib/postgresql/18/bin"
port=56442
database=polis_r0_r1_git_s25

if [[ -e "$evidence/postgres-temp-root.txt" || -e "$evidence/runtime-test-dsn.txt" ]]; then
  echo 'Slice 25 PostgreSQL evidence already exists; refusing reuse' >&2
  exit 1
fi
if "$pg/pg_isready" -h 127.0.0.1 -p "$port" >/dev/null 2>&1; then
  echo "Slice 25 PostgreSQL port $port is occupied" >&2
  exit 1
fi

pgroot="$(mktemp -d /tmp/polis-r1-s25.XXXXXX)"
pgdata="$pgroot/data"
socket="$pgroot/socket"
mkdir -m 700 -p "$socket"
printf '%s\n' "$pgroot" > "$evidence/postgres-temp-root.txt"
"$pg/initdb" -D "$pgdata" -L "$repo/.tools/pg/usr/share/postgresql/18" --locale=C --encoding=UTF8 --auth-local=trust --auth-host=trust > "$evidence/postgres-initdb.log" 2>&1
"$pg/pg_ctl" -D "$pgdata" -l "$evidence/postgres-server.log" -o "-k $socket -h 127.0.0.1 -p $port -c fsync=on -c synchronous_commit=on -c jit=off" -w start > "$evidence/postgres-start.log" 2>&1
admin_dsn="host=$socket port=$port dbname=postgres user=$(id -un)"
"$pg/psql" "$admin_dsn" -v ON_ERROR_STOP=1 -c 'CREATE ROLE polis_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE' > "$evidence/postgres-role.log" 2>&1
"$pg/createdb" -h "$socket" -p "$port" "$database"
database_admin_dsn="host=$socket port=$port dbname=$database user=$(id -un)"
POLIS_DSN="$database_admin_dsn" bash "$repo/scripts/go.sh" run ./cmd/polis migrate > "$evidence/migrations.log" 2>&1
"$pg/psql" "$database_admin_dsn" -v ON_ERROR_STOP=1 -c 'GRANT USAGE ON SCHEMA public TO polis_runtime; GRANT SELECT,INSERT,UPDATE ON ALL TABLES IN SCHEMA public TO polis_runtime; GRANT USAGE,SELECT,UPDATE ON ALL SEQUENCES IN SCHEMA public TO polis_runtime' > "$evidence/runtime-grants.log" 2>&1
printf 'host=%s port=%s dbname=%s user=polis_runtime\n' "$socket" "$port" "$database" > "$evidence/runtime-test-dsn.txt"
"$pg/psql" "$database_admin_dsn" -X -tA -v ON_ERROR_STOP=1 -c "SELECT current_setting('server_version_num'),(SELECT version_id FROM goose_db_version WHERE is_applied ORDER BY id DESC LIMIT 1)" > "$evidence/postgres-ready.txt"
echo 'SLICE_25_POSTGRES_READY'
