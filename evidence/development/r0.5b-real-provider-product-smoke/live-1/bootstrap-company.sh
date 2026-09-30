#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/../../../../" && pwd)"
cd "$repo"
evidence="$repo/evidence/development/r0.5b-real-provider-product-smoke/live-1"
export LD_LIBRARY_PATH="$repo/.tools/pg/usr/lib/x86_64-linux-gnu"
pg="$repo/.tools/pg/usr/lib/postgresql/18/bin"
pgroot="$(<"$evidence/postgres-temp-root.txt")"
socket="$pgroot/socket"
admin_dsn="host=$socket port=55481 dbname=polis_r0_5b_live_1 user=$(id -un)"

"$pg/psql" "$admin_dsn" -v ON_ERROR_STOP=1 -f "$evidence/company-bootstrap.sql" > "$evidence/company-bootstrap.txt" 2>&1
"$pg/psql" "$admin_dsn" -X -tA -v ON_ERROR_STOP=1 -c "SELECT (SELECT count(*) FROM companies), (SELECT count(*) FROM employees), (SELECT count(*) FROM missions), (SELECT count(*) FROM tasks), (SELECT count(*) FROM worker_sessions)" > "$evidence/company-bootstrap-state.txt"
