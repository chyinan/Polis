#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

export LD_LIBRARY_PATH="$PWD/.tools/pg/usr/lib/x86_64-linux-gnu"
pg="$PWD/.tools/pg/usr/lib/postgresql/18/bin"
b4_evidence_root="$PWD/evidence/development/r0.5b4-product-tool-surface-v2-live-qualification"
evidence="${POLIS_R05B4_OFFLINE_EVIDENCE_DIR:-$b4_evidence_root/offline}"
case "$evidence" in
  "$b4_evidence_root"/*) ;;
  *) echo "offline evidence output must remain inside the dedicated R0.5B4 directory" >&2; exit 1 ;;
esac
if [[ -e "$evidence" ]]; then
  echo "R0.5B4 offline evidence directory already exists; refusing to overwrite it" >&2
  exit 1
fi
mkdir -p "$evidence"

pgroot="$(mktemp -d "/tmp/polis-r05b4-pg.XXXXXX")"
case "$pgroot" in
  /tmp/polis-r05b4-pg.*) ;;
  *) echo "temporary PostgreSQL directory is outside the intended boundary" >&2; exit 1 ;;
esac
pgdata="$pgroot/data"
socketdir="$pgroot/socket"
port="${POLIS_R05B4_PG_PORT:-55449}"
database="polis_r0_r05b4_$$"
pg_started=0

cleanup() {
  if [[ "$pg_started" == 1 ]]; then "$pg/pg_ctl" -D "$pgdata" -m fast -w stop >/dev/null 2>&1 || true; fi
  case "$pgroot" in
    /tmp/polis-r05b4-pg.*) rm -rf -- "$pgroot" ;;
    *) echo "refusing to remove unexpected PostgreSQL path: $pgroot" >&2 ;;
  esac
}
trap cleanup EXIT

mkdir -m 700 -p "$socketdir"
"$pg/initdb" -D "$pgdata" -L "$PWD/.tools/pg/usr/share/postgresql/18" --locale=C --encoding=UTF8 --auth-local=trust --auth-host=trust >"$evidence/initdb.txt" 2>&1
"$pg/pg_ctl" -D "$pgdata" -l "$pgroot/postgres.log" -o "-k $socketdir -h 127.0.0.1 -p $port -c fsync=on -c synchronous_commit=on -c jit=off" -w start >"$evidence/postgres-start.txt" 2>&1
pg_started=1

admin_dsn="host=$socketdir port=$port dbname=postgres user=$(id -un)"
"$pg/psql" "$admin_dsn" -v ON_ERROR_STOP=1 -c 'CREATE ROLE polis_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE' >"$evidence/role.txt" 2>&1
"$pg/createdb" -h "$socketdir" -p "$port" "$database"
admin_db_dsn="host=$socketdir port=$port dbname=$database user=$(id -un)"
runtime_dsn="host=$socketdir port=$port dbname=$database user=polis_runtime"
POLIS_DSN="$admin_db_dsn" bash scripts/go.sh run ./cmd/polis migrate >"$evidence/migrations.txt" 2>&1
"$pg/psql" "$admin_db_dsn" -v ON_ERROR_STOP=1 -c 'GRANT USAGE ON SCHEMA public TO polis_runtime; GRANT SELECT,INSERT,UPDATE ON ALL TABLES IN SCHEMA public TO polis_runtime;' >"$evidence/runtime-grants.txt" 2>&1

POLIS_GO_ROOT="$PWD/.tools/go" bash scripts/go.sh test -count=1 ./internal/provider ./internal/codex >"$evidence/provider-codex-tests.txt" 2>&1
POLIS_TEST_DSN="$runtime_dsn" POLIS_R05B1_TEST_DSN="$runtime_dsn" POLIS_R05B3_TEST_DSN="$runtime_dsn" POLIS_GO_ROOT="$PWD/.tools/go" bash scripts/go.sh test -count=1 ./internal/taskvalidation >"$evidence/taskvalidation-tests.txt" 2>&1
POLIS_TEST_DSN="$runtime_dsn" POLIS_R05B1_TEST_DSN="$runtime_dsn" POLIS_R05B3_TEST_DSN="$runtime_dsn" bash scripts/go.sh test -count=1 ./internal/control >"$evidence/control-tests.txt" 2>&1
POLIS_TEST_DSN="$runtime_dsn" POLIS_R05B1_TEST_DSN="$runtime_dsn" POLIS_R05B3_TEST_DSN="$runtime_dsn" POLIS_GO_ROOT="$PWD/.tools/go" bash scripts/go.sh test -count=1 ./internal/kernel >"$evidence/kernel-tests.txt" 2>&1
POLIS_TEST_DSN="$runtime_dsn" POLIS_GO_ROOT="$PWD/.tools/go" bash scripts/go.sh test -count=1 ./internal/workbench >"$evidence/workbench-tests.txt" 2>&1

printf '%s\n' "postgres_database=$database" "postgres_temp_root=$pgroot" "database_prefix=polis_r0_" "provider_traffic=0" "live_business_mutations=0" "provider_registry=PASSED" "dispatcher_authorization=PASSED" "taskvalidation_integration=PASSED" "candidate_fencing_integration=PASSED" "real_provider_worker_adapter_compatibility=PASSED" >"$evidence/result.txt"
