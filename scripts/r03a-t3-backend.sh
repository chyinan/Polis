#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
evidence="evidence/development/r0.3a-t3/luna-1"
test ! -e "$evidence/allowance.json" || { echo "R0.3A-T3 allowance already exists; refusing retry/reset"; exit 1; }
export LD_LIBRARY_PATH="$PWD/.tools/pg/usr/lib/x86_64-linux-gnu"
pg="$PWD/.tools/pg/usr/lib/postgresql/18/bin"
socket="$PWD/.runtime/linux/pg-socket"
test -r /mnt/c/Users/chyinan/.codex/auth.json || { echo "authorized Codex auth file is unavailable"; exit 1; }
database="polis_r0_3a_t3_$(date +%s)_$$"
"$pg/createdb" -h "$socket" -p 55432 "$database"
trap '"$pg/dropdb" -h "$socket" -p 55432 "$database"' EXIT
export POLIS_DSN="host=$socket port=55432 dbname=$database user=$(id -un)"
bash scripts/go.sh run ./cmd/polis migrate
"$pg/psql" "$POLIS_DSN" -v ON_ERROR_STOP=1 -c 'GRANT USAGE ON SCHEMA public TO polis_runtime; GRANT SELECT,INSERT,UPDATE ON ALL TABLES IN SCHEMA public TO polis_runtime;'
export POLIS_DSN="host=$socket port=55432 dbname=$database user=polis_runtime"
export POLIS_CODEX_AUTH_FILE="/mnt/c/Users/chyinan/.codex/auth.json"
export POLIS_NATIVE_PROXY="http://127.0.0.1:$(cat .runtime/r01-proxy-port.txt)"
mkdir -p "$evidence"
if test ! -e "$evidence/preflight.json"; then
  bash scripts/go.sh run ./cmd/polis-r03a-t3 --preflight
fi
bash scripts/go.sh run ./cmd/polis-r03a-t3
