#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
action="${1:-}"
recovery_root="${POLIS_RECOVERY_ROOT_WSL:-}"
if [[ -n "$recovery_root" ]]; then
  data="$recovery_root/postgres/pgdata"
  log="$recovery_root/postgres/server.log"
  mkdir -p "$(dirname "$data")"
  umask 077
else
  data="/tmp/polis-r0_3a_real_backend_pg"
  log="/tmp/polis-r0_3a_real_backend_pg.log"
fi
pg="$PWD/.tools/pg/usr/lib/postgresql/18/bin"
port="55432"
listen_host="${POLIS_PG_LISTEN_HOST:-127.0.0.1}"
if [[ -n "${POLIS_PG_SOCKET_ROOT:-}" ]]; then
  socket="${POLIS_PG_SOCKET_ROOT}"
else
  socket="/tmp/polis-pg-$(printf '%s' "$data" | sha256sum | cut -c1-12)"
fi
socket_readiness="$log.socket-readiness.json"

case "$action" in
  start)
    mkdir -m 700 -p "$socket"
    bash scripts/go.sh run ./cmd/polis-r03a-postgres-socket-validate -directory "$socket" -port "$port" -output "$socket_readiness"
    if [[ ! -f "$data/PG_VERSION" ]]; then
      LD_LIBRARY_PATH="$PWD/.tools/pg/usr/lib/x86_64-linux-gnu" "$pg/initdb" -D "$data" -L "$PWD/.tools/pg/usr/share/postgresql/18" --locale=C --encoding=UTF8 --auth-local=trust --auth-host=trust
    fi
    LD_LIBRARY_PATH="$PWD/.tools/pg/usr/lib/x86_64-linux-gnu" "$pg/pg_ctl" -D "$data" -l "$log" -o "-k $socket -h $listen_host -p $port -c fsync=on -c synchronous_commit=on -c jit=off" -w start
    ;;
  create)
    database="${2:?database name required}"
    LD_LIBRARY_PATH="$PWD/.tools/pg/usr/lib/x86_64-linux-gnu" "$pg/createdb" -h "$socket" -p "$port" "$database"
    LD_LIBRARY_PATH="$PWD/.tools/pg/usr/lib/x86_64-linux-gnu" "$pg/psql" -h "$socket" -p "$port" -d postgres -v ON_ERROR_STOP=1 -c "DO \$\$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'polis_runtime') THEN CREATE ROLE polis_runtime LOGIN; END IF; END \$\$;"
    ;;
  grant)
    database="${2:?database name required}"
    LD_LIBRARY_PATH="$PWD/.tools/pg/usr/lib/x86_64-linux-gnu" "$pg/psql" -h "$socket" -p "$port" -d "$database" -v ON_ERROR_STOP=1 -c 'GRANT USAGE ON SCHEMA public TO polis_runtime; GRANT SELECT,INSERT,UPDATE ON ALL TABLES IN SCHEMA public TO polis_runtime;'
    ;;
  migrate)
    database="${2:?database name required}"
    owner="${3:-$(id -un)}"
    POLIS_DSN="host=$socket port=$port dbname=$database user=$owner" bash scripts/go.sh run ./cmd/polis migrate
    LD_LIBRARY_PATH="$PWD/.tools/pg/usr/lib/x86_64-linux-gnu" "$pg/psql" -h "$socket" -p "$port" -d "$database" -v ON_ERROR_STOP=1 -c 'GRANT USAGE ON SCHEMA public TO polis_runtime; GRANT SELECT,INSERT,UPDATE ON ALL TABLES IN SCHEMA public TO polis_runtime;'
    ;;
  drop)
    database="${2:?database name required}"
    LD_LIBRARY_PATH="$PWD/.tools/pg/usr/lib/x86_64-linux-gnu" "$pg/dropdb" -h "$socket" -p "$port" "$database"
    ;;
  stop)
    if [[ -f "$data/PG_VERSION" ]]; then
      LD_LIBRARY_PATH="$PWD/.tools/pg/usr/lib/x86_64-linux-gnu" "$pg/pg_ctl" -D "$data" -m fast -w stop
    fi
    rmdir -- "$socket" 2>/dev/null || true
    ;;
  *)
    echo "usage: $0 start|create DB|migrate DB [OWNER]|grant DB|drop DB|stop" >&2
    exit 2
    ;;
esac
