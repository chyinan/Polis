#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd -P)"
pg_root="$repo_root/.tools/pg"
pg_bin="$pg_root/usr/lib/postgresql/18/bin"
pg_share="$pg_root/usr/share/postgresql/18"
pg_lib="$pg_root/usr/lib/x86_64-linux-gnu"
pg_source=repository_runtime
mode="${1:-run}"

case "$mode" in
  inspect|run) ;;
  *) printf 'usage: %s [inspect|run]\n' "$0" >&2; exit 2 ;;
esac

if env | cut -d= -f1 | grep -Eq '^POLIS_.*DSN$'; then
  printf 'postgres_integration=NOT_RUN\nreason=refusing_inherited_database_dsn\n'
  exit 2
fi
while IFS= read -r pg_variable; do
  unset "$pg_variable"
done < <(compgen -e | grep '^PG[A-Z0-9_]*$' || true)
if [[ "$(id -u)" -eq 0 ]]; then
  printf 'postgres_integration=NOT_RUN\nreason=must_run_as_unprivileged_user\n'
  exit 2
fi

missing=()
if [[ ! -x "$pg_bin/postgres" ]]; then
  pg_config_command=''
  if command -v pg_config >/dev/null 2>&1; then
    pg_config_command="$(command -v pg_config)"
  else
    for candidate in /usr/lib/postgresql/18/bin/pg_config /usr/pgsql-18/bin/pg_config /usr/local/pgsql/bin/pg_config /opt/pgsql/18/bin/pg_config; do
      if [[ -x "$candidate" && ! -L "$candidate" ]]; then pg_config_command="$candidate"; break; fi
    done
  fi
  if [[ -n "$pg_config_command" ]]; then
    configured_version="$("$pg_config_command" --version 2>/dev/null || true)"
    if [[ "$configured_version" == *' 18.'* ]]; then
      pg_bin="$("$pg_config_command" --bindir 2>/dev/null || true)"
      pg_share="$("$pg_config_command" --sharedir 2>/dev/null || true)"
      pg_source=system_postgresql18
    fi
  fi
fi
for executable in initdb postgres pg_ctl psql createdb; do
  [[ -x "$pg_bin/$executable" ]] || missing+=("$executable")
done
[[ -d "$pg_share" && ! -L "$pg_share" ]] || missing+=(postgresql_18_share_files)
if ((${#missing[@]})); then
  printf 'postgresql18_runtime=unavailable\n'
  printf 'missing=%s\n' "$(IFS=,; printf '%s' "${missing[*]}")"
  printf 'postgres_integration=NOT_RUN\n'
  printf 'install_method=place approved PostgreSQL 18 Debian packages in .tools/downloads, then run bash scripts/prepare-pg.sh\n'
  exit 3
fi

if [[ "$pg_source" == repository_runtime ]]; then
  export LD_LIBRARY_PATH="$pg_lib"
else
  unset LD_LIBRARY_PATH
fi
version="$($pg_bin/postgres --version | awk '{print $3}')"
[[ "$version" == 18.* ]] || {
  printf 'postgresql18_runtime=invalid\nversion=%s\npostgres_integration=NOT_RUN\n' "$version"
  exit 3
}
ldd_output="$(ldd "$pg_bin/postgres" 2>&1)"
if grep -q 'not found' <<<"$ldd_output"; then
  printf 'postgresql18_runtime=dependencies_missing\n%s\npostgres_integration=NOT_RUN\n' "$ldd_output"
  exit 3
fi
printf 'postgresql18_runtime=available\nversion=%s\nsource=%s\nbin=%s\nshare=%s\n' "$version" "$pg_source" "$pg_bin" "$pg_share"
if [[ "$mode" == inspect ]]; then
  printf 'postgres_integration=READY_TO_RUN\n'
  exit 0
fi

umask 077
temp_root="$(mktemp -d /tmp/polis-self-dev-go-pg.XXXXXX)"
temp_root="$(cd "$temp_root" && pwd -P)"
case "$temp_root" in
  /tmp/polis-self-dev-go-pg.*) ;;
  *) printf 'unsafe temporary PostgreSQL root\n' >&2; exit 1 ;;
esac
data_dir="$temp_root/data"
socket_dir="$temp_root/socket"
blob_root="$temp_root/blobs"
mkdir -m 0700 "$data_dir" "$socket_dir" "$blob_root"
chmod 0700 "$data_dir" "$socket_dir" "$blob_root"

evidence_parent="$repo_root/evidence/development/polis-self-dev-v1-workspace-go-job-patch-slice/postgres-runtime"
mkdir -p "$evidence_parent"
evidence_dir="$evidence_parent/$(date -u +%Y%m%dT%H%M%SZ)-$$"
mkdir -m 0700 "$evidence_dir"

database="polis_r0_self_dev_go_$(date +%s)_$$"
runtime_role="polis_self_dev_go_runtime_$(date +%s)_$$"
admin_user="$(id -un)"
server_started=0

cleanup() {
  local status=$?
  trap - EXIT
  if [[ "$server_started" -eq 1 ]]; then
    if "$pg_bin/pg_ctl" -D "$data_dir" -m fast -w stop >>"$evidence_dir/cleanup.log" 2>&1; then
      server_started=0
    else
      status=1
    fi
  fi
  printf 'postgres_integration_exit=%s\n' "$status" >>"$evidence_dir/summary.txt"
  if [[ "$server_started" -eq 0 && "$temp_root" == /tmp/polis-self-dev-go-pg.* && ! -L "$temp_root" && -d "$data_dir" && ! -L "$data_dir" && -d "$socket_dir" && ! -L "$socket_dir" && -d "$blob_root" && ! -L "$blob_root" ]]; then
    rm -rf -- "$temp_root"
  else
    printf 'cleanup=preserved_for_manual_recovery\ntemporary_root=%s\n' "$temp_root" >>"$evidence_dir/summary.txt"
    status=1
  fi
  printf 'evidence_dir=%s\n' "$evidence_dir"
  exit "$status"
}
trap cleanup EXIT

printf 'environment=disposable_PostgreSQL_18_unix_socket_only\nversion=%s\n' "$version" >"$evidence_dir/summary.txt"
printf 'database=%s\nruntime_role=%s\nnetwork_listener=disabled\n' "$database" "$runtime_role" >>"$evidence_dir/summary.txt"

"$pg_bin/initdb" -D "$data_dir" -L "$pg_share" --locale=C --encoding=UTF8 --auth-local=trust --auth-host=reject --username="$admin_user" >"$evidence_dir/initdb.log" 2>&1
if "$pg_bin/pg_ctl" -D "$data_dir" -l "$evidence_dir/postgres.log" -o "-k $socket_dir -c listen_addresses= -c unix_socket_permissions=0700 -c fsync=on -c synchronous_commit=on -c full_page_writes=on -c jit=off -c max_connections=24" -w start >>"$evidence_dir/initdb.log" 2>&1; then
  server_started=1
else
  if "$pg_bin/pg_ctl" -D "$data_dir" status >/dev/null 2>&1; then server_started=1; fi
  exit 1
fi
admin_dsn="host=$socket_dir dbname=postgres user=$admin_user"
runtime_dsn="host=$socket_dir dbname=$database user=$runtime_role"

"$pg_bin/psql" "$admin_dsn" -X -v ON_ERROR_STOP=1 -c "CREATE ROLE $runtime_role LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT" >"$evidence_dir/role.log" 2>&1
"$pg_bin/createdb" -h "$socket_dir" -U "$admin_user" -O "$runtime_role" "$database"
export POLIS_DSN="host=$socket_dir dbname=$database user=$admin_user"
export POLIS_BLOB_ROOT="$blob_root"
if ! bash "$repo_root/scripts/go.sh" run ./cmd/polis migrate >"$evidence_dir/migration.log" 2>&1; then
  printf 'migration=FAIL\n' >>"$evidence_dir/summary.txt"
  exit 1
fi

"$pg_bin/psql" "$POLIS_DSN" -X -v ON_ERROR_STOP=1 -c "GRANT USAGE ON SCHEMA public TO $runtime_role; GRANT SELECT,INSERT,UPDATE ON ALL TABLES IN SCHEMA public TO $runtime_role; GRANT USAGE,SELECT ON ALL SEQUENCES IN SCHEMA public TO $runtime_role; CREATE TABLE self_dev_go_job_privilege_probe(id integer PRIMARY KEY, value text NOT NULL); GRANT SELECT,INSERT,UPDATE ON self_dev_go_job_privilege_probe TO $runtime_role" >"$evidence_dir/grants.log" 2>&1
schema_identity="$("$pg_bin/psql" "$POLIS_DSN" -X -At -v ON_ERROR_STOP=1 -c "SELECT current_setting('server_version_num')::integer/10000 || ':' || COALESCE(max(version_id),0)::text FROM goose_db_version WHERE is_applied")"
expected_schema="$(find "$repo_root/db/migrations" -maxdepth 1 -type f -name '[0-9][0-9][0-9][0-9][0-9]_*.sql' -printf '%f\n' | sort | tail -n 1 | cut -d_ -f1 | sed 's/^0*//')"
[[ "$schema_identity" == "18:${expected_schema}" ]] || { printf 'schema_identity=FAIL:%s expected=18:%s\n' "$schema_identity" "$expected_schema" >>"$evidence_dir/summary.txt"; exit 1; }

export POLIS_TEST_DSN="$runtime_dsn"
export POLIS_MEMORY_REVOCATION_ROOT="$temp_root/memory-revocations"
mkdir -m 0700 "$POLIS_MEMORY_REVOCATION_ROOT"
"$pg_bin/psql" "$POLIS_TEST_DSN" -X -A -t -v ON_ERROR_STOP=1 -c "INSERT INTO self_dev_go_job_privilege_probe VALUES(1,'insert'); UPDATE self_dev_go_job_privilege_probe SET value='updated' WHERE id=1; SELECT value FROM self_dev_go_job_privilege_probe WHERE id=1" >"$evidence_dir/least-privilege-read-write.log" 2>&1
if "$pg_bin/psql" "$POLIS_TEST_DSN" -X -v ON_ERROR_STOP=1 -c 'DELETE FROM self_dev_go_job_privilege_probe WHERE id=1' >"$evidence_dir/least-privilege-delete.log" 2>&1; then
  printf 'least_privilege=FAIL_delete_was_allowed\n' >>"$evidence_dir/summary.txt"
  exit 1
fi
"$pg_bin/psql" "$POLIS_TEST_DSN" -X -A -t -v ON_ERROR_STOP=1 -c "SELECT current_user,rolsuper,current_setting('server_version_num') FROM pg_roles WHERE rolname=current_user" -c "SELECT pg_try_advisory_lock(714209831);" -c "SELECT pg_advisory_unlock(714209831);" >"$evidence_dir/runtime-identity.log" 2>&1
{
  printf 'temporary_memory_revocation_root=%s\n' "$POLIS_MEMORY_REVOCATION_ROOT"
  stat -c 'mode=%a path=%n' "$POLIS_MEMORY_REVOCATION_ROOT" "$repo_root/.runtime/linux/test-tmp"
} >"$evidence_dir/runtime-filesystem.log" 2>&1

unset POLIS_DSN
go_test_args=(-p 1 -count=1)
if [[ -n "${SELFDEV_GO_TEST_PATTERN:-}" ]]; then
  go_test_args+=(-run "$SELFDEV_GO_TEST_PATTERN" -v)
  printf 'test_pattern=%s\n' "$SELFDEV_GO_TEST_PATTERN" >>"$evidence_dir/summary.txt"
fi
if ! bash "$repo_root/scripts/go.sh" test "${go_test_args[@]}" ./internal/kernel ./internal/control ./internal/workbench >"$evidence_dir/go-postgres-tests.log" 2>&1; then
  printf 'migration=PASS\nschema_identity=%s\nleast_privilege=PASS\npostgres_backed_tests=FAIL\n' "$schema_identity" >>"$evidence_dir/summary.txt"
  exit 1
fi
if [[ -n "${SELFDEV_GO_TEST_PATTERN:-}" ]]; then
  printf 'migration=PASS\nschema_identity=%s\nleast_privilege=PASS\npostgres_backed_tests=TARGETED_PASS\n' "$schema_identity" >>"$evidence_dir/summary.txt"
else
  printf 'migration=PASS\nschema_identity=%s\nleast_privilege=PASS\npostgres_backed_tests=PASS\n' "$schema_identity" >>"$evidence_dir/summary.txt"
fi
printf 'postgres_integration=PASS\n'
