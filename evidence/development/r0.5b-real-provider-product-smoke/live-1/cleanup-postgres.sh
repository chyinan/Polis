#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/../../../../" && pwd)"
cd "$repo"
evidence="$repo/evidence/development/r0.5b-real-provider-product-smoke/live-1"
export LD_LIBRARY_PATH="$repo/.tools/pg/usr/lib/x86_64-linux-gnu"
pg="$repo/.tools/pg/usr/lib/postgresql/18/bin"
pgroot="$(<"$evidence/postgres-temp-root.txt")"
canonical="$(realpath "$pgroot")"
case "$pgroot" in
  /tmp/polis-r05b-live-1-pg.*) ;;
  *) echo "refusing to clean unexpected PostgreSQL path: $pgroot" >&2; exit 1 ;;
esac
if [[ "$canonical" != "$pgroot" ]]; then
  echo "refusing to clean PostgreSQL path whose canonical form changed: $canonical" >&2
  exit 1
fi
"$pg/pg_ctl" -D "$pgroot/data" -m fast -w stop > "$evidence/postgres-stop.txt" 2>&1
rm -rf -- "$pgroot"
if [[ -e "$pgroot" ]]; then
  echo 'temporary PostgreSQL cluster directory still exists' >&2
  exit 1
fi
printf 'postgres_stopped=true\ncluster_removed=true\nremoved_path=%s\n' "$pgroot" > "$evidence/postgres-cleanup.txt"
