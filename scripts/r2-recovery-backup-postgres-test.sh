#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

export LD_LIBRARY_PATH="$PWD/.tools/pg/usr/lib/x86_64-linux-gnu"
pg="$PWD/.tools/pg/usr/lib/postgresql/18/bin"
root="$(mktemp -d /tmp/polis-r2-recovery-backup.XXXXXX)"
data="$root/data"
socket="$root/socket"
port=55441
source_database="polis_r0_recovery_source_$(date +%s)_$$"
target_database="polis_r0_recovery_target_$(date +%s)_$$"
user="$(id -un)"
server_started=0
source_created=0
target_created=0

cleanup() {
	if [ "$target_created" -eq 1 ]; then "$pg/dropdb" -h "$socket" -p "$port" "$target_database" >/dev/null 2>&1 || true; fi
	if [ "$source_created" -eq 1 ]; then "$pg/dropdb" -h "$socket" -p "$port" "$source_database" >/dev/null 2>&1 || true; fi
	if [ "$server_started" -eq 1 ]; then "$pg/pg_ctl" -D "$data" -m fast -w stop >/dev/null 2>&1 || true; fi
	case "$root" in
		/tmp/polis-r2-recovery-backup.*) rm -rf -- "$root" ;;
		*) printf 'refusing to remove unexpected temporary path: %s\n' "$root" >&2; return 1 ;;
	esac
}
trap cleanup EXIT

mkdir -m 700 "$root/socket"
"$pg/initdb" -D "$data" -L "$PWD/.tools/pg/usr/share/postgresql/18" --locale=C --encoding=UTF8 --auth-local=trust --auth-host=reject
"$pg/pg_ctl" -D "$data" -l "$root/postgres.log" -o "-k $socket -p $port -c listen_addresses='' -c fsync=on -c synchronous_commit=on -c jit=off" -w start
server_started=1
"$pg/createuser" --host "$socket" --port "$port" --username "$user" --no-superuser --no-createdb --no-createrole --login polis_runtime
"$pg/createdb" -h "$socket" -p "$port" "$source_database"
source_created=1
"$pg/createdb" -h "$socket" -p "$port" "$target_database"
target_created=1

source_admin_dsn="host=$socket port=$port dbname=$source_database user=$user"
target_admin_dsn="host=$socket port=$port dbname=$target_database user=$user"
POLIS_DSN="$source_admin_dsn" bash scripts/go.sh run ./cmd/polis migrate
"$pg/psql" "$source_admin_dsn" -v ON_ERROR_STOP=1 -c 'GRANT USAGE ON SCHEMA public TO polis_runtime; GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA public TO polis_runtime; GRANT USAGE,SELECT ON ALL SEQUENCES IN SCHEMA public TO polis_runtime;'
source_runtime_dsn="host=$socket port=$port dbname=$source_database user=polis_runtime"
target_runtime_dsn="host=$socket port=$port dbname=$target_database user=polis_runtime"
POLIS_RECOVERY_TEST_SOURCE_DSN="$source_runtime_dsn" \
POLIS_RECOVERY_TEST_BACKUP_DSN="$source_admin_dsn" \
POLIS_RECOVERY_TEST_TARGET_DSN="$target_admin_dsn" \
POLIS_RECOVERY_TEST_TARGET_RUNTIME_DSN="$target_runtime_dsn" \
POLIS_RECOVERY_TEST_BLOB_ROOT="$root/source-blobs" \
POLIS_PG_DUMP_PATH="$pg/pg_dump" \
POLIS_PG_RESTORE_PATH="$pg/pg_restore" \
bash scripts/go.sh test ./internal/recovery -run TestRecoveryBackupPackagePostgresRoundTrip -count=1

cli_output="$root/cli-packages"
mkdir -m 700 "$cli_output"
POLIS_BACKUP_DSN="$source_admin_dsn" POLIS_BLOB_ROOT="$root/source-blobs" POLIS_PG_DUMP_PATH="$pg/pg_dump" \
	bash scripts/go.sh run ./cmd/polis recovery-backup "$cli_output" > "$root/cli-backup-result.json"
package_path="$(find "$cli_output" -mindepth 1 -maxdepth 1 -type d -name 'polis-recovery-*' -print -quit)"
[[ -n "$package_path" ]] || { printf 'CLI backup did not publish a package\n' >&2; exit 1; }
bash scripts/go.sh run ./cmd/polis recovery-backup-verify "$package_path" > "$root/cli-verify-result.json"

cli_target_database="polis_r0_recovery_cli_target_$(date +%s)_$$"
"$pg/createdb" -h "$socket" -p "$port" "$cli_target_database"
target_cli_admin_dsn="host=$socket port=$port dbname=$cli_target_database user=$user"
target_cli_runtime_dsn="host=$socket port=$port dbname=$cli_target_database user=polis_runtime"
POLIS_RESTORE_DSN="$target_cli_admin_dsn" POLIS_RUNTIME_ROLE=polis_runtime POLIS_PG_RESTORE_PATH="$pg/pg_restore" \
	bash scripts/go.sh run ./cmd/polis recovery-backup-restore "$package_path" "$root/cli-restored-blobs" > "$root/cli-restore-result.json"
POLIS_GENERATION_DSN="$target_cli_runtime_dsn" \
	bash scripts/go.sh run ./cmd/polis recovery-generation-verify "$package_path" "$root/cli-restored-blobs" > "$root/cli-generation-verification.json"
company_id="$("$pg/psql" "$source_admin_dsn" -Atc "SELECT id FROM companies WHERE id LIKE 'recovery-backup-%' ORDER BY id DESC LIMIT 1")"
mission_id="$("$pg/psql" "$source_admin_dsn" -Atc "SELECT id FROM missions WHERE company_id='$company_id' ORDER BY id DESC LIMIT 1")"
input_digest="$("$pg/psql" "$source_admin_dsn" -Atc "SELECT content_digest FROM mission_inputs WHERE company_id='$company_id' ORDER BY created_at DESC LIMIT 1")"
POLIS_DSN="$target_cli_runtime_dsn" bash scripts/go.sh run ./cmd/polis status "$company_id" "$mission_id" > "$root/cli-restored-status.json"
cmp "$root/source-blobs/$company_id/$input_digest" "$root/cli-restored-blobs/$company_id/$input_digest"
python3 - "$root/cli-backup-result.json" "$root/cli-verify-result.json" "$root/cli-restore-result.json" "$root/cli-generation-verification.json" "$root/cli-restored-status.json" <<'PY'
import json
import sys

backup, verify, restore, generation, status = [json.load(open(path, encoding="utf-8")) for path in sys.argv[1:]]
assert backup["verification"]["status"] == "PASSED"
assert verify["status"] == "PASSED"
assert restore["status"] == "RESTORED"
assert generation["status"] == "PASSED"
assert generation["generationId"] == verify["generationId"]
assert generation["manifestSha256"] == verify["manifestSha256"]
assert generation["databaseName"]
assert status["MissionState"] == "draft"
PY
printf 'R2_RECOVERY_BACKUP_RESTORE=PASSED\n'
