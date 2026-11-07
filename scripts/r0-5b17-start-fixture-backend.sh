#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
evidence="$repo/evidence/development/r0.5b17-checkpoint-projection-hardening"
export LD_LIBRARY_PATH="$repo/.tools/pg/usr/lib/x86_64-linux-gnu"
pg="$repo/.tools/pg/usr/lib/postgresql/18/bin"
port=55497
backend_port=18092
database=polis_r0_5b17_browser
company=r0-5b17-browser
pgroot="$(mktemp -d /tmp/polis-r05b17.XXXXXX)"
pgdata="$pgroot/data"
socket="$pgroot/socket"
backend_pid=""

mkdir -m 700 -p "$socket" "$evidence"

cleanup() {
  if [[ -n "$backend_pid" ]]; then
    kill "$backend_pid" >/dev/null 2>&1 || true
    wait "$backend_pid" >/dev/null 2>&1 || true
  fi
  "$pg/pg_ctl" -D "$pgdata" -m fast -w stop >/dev/null 2>&1 || true
  rm -rf -- "$pgroot"
}
trap cleanup EXIT

"$pg/initdb" -D "$pgdata" -L "$repo/.tools/pg/usr/share/postgresql/18" --locale=C --encoding=UTF8 --auth-local=trust --auth-host=trust >"$evidence/postgres-initdb.log" 2>&1
"$pg/pg_ctl" -D "$pgdata" -l "$evidence/postgres-server.log" -o "-k $socket -h 127.0.0.1 -p $port -c fsync=on -c synchronous_commit=on -c jit=off" -w start >"$evidence/postgres-start.log" 2>&1
admin_dsn="host=$socket port=$port dbname=postgres user=$(id -un)"
"$pg/psql" "$admin_dsn" -v ON_ERROR_STOP=1 -c "SELECT 'CREATE ROLE polis_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE' WHERE NOT EXISTS (SELECT FROM pg_roles WHERE rolname='polis_runtime')" -tA | "$pg/psql" "$admin_dsn" -v ON_ERROR_STOP=1
"$pg/createdb" -h "$socket" -p "$port" "$database"
database_admin_dsn="host=$socket port=$port dbname=$database user=$(id -un)"
POLIS_DSN="$database_admin_dsn" bash "$repo/scripts/go.sh" run ./cmd/polis migrate >"$evidence/migrations.log" 2>&1
"$pg/psql" "$database_admin_dsn" -v ON_ERROR_STOP=1 -c 'GRANT USAGE ON SCHEMA public TO polis_runtime; GRANT SELECT,INSERT,UPDATE ON ALL TABLES IN SCHEMA public TO polis_runtime;'
runtime_dsn="host=127.0.0.1 port=$port dbname=$database user=polis_runtime"
printf '%s\n' "$runtime_dsn" >"$evidence/runtime-dsn.txt"
printf '%s\n' "$company" >"$evidence/company-id.txt"
mkdir -p "$pgroot/blobs"

POLIS_TEST_DSN="$runtime_dsn" POLIS_B17_COMPANY_ID="$company" POLIS_B17_BLOB_ROOT="$pgroot/blobs" bash "$repo/scripts/go.sh" test -count=1 -run TestPostgresReadStoreProjectsQualifiedCheckpointWithoutLifecycleFilters ./internal/workbench >"$evidence/postgres-read-model-test.log" 2>&1

POLIS_DSN="$runtime_dsn" POLIS_BLOB_ROOT="$pgroot/blobs" POLIS_WORKBENCH_ADDR="127.0.0.1:$backend_port" POLIS_WORKER_MODE=deterministic bash "$repo/scripts/go.sh" run ./cmd/polis serve >"$evidence/backend.log" 2>&1 &
backend_pid=$!
for attempt in $(seq 1 60); do
  if curl --fail --silent "http://127.0.0.1:$backend_port/api/workbench/companies/$company/overview" >"$evidence/backend-overview.json"; then
    echo "R0_5B17_FIXTURE_BACKEND_READY company=$company backend_port=$backend_port postgres_port=$port"
    wait "$backend_pid"
    exit $?
  fi
  sleep 1
done
echo "R0_5B17_FIXTURE_BACKEND_TIMEOUT" >&2
exit 1
