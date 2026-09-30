#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/../../.." && pwd)"
evidence="$repo/evidence/development/r1-r3-implementation-validation-20260925-slice-25"
export LD_LIBRARY_PATH="$repo/.tools/pg/usr/lib/x86_64-linux-gnu"
pg="$repo/.tools/pg/usr/lib/postgresql/18/bin"
pgroot="$(cat "$evidence/postgres-temp-root.txt")"

case "$pgroot" in
  /tmp/polis-r1-s25.*) ;;
  *) echo 'Refusing to stop or remove a non-Slice-25 temporary root' >&2; exit 1 ;;
esac
if [[ -L "$pgroot" || ! -d "$pgroot/data" ]]; then
  echo 'Refusing to stop or remove an unexpected PostgreSQL root' >&2
  exit 1
fi
resolved_root="$(realpath "$pgroot")"
if [[ "$resolved_root" != "$pgroot" ]]; then
  echo 'Refusing to remove a PostgreSQL root that resolves elsewhere' >&2
  exit 1
fi
if "$pg/pg_ctl" -D "$pgroot/data" status >/dev/null 2>&1; then
  "$pg/pg_ctl" -D "$pgroot/data" -m fast -w stop > "$evidence/postgres-stop.log" 2>&1
fi
if "$pg/pg_ctl" -D "$pgroot/data" status >/dev/null 2>&1; then
  echo 'PostgreSQL remained running after stop' >&2
  exit 1
fi
rm -rf -- "$pgroot"
if [[ -e "$pgroot" ]]; then
  echo 'PostgreSQL temporary root remains after cleanup' >&2
  exit 1
fi
printf 'stopped and removed %s\n' "$resolved_root" > "$evidence/postgres-cleanup.txt"
echo 'SLICE_25_POSTGRES_STOPPED'
