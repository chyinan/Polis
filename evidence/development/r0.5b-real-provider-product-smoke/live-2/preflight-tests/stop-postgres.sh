#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/../../../../../" && pwd)"
evidence="$repo/evidence/development/r0.5b-real-provider-product-smoke/live-2/preflight-tests"
read -r recorded_root < "$evidence/postgres-temp-root.txt"
case "$recorded_root" in
  /tmp/polis-r05b-live2-auth-preflight.*) ;;
  *) echo "refusing unexpected preflight-test PostgreSQL root: $recorded_root" >&2; exit 1 ;;
esac
pgroot="$(realpath -e -- "$recorded_root")"
if [[ "$pgroot" != "$recorded_root" || "$pgroot" == /tmp || "$pgroot" == / ]]; then
  echo "refusing unresolved or broad PostgreSQL cleanup target: $pgroot" >&2
  exit 1
fi
pgdata="$(realpath -e -- "$pgroot/data")"
if [[ "$pgdata" != "$pgroot/data" ]]; then
  echo "refusing unexpected PostgreSQL data directory: $pgdata" >&2
  exit 1
fi
pg="$repo/.tools/pg/usr/lib/postgresql/18/bin"
export LD_LIBRARY_PATH="$repo/.tools/pg/usr/lib/x86_64-linux-gnu"
"$pg/pg_ctl" -D "$pgdata" status
"$pg/pg_ctl" -D "$pgdata" -m fast -w stop
if "$pg/pg_ctl" -D "$pgdata" status >/dev/null 2>&1; then
  echo "preflight-test PostgreSQL is still running after stop" >&2
  exit 1
fi
rm -rf -- "$pgroot"
if [[ -e "$pgroot" ]]; then
  echo "preflight-test PostgreSQL root still exists: $pgroot" >&2
  exit 1
fi
printf 'stopped_and_removed=%s\n' "$pgroot" > "$evidence/postgres-cleanup.txt"
