#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/../../../../../" && pwd)"
evidence="$repo/evidence/development/r0.5b-real-provider-product-smoke/live-2/run"
export LD_LIBRARY_PATH="$repo/.tools/pg/usr/lib/x86_64-linux-gnu"
pg="$repo/.tools/pg/usr/lib/postgresql/18/bin"
read -r dsn < "$evidence/runtime-test-dsn.txt"
read -r mission < "$evidence/mission-id.txt"
mission="${mission%$'\r'}"
"$pg/psql" "$dsn" -X -tA -v ON_ERROR_STOP=1 -v mission="$mission" -f "$evidence/draft-counts.sql" > "$evidence/draft-counts.txt"
expected='1|4|1|draft|0|0'
if [[ "$(<"$evidence/draft-counts.txt")" != "$expected" ]]; then
  echo "LIVE_2 draft state did not match the single fresh Mission pre-Start boundary" >&2
  exit 1
fi
"$pg/psql" "$dsn" -X -tA -v ON_ERROR_STOP=1 -v company='r05b-live-2' -v mission="$mission" -f "$evidence/state-snapshot.sql" > "$evidence/mission-draft-snapshot.json"
