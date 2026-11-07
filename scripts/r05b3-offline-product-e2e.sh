#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

export LD_LIBRARY_PATH="$PWD/.tools/pg/usr/lib/x86_64-linux-gnu"
pg="$PWD/.tools/pg/usr/lib/postgresql/18/bin"
evidence="$PWD/evidence/development/r0.5b3-product-employee-surface-semantic-remediation"
mkdir -p "$evidence"

pgroot="$(mktemp -d "/tmp/polis-r05b3-pg.XXXXXX")"
case "$pgroot" in
  /tmp/polis-r05b3-pg.*) ;;
  *) echo "temporary PostgreSQL directory is outside the intended boundary" >&2; exit 1 ;;
esac
pgdata="$pgroot/data"
socketdir="$pgroot/socket"
port="${POLIS_R05B3_PG_PORT:-55439}"
api_port="${POLIS_R05B3_API_PORT:-18081}"
frontend_port="${POLIS_R05B3_FRONTEND_PORT:-4176}"
database="polis_r0_r05b3_$$"
pg_started=0
api_pid=""
frontend_pid=""

cleanup() {
  if [[ -n "$frontend_pid" ]]; then kill -TERM "$frontend_pid" 2>/dev/null || true; wait "$frontend_pid" 2>/dev/null || true; fi
  if [[ -n "$api_pid" ]]; then kill -TERM "$api_pid" 2>/dev/null || true; wait "$api_pid" 2>/dev/null || true; fi
  if [[ "$pg_started" == 1 ]]; then "$pg/pg_ctl" -D "$pgdata" -m fast -w stop >/dev/null 2>&1 || true; fi
  case "$pgroot" in
    /tmp/polis-r05b3-pg.*) rm -rf -- "$pgroot" ;;
    *) echo "refusing to remove unexpected PostgreSQL path: $pgroot" >&2 ;;
  esac
}
trap cleanup EXIT

mkdir -m 700 -p "$socketdir"
"$pg/initdb" -D "$pgdata" -L "$PWD/.tools/pg/usr/share/postgresql/18" --locale=C --encoding=UTF8 --auth-local=trust --auth-host=trust >/dev/null
"$pg/pg_ctl" -D "$pgdata" -l "$pgroot/postgres.log" -o "-k $socketdir -h 127.0.0.1 -p $port -c fsync=on -c synchronous_commit=on -c jit=off" -w start >/dev/null
pg_started=1

admin_dsn="host=$socketdir port=$port dbname=postgres user=$(id -un)"
"$pg/psql" "$admin_dsn" -v ON_ERROR_STOP=1 -c 'CREATE ROLE polis_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE' >/dev/null
"$pg/createdb" -h "$socketdir" -p "$port" "$database"
admin_db_dsn="host=$socketdir port=$port dbname=$database user=$(id -un)"
runtime_dsn="host=$socketdir port=$port dbname=$database user=polis_runtime"
POLIS_DSN="$admin_db_dsn" bash scripts/go.sh run ./cmd/polis migrate
"$pg/psql" "$admin_db_dsn" -v ON_ERROR_STOP=1 -c 'GRANT USAGE ON SCHEMA public TO polis_runtime; GRANT SELECT,INSERT,UPDATE ON ALL TABLES IN SCHEMA public TO polis_runtime;' >/dev/null
"$pg/psql" "$admin_db_dsn" -v ON_ERROR_STOP=1 -f scripts/r05b3-offline-fixture.sql >/dev/null

if [[ "${R05B3_SKIP_PG_TESTS:-0}" != 1 ]]; then
  if [[ -n "${R05B3_TEST_FILTER:-}" ]]; then
    POLIS_TEST_DSN="$runtime_dsn" POLIS_R05B3_TEST_DSN="$runtime_dsn" POLIS_GO_ROOT="$PWD/.tools/go" \
      bash scripts/go.sh test -count=1 -run "$R05B3_TEST_FILTER" ./internal/kernel 2>&1 | tee "$evidence/postgres-focused-regression.txt"
  else
    POLIS_TEST_DSN="$runtime_dsn" \
    POLIS_R05B1_TEST_DSN="$runtime_dsn" \
    POLIS_R05B3_TEST_DSN="$runtime_dsn" \
    POLIS_GO_ROOT="$PWD/.tools/go" \
    bash scripts/go.sh test -count=1 ./internal/taskvalidation 2>&1 | tee "$evidence/postgres-taskvalidation.txt"
    POLIS_TEST_DSN="$runtime_dsn" POLIS_R05B1_TEST_DSN="$runtime_dsn" POLIS_R05B3_TEST_DSN="$runtime_dsn" bash scripts/go.sh test -count=1 ./internal/control 2>&1 | tee "$evidence/postgres-control-integration.txt"
    POLIS_TEST_DSN="$runtime_dsn" POLIS_R05B1_TEST_DSN="$runtime_dsn" POLIS_R05B3_TEST_DSN="$runtime_dsn" POLIS_GO_ROOT="$PWD/.tools/go" bash scripts/go.sh test -count=1 ./internal/kernel 2>&1 | tee "$evidence/postgres-kernel-integration.txt"
    POLIS_TEST_DSN="$runtime_dsn" bash scripts/go.sh test -count=1 ./internal/workbench 2>&1 | tee "$evidence/postgres-workbench-integration.txt"
  fi
fi

if [[ "${R05B3_TEST_ONLY:-0}" == 1 ]]; then exit 0; fi

api_binary="$pgroot/polis"
bash scripts/go.sh build -o "$api_binary" ./cmd/polis
POLIS_DSN="$runtime_dsn" \
POLIS_BLOB_ROOT="$pgroot/blobstore" \
POLIS_WORKER_MODE=real \
POLIS_PROVIDER_TRANSPORT=fake \
POLIS_PROVIDER_MODEL=offline-model \
POLIS_PROVIDER_EFFORT=medium \
POLIS_PROVIDER_PROFILE=offline-model/medium \
POLIS_WORKBENCH_ADDR="127.0.0.1:$api_port" \
"$api_binary" serve >"$pgroot/workbench-api.log" 2>&1 &
api_pid=$!

ready=0
for _ in $(seq 1 80); do
  if curl -fsS "http://127.0.0.1:$api_port/api/workbench/companies/r05b3-offline-company/overview" >/dev/null; then
    ready=1
    break
  fi
  sleep 0.25
done
if [[ "$ready" != 1 ]]; then
  echo "offline Polis API did not become ready" >&2
  tail -n 80 "$pgroot/workbench-api.log" >&2 || true
  exit 1
fi

echo "postgres_data=$pgdata"
echo "runtime_dsn=$runtime_dsn"
echo "api_url=http://127.0.0.1:$api_port"
echo "company_id=r05b3-offline-company"
echo "frontend_url=http://127.0.0.1:$frontend_port"
read -r -p "Press Enter after browser E2E and state capture to tear down this temporary PostgreSQL cluster: " _

"$pg/psql" "$admin_db_dsn" -X -A -t -v ON_ERROR_STOP=1 -f scripts/r05b3-offline-state-query.sql >"$evidence/authoritative-state.json"
python3 -m json.tool "$evidence/authoritative-state.json" >/dev/null
cat "$evidence/authoritative-state.json"
