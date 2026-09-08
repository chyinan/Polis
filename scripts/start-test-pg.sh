#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
export LD_LIBRARY_PATH="$PWD/.tools/pg/usr/lib/x86_64-linux-gnu"
pg="$PWD/.tools/pg/usr/lib/postgresql/18/bin"
data="$PWD/.runtime/linux/pg-data"
socket="$PWD/.runtime/linux/pg-socket"
mkdir -p "$socket"
chmod 700 "$socket"
if [ ! -f "$data/PG_VERSION" ]; then
  "$pg/initdb" -D "$data" -L "$PWD/.tools/pg/usr/share/postgresql/18" --locale=C --encoding=UTF8 --auth-local=trust --auth-host=reject
fi
"$pg/pg_ctl" -D "$data" -l "$PWD/.runtime/postgres.log" -o "-k $socket -p 55432 -c listen_addresses='' -c fsync=on -c synchronous_commit=on -c jit=off" -w start
