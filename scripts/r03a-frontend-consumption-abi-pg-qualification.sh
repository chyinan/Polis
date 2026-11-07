#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
export POLIS_PG_SOCKET_ROOT="${POLIS_PG_SOCKET_ROOT:-/tmp/polis-pg-r03a-frontend-consumption-abi}"
export POLIS_PG_PORT="${POLIS_PG_PORT:-55432}"

cleanup() {
  bash scripts/r03a-real-backend-pg.sh stop >/dev/null 2>&1 || true
}
trap cleanup EXIT

bash scripts/r03a-real-backend-pg.sh start
bash scripts/test.sh
