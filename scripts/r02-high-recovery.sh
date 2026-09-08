#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
old="evidence/development/r0.2/luna-1"
test -e "$old/allowance.json" || { echo "original R0.2 allowance not found"; exit 1; }
export LD_LIBRARY_PATH="$PWD/.tools/pg/usr/lib/x86_64-linux-gnu"
pg="$PWD/.tools/pg/usr/lib/postgresql/18/bin"
socket="$PWD/.runtime/linux/pg-socket"
database="polis_r0_2_high_$(date +%s)_$$"
"$pg/createdb" -h "$socket" -p 55432 "$database"
trap '"$pg/dropdb" -h "$socket" -p 55432 "$database"' EXIT
export POLIS_DSN="host=$socket port=55432 dbname=$database user=$(id -un)"
bash scripts/go.sh run ./cmd/polis migrate
"$pg/psql" "$POLIS_DSN" -v ON_ERROR_STOP=1 -c 'GRANT USAGE ON SCHEMA public TO polis_runtime; GRANT SELECT,INSERT,UPDATE ON ALL TABLES IN SCHEMA public TO polis_runtime;'
export POLIS_DSN="host=$socket port=55432 dbname=$database user=polis_runtime"
export POLIS_CODEX_AUTH_FILE="/mnt/c/Users/chyinan/.codex/auth.json"
export POLIS_NATIVE_PROXY="http://127.0.0.1:$(cat .runtime/r01-proxy-port.txt)"
export POLIS_SCHEMA_DIGEST=6032863e8a892b00eb34433d5fefb057066a7b700cf5f6ce06b1321e382b7455
bash scripts/go.sh run ./cmd/polis-r02-high --old-evidence "$old"
