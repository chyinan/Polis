#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
umask 077
pg="$PWD/.tools/pg/usr/lib/postgresql/18/bin"
export LD_LIBRARY_PATH="$PWD/.tools/pg/usr/lib/x86_64-linux-gnu"
root="/home/chyinan/.local/state/polis-recovery/qualification/r03a-postgres-socket-v1-$$"
data="$root/postgres/pgdata"
socket="/tmp/polis-pg-qual-$$"
port=55437
started=0
cleanup() {
  if [[ "$started" == 1 ]]; then "$pg/pg_ctl" -D "$data" -m fast -w stop >/dev/null 2>&1 || true; fi
  rmdir -- "$socket" 2>/dev/null || true
  rm -rf -- "$root"
}
trap cleanup EXIT

mkdir -m 700 -p "$root" "$socket"
bash scripts/go.sh run ./cmd/polis-r03a-postgres-socket-validate -directory "$socket" -port "$port" -output "$root/short-socket-readiness.json"
"$pg/initdb" -D "$data" -L "$PWD/.tools/pg/usr/share/postgresql/18" --locale=C --encoding=UTF8 --auth-local=trust --auth-host=reject >/dev/null
"$pg/pg_ctl" -D "$data" -l "$root/server.log" -o "-k $socket -h '' -p $port -c listen_addresses='' -c fsync=on -c synchronous_commit=on -c jit=off" -w start >/dev/null
started=1
[[ "$("$pg/psql" -X -h "$socket" -p "$port" -d postgres -Atc 'SELECT 1')" == 1 ]]

overdir="$root/$(printf 'x%.0s' $(seq 1 80))"
mkdir -m 700 -p "$overdir"
set +e
bash scripts/go.sh run ./cmd/polis-r03a-postgres-socket-validate -directory "$overdir" -port "$port" -output "$root/overlong-readiness.json" >/dev/null 2>&1
over_rc=$?
set -e
[[ "$over_rc" -ne 0 ]]
echo "status=PASSED short_socket=PASS real_postgres_start=PASS overlong_preflight=DENIED" 
