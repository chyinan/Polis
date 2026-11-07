#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/../../../../" && pwd)"
cd "$repo"
evidence="$repo/evidence/development/r0.5b-real-provider-product-smoke/live-1"
export LD_LIBRARY_PATH="$repo/.tools/pg/usr/lib/x86_64-linux-gnu"
pg="$repo/.tools/pg/usr/lib/postgresql/18/bin"

pgroot="$(mktemp -d /tmp/polis-r05b-live-1-pg.XXXXXX)"
pgdata="$pgroot/data"
socket="$pgroot/socket"
mkdir -m 700 -p "$socket"
printf '%s\n' "$pgroot" > "$evidence/postgres-temp-root.txt"

"$pg/initdb" -D "$pgdata" -L "$repo/.tools/pg/usr/share/postgresql/18" --locale=C --encoding=UTF8 --auth-local=trust --auth-host=trust > "$evidence/postgres-initdb.txt" 2>&1
wsl_ip="$(hostname -I | awk '{print $1}')"
gateway="$(ip route show default | awk '$1 == "default" {print $3; exit}')"
cat >> "$pgdata/postgresql.conf" <<EOF
listen_addresses = '$wsl_ip'
port = 55481
unix_socket_directories = '$socket'
fsync = on
synchronous_commit = on
jit = off
EOF
cat > "$pgdata/pg_hba.conf" <<EOF
local all all trust
host all all 127.0.0.1/32 trust
host all all $gateway/32 trust
EOF

"$pg/pg_ctl" -D "$pgdata" -l "$evidence/postgres-server.log" -o "-k $socket -h $wsl_ip -p 55481" -w start > "$evidence/postgres-start.txt" 2>&1
admin_dsn="host=$socket port=55481 dbname=postgres user=$(id -un)"
"$pg/psql" "$admin_dsn" -v ON_ERROR_STOP=1 -c 'CREATE ROLE polis_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE' > "$evidence/postgres-role.txt" 2>&1
"$pg/createdb" -h "$socket" -p 55481 polis_r0_5b_live_1
database_admin_dsn="host=$socket port=55481 dbname=polis_r0_5b_live_1 user=$(id -un)"
POLIS_DSN="$database_admin_dsn" bash scripts/go.sh run ./cmd/polis migrate > "$evidence/migrations.txt" 2>&1
"$pg/psql" "$database_admin_dsn" -v ON_ERROR_STOP=1 -c 'GRANT USAGE ON SCHEMA public TO polis_runtime; GRANT SELECT,INSERT,UPDATE ON ALL TABLES IN SCHEMA public TO polis_runtime;' > "$evidence/postgres-runtime-grants.txt" 2>&1
"$pg/psql" "$database_admin_dsn" -X -tA -v ON_ERROR_STOP=1 -c "SELECT current_setting('server_version_num'), current_setting('port'), current_setting('fsync'), current_setting('synchronous_commit'), (SELECT version_id FROM goose_db_version ORDER BY id DESC LIMIT 1)" > "$evidence/postgres-ready.txt"
{
  printf 'wsl_ip=%s\n' "$wsl_ip"
  printf 'gateway=%s\n' "$gateway"
  printf 'port=55481\n'
  printf 'database=polis_r0_5b_live_1\n'
  printf 'schema_version=7\n'
} >> "$evidence/postgres-ready.txt"
