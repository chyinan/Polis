#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/../../../../../" && pwd)"
evidence="$repo/evidence/development/r0.5b-real-provider-product-smoke/live-2/run"
export LD_LIBRARY_PATH="$repo/.tools/pg/usr/lib/x86_64-linux-gnu"
pg="$repo/.tools/pg/usr/lib/postgresql/18/bin"
read -r dsn < "$evidence/runtime-test-dsn.txt"
"$pg/psql" "$dsn" -X -tA -v ON_ERROR_STOP=1 -c "SELECT count(*) FROM companies WHERE id='r05b-live-2'" > "$evidence/company-before-bootstrap.txt"
if [[ "$(<"$evidence/company-before-bootstrap.txt")" != "0" ]]; then
  echo "refusing to reuse or replace an existing LIVE_2 Company" >&2
  exit 1
fi
"$pg/psql" "$dsn" -X -v ON_ERROR_STOP=1 -f "$evidence/bootstrap-company.sql" > "$evidence/company-bootstrap.log" 2>&1
"$pg/psql" "$dsn" -X -tA -v ON_ERROR_STOP=1 -f "$evidence/preflight-state.sql" > "$evidence/preflight-state.json"
