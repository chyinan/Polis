#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
export LD_LIBRARY_PATH="$PWD/.tools/pg/usr/lib/x86_64-linux-gnu"
pg="$PWD/.tools/pg/usr/lib/postgresql/18/bin"
socket="${POLIS_PG_SOCKET_ROOT:-$PWD/.runtime/linux/pg-socket}"
port="${POLIS_PG_PORT:-55432}"
export TMPDIR="$PWD/.runtime/linux/test-tmp"
export POLIS_GO_ROOT="$PWD/.tools/go"
export POLIS_CODEX_BINARY="$PWD/.tools/codex-linux/package/vendor/x86_64-unknown-linux-musl/bin/codex"
mkdir -p "$TMPDIR" bin evidence/development
database="polis_r0_test_$(date +%s)_$$"
admin="host=$socket port=$port dbname=postgres user=$(id -un)"
"$pg/psql" "$admin" -v ON_ERROR_STOP=1 -c "SELECT 'CREATE ROLE polis_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE' WHERE NOT EXISTS (SELECT FROM pg_roles WHERE rolname='polis_runtime')" -tA | "$pg/psql" "$admin" -v ON_ERROR_STOP=1
"$pg/createdb" -h "$socket" -p "$port" "$database"
# This trap targets only the unique database created by this invocation.
trap '"$pg/dropdb" -h "$socket" -p "$port" "$database"' EXIT
export POLIS_DSN="host=$socket port=$port dbname=$database user=$(id -un)"
bash scripts/go.sh run ./cmd/polis migrate
"$pg/psql" "$POLIS_DSN" -v ON_ERROR_STOP=1 -c 'GRANT USAGE ON SCHEMA public TO polis_runtime; GRANT SELECT,INSERT,UPDATE ON ALL TABLES IN SCHEMA public TO polis_runtime; GRANT USAGE,SELECT ON ALL SEQUENCES IN SCHEMA public TO polis_runtime;'
export POLIS_TEST_DSN="host=$socket port=$port dbname=$database user=polis_runtime"
test_flags=()
mkdir -p evidence/development/r0.1
log=r0.1/go-tests.txt
if [ "${POLIS_TEST_RACE:-0}" = 1 ]; then test_flags=(-race); log=r0.1/go-tests-race.txt; fi
bash scripts/go.sh test "${test_flags[@]}" -count=1 -v ./... 2>&1 | tee "evidence/development/$log"
bash scripts/go.sh vet ./...
bash scripts/go.sh build -o bin/polis ./cmd/polis
bash scripts/go.sh build -o bin/polisd ./cmd/polisd
bash scripts/go.sh build -o bin/polis-probe ./cmd/polis-probe
bash scripts/go.sh build -o bin/polis-r02 ./cmd/polis-r02
bash scripts/go.sh build -o bin/polis-r02-high ./cmd/polis-r02-high
