#!/usr/bin/env bash
set -euo pipefail
repo="$(cd "$(dirname "$0")/.." && pwd)"
export LD_LIBRARY_PATH="$repo/.tools/pg/usr/lib/x86_64-linux-gnu"
pg="$repo/.tools/pg/usr/lib/postgresql/18/bin"
evidence="$repo/evidence/development/r0.5b15-business-path-provider-initialize-hardening/test-db-2"
pgroot="$(cat "$evidence/postgres-temp-root.txt")"
"$pg/pg_ctl" -D "$pgroot/data" -m fast -w stop > "$evidence/postgres-stop.log" 2>&1
echo 'R0.5B15_POSTGRES_STOPPED'
