#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
export LD_LIBRARY_PATH="$PWD/.tools/pg/usr/lib/x86_64-linux-gnu"
{
 uname -a
 cat /proc/self/cgroup
 .tools/go/bin/go version
 .tools/bin/sqlc version
 .tools/pg/usr/lib/postgresql/18/bin/postgres --version
 gcc --version
 .tools/pg/usr/lib/postgresql/18/bin/psql "host=$PWD/.runtime/linux/pg-socket port=55432 dbname=postgres user=$(id -un)" -X -c "SELECT version(), current_setting('fsync') AS fsync, current_setting('synchronous_commit') AS synchronous_commit, current_setting('listen_addresses') AS listen_addresses;"
 bash scripts/go.sh list -m all
} > evidence/development/native-versions.txt
