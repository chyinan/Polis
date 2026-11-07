#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/../../../../../" && pwd)"
evidence="$repo/evidence/development/r0.5b-real-provider-product-smoke/live-2/run"
export LD_LIBRARY_PATH="$repo/.tools/pg/usr/lib/x86_64-linux-gnu"
pg="$repo/.tools/pg/usr/lib/postgresql/18/bin"
read -r dsn < "$evidence/runtime-test-dsn.txt"
"$pg/psql" "$dsn" -X -tA -v ON_ERROR_STOP=1 -f "$evidence/state-counts.sql" > "$evidence/current-counts.json"
"$pg/psql" "$dsn" -X -tA -v ON_ERROR_STOP=1 -f "$evidence/state-snapshot.sql" > "$evidence/current-state.json"
