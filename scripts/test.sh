#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
export LD_LIBRARY_PATH="$PWD/.tools/pg/usr/lib/x86_64-linux-gnu"
pg="$PWD/.tools/pg/usr/lib/postgresql/18/bin"
socket="$PWD/.runtime/linux/pg-socket"
export TMPDIR="$PWD/.runtime/linux/test-tmp"
mkdir -p "$TMPDIR" bin evidence/development
database="polis_r0_test_$(date +%s)_$$"
admin="host=$socket port=55432 dbname=postgres user=$(id -un)"
"$pg/psql" "$admin" -v ON_ERROR_STOP=1 -c "SELECT 'CREATE ROLE polis_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE' WHERE NOT EXISTS (SELECT FROM pg_roles WHERE rolname='polis_runtime')" -tA | "$pg/psql" "$admin" -v ON_ERROR_STOP=1
"$pg/createdb" -h "$socket" -p 55432 "$database"
# This trap targets only the unique database created by this invocation.
trap '"$pg/dropdb" -h "$socket" -p 55432 "$database"' EXIT
export POLIS_DSN="host=$socket port=55432 dbname=$database user=$(id -un)"
bash scripts/go.sh run ./cmd/polis migrate
"$pg/psql" "$POLIS_DSN" -v ON_ERROR_STOP=1 -c 'GRANT USAGE ON SCHEMA public TO polis_runtime; GRANT SELECT,INSERT,UPDATE ON ALL TABLES IN SCHEMA public TO polis_runtime;'
export POLIS_TEST_DSN="host=$socket port=55432 dbname=$database user=polis_runtime"
test_flags=()
log=go-tests.txt
if [ "${POLIS_TEST_RACE:-0}" = 1 ]; then test_flags=(-race); log=go-tests-race.txt; fi
bash scripts/go.sh test "${test_flags[@]}" -count=1 -v ./... 2>&1 | tee "evidence/development/$log"
bash scripts/go.sh vet ./...
bash scripts/go.sh build -o bin/polis ./cmd/polis
bash scripts/go.sh build -o bin/polisd ./cmd/polisd
