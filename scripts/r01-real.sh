#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
# A second invocation cannot reset the persisted experiment allowance.
label="${POLIS_R01_RUN_LABEL:-real}"
[[ "$label" =~ ^[a-zA-Z0-9_-]{1,40}$ ]] || exit 1
test ! -e "evidence/development/r0.1/$label/allowance.json" || { echo "R0.1 allowance already exists; automatic rerun refused"; exit 1; }
export LD_LIBRARY_PATH="$PWD/.tools/pg/usr/lib/x86_64-linux-gnu"
pg="$PWD/.tools/pg/usr/lib/postgresql/18/bin"
socket="$PWD/.runtime/linux/pg-socket"
database="polis_r0_probe_$(date +%s)_$$"
"$pg/createdb" -h "$socket" -p 55432 "$database"
export POLIS_DSN="host=$socket port=55432 dbname=$database user=$(id -un)"
bash scripts/go.sh run ./cmd/polis migrate
"$pg/psql" "$POLIS_DSN" -v ON_ERROR_STOP=1 -c 'GRANT USAGE ON SCHEMA public TO polis_runtime; GRANT SELECT,INSERT,UPDATE ON ALL TABLES IN SCHEMA public TO polis_runtime;'
export POLIS_DSN="host=$socket port=55432 dbname=$database user=polis_runtime"
export POLIS_CODEX_AUTH_FILE="/mnt/c/Users/chyinan/.codex/auth.json"
export POLIS_NATIVE_PROXY="http://127.0.0.1:$(cat .runtime/r01-proxy-port.txt)"
export POLIS_SCHEMA_DIGEST=6032863e8a892b00eb34433d5fefb057066a7b700cf5f6ce06b1321e382b7455
mkdir -p "evidence/development/r0.1/$label"
printf '%s\n' "$database" > "evidence/development/r0.1/$label/probe-database.txt"
# Retain the dedicated database for inspection even after failure; no restart,
# retry or second real experiment is triggered by this script.
bash scripts/go.sh run ./cmd/polis-probe --inspect --run-label "$label"
bash scripts/go.sh run ./cmd/polis-probe --real --run-label "$label" --medium-turns "${POLIS_R01_MEDIUM_LIMIT:-3}" --high-turns "${POLIS_R01_HIGH_LIMIT:-3}"
