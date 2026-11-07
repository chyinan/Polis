#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/../../../../" && pwd)"
cd "$repo"
evidence="$repo/evidence/development/r0.5b-real-provider-product-smoke/live-1"
export LD_LIBRARY_PATH="$repo/.tools/pg/usr/lib/x86_64-linux-gnu"
pg="$repo/.tools/pg/usr/lib/postgresql/18/bin"
pgroot="$(<"$evidence/postgres-temp-root.txt")"
pgdata="$pgroot/data"
socket="$pgroot/socket"
admin_dsn="host=$socket port=55481 dbname=polis_r0_5b_live_1 user=$(id -un)"

"$pg/pg_ctl" -D "$pgdata" status > "$evidence/postgres-status.txt" 2>&1
"$pg/psql" "$admin_dsn" -X -tA -v ON_ERROR_STOP=1 -c "SELECT current_setting('server_version_num'), current_setting('port'), current_setting('fsync'), current_setting('synchronous_commit'), (SELECT version_id FROM goose_db_version ORDER BY id DESC LIMIT 1), (SELECT count(*) FROM companies), (SELECT count(*) FROM missions), (SELECT count(*) FROM tasks), (SELECT count(*) FROM worker_sessions)" > "$evidence/postgres-ready.txt"
wsl_ip="$(hostname -I | awk '{print $1}')"
gateway="$(ip route show default | awk '$1 == "default" {print $3; exit}')"
{
  printf 'wsl_ip=%s\n' "$wsl_ip"
  printf 'gateway=%s\n' "$gateway"
  printf 'port=55481\n'
  printf 'database=polis_r0_5b_live_1\n'
} >> "$evidence/postgres-ready.txt"
