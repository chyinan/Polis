#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
export LD_LIBRARY_PATH="$PWD/.tools/pg/usr/lib/x86_64-linux-gnu"
pg="$PWD/.tools/pg/usr/lib/postgresql/18/bin"
temp_root="$(mktemp -d /tmp/polis-worker-recovery.XXXXXX)"
data="$temp_root/data"
socket="$temp_root/socket"
port=55432
tmp="$PWD/.runtime/linux/test-tmp"
mkdir -p "$tmp" "evidence/development/r1-r3-implementation-validation-20260927-slice-50-worker-process-reconciliation"
database="polis_r0_worker_recovery_$(date +%s)_$$"
database_created=0
server_started=0
case "$database" in
  polis_r0_worker_recovery_*) ;;
  *) echo "refusing unexpected temporary database name" >&2; exit 1 ;;
esac
cleanup() {
  if [ "$database_created" -eq 1 ]; then
    "$pg/dropdb" -h "$socket" -p "$port" "$database"
  fi
  if [ "$server_started" -eq 1 ]; then
    "$pg/pg_ctl" -D "$data" -m fast -w stop
  fi
  resolved_temp_root="$(realpath "$temp_root")"
  case "$resolved_temp_root" in
    /tmp/polis-worker-recovery.*) rm -rf -- "$resolved_temp_root" ;;
    *) echo "refusing to remove unexpected test directory: $resolved_temp_root" >&2; return 1 ;;
  esac
}
trap cleanup EXIT
mkdir -p "$socket"
chmod 700 "$socket"
"$pg/initdb" -D "$data" -L "$PWD/.tools/pg/usr/share/postgresql/18" --locale=C --encoding=UTF8 --auth-local=trust --auth-host=reject
"$pg/pg_ctl" -D "$data" -l "$temp_root/postgres.log" -o "-k $socket -p $port -c listen_addresses='' -c fsync=on -c synchronous_commit=on -c jit=off" -w start
server_started=1
admin="host=$socket port=$port dbname=postgres user=$(id -un)"
runtime_dsn="host=$socket port=$port dbname=$database user=polis_runtime"
"$pg/psql" "$admin" -v ON_ERROR_STOP=1 -c 'CREATE ROLE polis_runtime LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE'
"$pg/createdb" -h "$socket" -p "$port" "$database"
database_created=1
export POLIS_DSN="host=$socket port=$port dbname=$database user=$(id -un)"
bash scripts/go.sh run ./cmd/polis migrate
"$pg/psql" "$POLIS_DSN" -v ON_ERROR_STOP=1 -c 'GRANT USAGE ON SCHEMA public TO polis_runtime; GRANT SELECT,INSERT,UPDATE ON ALL TABLES IN SCHEMA public TO polis_runtime;'
export POLIS_TEST_DSN="$runtime_dsn"
bash scripts/go.sh test -count=1 -v -run '^(TestWorkerHostContainmentBindingScopesRecoveryCandidates|TestWorkerLifecycleAndMediation|TestTXFinalizeWorkerBeforeProcessIsIdempotentForSameFailure|TestWindowsWorkerHostStopReceiptReplayRequiresExactEvidence)$' ./internal/kernel 2>&1 | tee evidence/development/r1-r3-implementation-validation-20260927-slice-50-worker-process-reconciliation/kernel-postgres-test.txt
bash scripts/go.sh test -count=1 -v -run '^(TestDeterministicWorkerRetainsOwnershipWhenStopPersistenceFails|TestDeterministicStopRetriesProcessAttachmentBeforeConfirmation|TestDeterministicStopReplaysConfirmationAfterAmbiguousCommit|TestDeterministicStopRetainsWindowsHostReconciliationOwner|TestRealProviderWorkerRetainsOwnershipWhenStopPersistenceFails|TestRealProviderStartRetainsSessionWhenUnactivatedCleanupFails|TestRealProviderStartRetainsReservationWhenNoProcessCleanupFails|TestRealProviderStopReplaysConfirmationAfterAmbiguousCommit|TestBusinessPreflightRetainsOwnerWhenProcessStopFails|TestLocalLaunchQualificationRetainsSessionWhenStopFails)$' ./internal/control 2>&1 | tee evidence/development/r1-r3-implementation-validation-20260927-slice-50-worker-process-reconciliation/control-stop-retry-postgres-test.txt
