#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
label="luna-1"
test ! -e "evidence/development/r0.2/$label/allowance.json" || { echo "R0.2 allowance already exists; refusing to reset or retry"; exit 1; }
export LD_LIBRARY_PATH="$PWD/.tools/pg/usr/lib/x86_64-linux-gnu"
pg="$PWD/.tools/pg/usr/lib/postgresql/18/bin"
socket="$PWD/.runtime/linux/pg-socket"
test -r /mnt/c/Users/chyinan/.codex/auth.json || { echo "authorized Codex auth file is unavailable"; exit 1; }
database="polis_r0_2_$(date +%s)_$$"
"$pg/createdb" -h "$socket" -p 55432 "$database"
trap '"$pg/dropdb" -h "$socket" -p 55432 "$database"' EXIT
export POLIS_DSN="host=$socket port=55432 dbname=$database user=$(id -un)"
bash scripts/go.sh run ./cmd/polis migrate
"$pg/psql" "$POLIS_DSN" -v ON_ERROR_STOP=1 -c 'GRANT USAGE ON SCHEMA public TO polis_runtime; GRANT SELECT,INSERT,UPDATE ON ALL TABLES IN SCHEMA public TO polis_runtime;'
export POLIS_DSN="host=$socket port=55432 dbname=$database user=polis_runtime"
export POLIS_CODEX_AUTH_FILE="/mnt/c/Users/chyinan/.codex/auth.json"
export POLIS_NATIVE_PROXY="http://127.0.0.1:$(cat .runtime/r01-proxy-port.txt)"
export POLIS_SCHEMA_DIGEST=6032863e8a892b00eb34433d5fefb057066a7b700cf5f6ce06b1321e382b7455
mkdir -p "evidence/development/r0.2/$label"
printf '%s\n' "$database" > "evidence/development/r0.2/$label/probe-database.txt"
bash scripts/go.sh run ./cmd/polis-r02 --inspect
bash scripts/go.sh run ./cmd/polis-r02
"$pg/psql" "$POLIS_DSN" -X -tA -v ON_ERROR_STOP=1 -c "SELECT json_build_object('sessions',(SELECT json_agg(row_to_json(s)) FROM worker_sessions s),'tasks',(SELECT json_agg(row_to_json(t)) FROM tasks t),'checkpoints',(SELECT count(*) FROM worker_checkpoints),'checks',(SELECT count(*) FROM worker_checks),'artifacts',(SELECT count(*) FROM artifacts),'tool_receipts',(SELECT count(*) FROM receipts WHERE key LIKE 'tool-%'));" > "evidence/development/r0.2/$label/db-snapshot.json"
