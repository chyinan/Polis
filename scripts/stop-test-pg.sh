#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
export LD_LIBRARY_PATH="$PWD/.tools/pg/usr/lib/x86_64-linux-gnu"
exec .tools/pg/usr/lib/postgresql/18/bin/pg_ctl -D "$PWD/.runtime/linux/pg-data" -m fast -w stop
