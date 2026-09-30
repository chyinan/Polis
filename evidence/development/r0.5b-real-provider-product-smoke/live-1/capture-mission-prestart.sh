#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/../../../../" && pwd)"
cd "$repo"
evidence="$repo/evidence/development/r0.5b-real-provider-product-smoke/live-1"
export LD_LIBRARY_PATH="$repo/.tools/pg/usr/lib/x86_64-linux-gnu"
pg="$repo/.tools/pg/usr/lib/postgresql/18/bin"
pgroot="$(<"$evidence/postgres-temp-root.txt")"
socket="$pgroot/socket"
dsn="host=$socket port=55481 dbname=polis_r0_5b_live_1 user=$(id -un)"
"$pg/psql" "$dsn" -X -tA -v ON_ERROR_STOP=1 -f "$evidence/mission-prestart.sql" > "$evidence/mission-prestart.json"
