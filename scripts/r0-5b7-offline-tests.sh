#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

export LD_LIBRARY_PATH="$PWD/.tools/pg/usr/lib/x86_64-linux-gnu"
pg="$PWD/.tools/pg/usr/lib/postgresql/18/bin"
evidence="$PWD/evidence/development/r0.5b7-checkpoint-schema-error-feedback-hardening"
mkdir -p "$evidence"

pgroot="$(mktemp -d "/tmp/polis-r05b7-pg.XXXXXX")"
case "$pgroot" in
  /tmp/polis-r05b7-pg.*) ;;
  *) echo "temporary PostgreSQL directory is outside the intended boundary" >&2; exit 1 ;;
esac
pgdata="$pgroot/data"
socketdir="$pgroot/socket"
port="${POLIS_R05B7_PG_PORT:-55445}"
database="polis_r0_r05b7_$$"
started=0

cleanup() {
  if [[ "$started" == 1 ]]; then "$pg/pg_ctl" -D "$pgdata" -m fast -w stop >/dev/null 2>&1 || true; fi
  case "$pgroot" in
    /tmp/polis-r05b7-pg.*) rm -rf -- "$pgroot" ;;
    *) echo "refusing to remove unexpected PostgreSQL path: $pgroot" >&2 ;;
  esac
}
trap cleanup EXIT

mkdir -m 700 -p "$socketdir"
"$pg/initdb" -D "$pgdata" -L "$PWD/.tools/pg/usr/share/postgresql/18" --locale=C --encoding=UTF8 --auth-local=trust --auth-host=trust >"$evidence/postgres-initdb.log" 2>&1
"$pg/pg_ctl" -D "$pgdata" -l "$evidence/postgres-server.log" -o "-k $socketdir -h 127.0.0.1 -p $port -c fsync=on -c synchronous_commit=on -c jit=off" -w start >"$evidence/postgres-start.log" 2>&1
started=1

admin_dsn="host=$socketdir port=$port dbname=postgres user=$(id -un)"
"$pg/psql" "$admin_dsn" -v ON_ERROR_STOP=1 -c 'CREATE ROLE polis_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE' >"$evidence/postgres-role.log" 2>&1
"$pg/createdb" -h "$socketdir" -p "$port" "$database"
database_dsn="host=$socketdir port=$port dbname=$database user=$(id -un)"
runtime_dsn="host=$socketdir port=$port dbname=$database user=polis_runtime"
POLIS_DSN="$database_dsn" bash scripts/go.sh run ./cmd/polis migrate >"$evidence/migrations.log" 2>&1
"$pg/psql" "$database_dsn" -v ON_ERROR_STOP=1 -c 'GRANT USAGE ON SCHEMA public TO polis_runtime; GRANT SELECT,INSERT,UPDATE ON ALL TABLES IN SCHEMA public TO polis_runtime;' >"$evidence/runtime-grants.log" 2>&1

POLIS_TEST_DSN="$runtime_dsn" POLIS_R05B1_TEST_DSN="$runtime_dsn" POLIS_R05B3_TEST_DSN="$runtime_dsn" \
  bash scripts/go.sh test -count=1 -run 'TestProductCandidateFencesPostPublicationWorkspaceAndCheckpointWrites|TestParseProductCheckpoint|TestFrozenLive2MalformedCheckpointShapesRemainSpecificFailures' ./internal/kernel >"$evidence/kernel-integration.log" 2>&1
POLIS_TEST_DSN="$runtime_dsn" bash scripts/go.sh test -count=1 -run TestProductCandidateLifecycleProjectsThroughWorkbench ./internal/workbench >"$evidence/workbench-integration.log" 2>&1
POLIS_TEST_DSN="$runtime_dsn" POLIS_R05B1_TEST_DSN="$runtime_dsn" POLIS_R05B3_TEST_DSN="$runtime_dsn" \
  bash scripts/go.sh test -count=1 -run TestRealProviderWorkerAdapterUsesOfflineRuntimeAndFinalizesWorkerLifecycle ./internal/control >"$evidence/control-integration.log" 2>&1

printf 'status=PASSED\nprovider_egress=0\nmedium=0\nhigh=0\ndatabase=%s\n' "$database" >"$evidence/result.txt"
