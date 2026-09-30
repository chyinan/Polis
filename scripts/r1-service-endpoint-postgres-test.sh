#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

export LD_LIBRARY_PATH="$PWD/.tools/pg/usr/lib/x86_64-linux-gnu"
pg="$PWD/.tools/pg/usr/lib/postgresql/18/bin"
root="$(mktemp -d /tmp/polis-r1-service-endpoint.XXXXXX)"
data="$root/data"
socket="$root/socket"
port=55462
database="polis_r0_r1_service_endpoint_$(date +%s)_$$"
user="$(id -un)"
server_started=0
database_created=0

cleanup() {
	if [ "$database_created" -eq 1 ]; then
		"$pg/dropdb" -h "$socket" -p "$port" "$database" >/dev/null 2>&1 || true
	fi
	if [ "$server_started" -eq 1 ]; then
		"$pg/pg_ctl" -D "$data" -m fast -w stop >/dev/null 2>&1 || true
	fi
	case "$root" in
		/tmp/polis-r1-service-endpoint.*) rm -rf -- "$root" ;;
		*) printf 'refusing to remove unexpected temporary path: %s\n' "$root" >&2; return 1 ;;
	esac
}
trap cleanup EXIT

mkdir -m 700 "$root/socket"
"$pg/initdb" -D "$data" -L "$PWD/.tools/pg/usr/share/postgresql/18" --locale=C --encoding=UTF8 --auth-local=trust --auth-host=reject
"$pg/pg_ctl" -D "$data" -l "$root/postgres.log" -o "-k $socket -p $port -c listen_addresses='' -c fsync=on -c synchronous_commit=on -c jit=off" -w start
server_started=1
"$pg/createdb" -h "$socket" -p "$port" "$database"
database_created=1
admin_dsn="host=$socket port=$port dbname=$database user=$user"

"$pg/createuser" --host "$socket" --port "$port" --username "$user" --no-superuser --no-createdb --no-createrole --login polis_runtime
POLIS_DSN="$admin_dsn" POLIS_MIGRATION_ATTEMPT_ROOT="$root/migration-attempts" bash scripts/go.sh run ./cmd/polis migrate
migration_attempts="$(POLIS_MIGRATION_ATTEMPT_ROOT="$root/migration-attempts" bash scripts/go.sh run ./cmd/polis migration-attempt-status)"
if ! printf '%s' "$migration_attempts" | grep -q '"status":"applied"'; then
	printf 'migration attempt journal did not report the successful upgrade: %s\n' "$migration_attempts" >&2
	exit 1
fi
"$pg/psql" "$admin_dsn" -v ON_ERROR_STOP=1 -c 'GRANT USAGE ON SCHEMA public TO polis_runtime; GRANT SELECT,INSERT,UPDATE ON ALL TABLES IN SCHEMA public TO polis_runtime; GRANT USAGE,SELECT ON ALL SEQUENCES IN SCHEMA public TO polis_runtime;'
runtime_dsn="host=$socket port=$port dbname=$database user=polis_runtime"
POLIS_TEST_DSN="$runtime_dsn" bash scripts/go.sh test ./internal/workbench -run TestEnvironmentReadStoreShowsPolicyAndExecutorQualificationGates -count=1
POLIS_TEST_DSN="$runtime_dsn" bash scripts/go.sh test ./internal/control -run TestProjectJobLifecycleLaunchesOncePersistsLogsAndReplaysWithoutExecution -count=1
POLIS_TEST_DSN="$runtime_dsn" bash scripts/go.sh test ./internal/control -run TestEnvironmentPreparationExecutorRunsOnlyAfterQualificationAndReplaysReadyState -count=1
POLIS_TEST_DSN="$runtime_dsn" bash scripts/go.sh test ./internal/kernel -run TestTaskWorkspaceCASRehydratesForNewSessionAfterKernelRestart -count=1
