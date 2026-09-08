#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p .tools/pg
for archive in .tools/downloads/*.deb; do dpkg-deb -x "$archive" .tools/pg; done
export LD_LIBRARY_PATH="$PWD/.tools/pg/usr/lib/x86_64-linux-gnu"
ldd .tools/pg/usr/lib/postgresql/18/bin/postgres
.tools/pg/usr/lib/postgresql/18/bin/postgres --version
