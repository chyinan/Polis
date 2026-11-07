#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
evidence="$repo/evidence/development/r0.5b-real-provider-product-smoke-live-3"
export LD_LIBRARY_PATH="$repo/.tools/pg/usr/lib/x86_64-linux-gnu"
pg="$repo/.tools/pg/usr/lib/postgresql/18/bin"
port=55484
database=polis_r0_5b_live3
company=r05b-live-3

if [[ ! -d "$evidence" ]]; then
  echo "LIVE_3 evidence directory is missing" >&2
  exit 1
fi
if [[ -e "$evidence/postgres-temp-root.txt" || -e "$evidence/windows-runtime-dsn.txt" ]]; then
  echo "LIVE_3 PostgreSQL setup evidence already exists; refusing reuse" >&2
  exit 1
fi
if "$pg/pg_isready" -h 127.0.0.1 -p "$port" >/dev/null 2>&1; then
  echo "LIVE_3 PostgreSQL port $port is occupied" >&2
  exit 1
fi

pgroot="$(mktemp -d /tmp/polis-r05b-live3.XXXXXX)"
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
"$pg/psql" "$database_admin_dsn" -v ON_ERROR_STOP=1 -c 'GRANT USAGE ON SCHEMA public TO polis_runtime; GRANT SELECT,INSERT,UPDATE ON ALL TABLES IN SCHEMA public TO polis_runtime;' > "$evidence/runtime-grants.log" 2>&1
"$pg/psql" "$database_admin_dsn" -v ON_ERROR_STOP=1 -f "$repo/scripts/r0-5b-live3-company-bootstrap.sql" > "$evidence/company-roster-bootstrap.log" 2>&1

printf 'host=127.0.0.1 port=%s dbname=%s user=polis_runtime\n' "$port" "$database" > "$evidence/windows-runtime-dsn.txt"
printf 'host=%s port=%s dbname=%s user=polis_runtime\n' "$socket" "$port" "$database" > "$evidence/runtime-test-dsn.txt"
"$pg/psql" "$database_admin_dsn" -X -tA -v ON_ERROR_STOP=1 -c "SELECT current_setting('server_version_num'),current_setting('fsync'),current_setting('synchronous_commit'),(SELECT version_id FROM goose_db_version WHERE is_applied ORDER BY id DESC LIMIT 1),(SELECT count(*) FROM companies WHERE id='$company'),(SELECT count(*) FROM employees WHERE company_id='$company')" > "$evidence/postgres-ready.txt"

echo "LIVE_3_POSTGRES_READY port=$port database=$database company=$company"
