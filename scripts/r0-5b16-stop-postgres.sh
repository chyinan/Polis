#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
evidence="$repo/evidence/development/r0.5b16-reservation-to-initialize-bridge-hardening/test-db"
export LD_LIBRARY_PATH="$repo/.tools/pg/usr/lib/x86_64-linux-gnu"
pg="$repo/.tools/pg/usr/lib/postgresql/18/bin"
if [[ ! -f "$evidence/postgres-temp-root.txt" ]]; then
  echo 'R0.5B16 PostgreSQL temp-root evidence is missing' >&2
  exit 1
fi
pgroot="$(tr -d '\r\n' < "$evidence/postgres-temp-root.txt")"
"$pg/pg_ctl" -D "$pgroot/data" -m fast -w stop > "$evidence/postgres-stop.log" 2>&1
echo 'R0.5B16_POSTGRES_STOPPED'
