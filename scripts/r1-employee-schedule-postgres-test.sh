#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

export LD_LIBRARY_PATH="$PWD/.tools/pg/usr/lib/x86_64-linux-gnu"
pg="$PWD/.tools/pg/usr/lib/postgresql/18/bin"
root="$(mktemp -d /tmp/polis-r1-employee-schedule.XXXXXX)"
data="$root/data"
socket="$root/socket"
port=$((56000 + $$ % 5000))
database="polis_r0_employee_schedule_$(date +%s)_$$"
user="$(id -un)"
server_started=0
database_created=0

cleanup() {
	if [[ "$database_created" == 1 ]]; then "$pg/dropdb" -h "$socket" -p "$port" "$database" >/dev/null 2>&1 || true; fi
	if [[ "$server_started" == 1 ]]; then "$pg/pg_ctl" -D "$data" -m fast -w stop >/dev/null 2>&1 || true; fi
	case "$root" in
		/tmp/polis-r1-employee-schedule.*) rm -rf -- "$root" ;;
		*) printf 'refusing to remove unexpected temporary path: %s\n' "$root" >&2; return 1 ;;
	esac
}
trap cleanup EXIT

mkdir -m 700 "$root/socket"
"$pg/initdb" -D "$data" -L "$PWD/.tools/pg/usr/share/postgresql/18" --locale=C --encoding=UTF8 --auth-local=trust --auth-host=reject
"$pg/pg_ctl" -D "$data" -l "$root/postgres.log" -o "-k $socket -p $port -c listen_addresses='' -c fsync=on -c synchronous_commit=on -c jit=off" -w start
server_started=1
"$pg/createuser" --host "$socket" --port "$port" --username "$user" --no-superuser --no-createdb --no-createrole --login polis_runtime
"$pg/createdb" -h "$socket" -p "$port" "$database"
database_created=1
admin_dsn="host=$socket port=$port dbname=$database user=$user"
runtime_dsn="host=$socket port=$port dbname=$database user=polis_runtime"
migration_root="$root/migration-attempts"

POLIS_MIGRATION_ATTEMPT_ROOT="$migration_root" POLIS_DSN="$admin_dsn" bash scripts/go.sh run ./cmd/polis migrate-to 60
"$pg/psql" "$admin_dsn" -v ON_ERROR_STOP=1 <<'SQL'
INSERT INTO companies(id,name,workspace_root) VALUES
 ('wake_migration_pending','wake migration pending','/tmp'),
 ('wake_migration_paused','wake migration paused','/tmp'),
 ('wake_migration_terminal','wake migration terminal','/tmp');
INSERT INTO employees(company_id,id,display_name,role_name,model_profile,enabled) VALUES
 ('wake_migration_pending','emp-backend','Backend','backend','deterministic/fake',true),
 ('wake_migration_pending','emp-frontend','Frontend','frontend','deterministic/fake',true),
 ('wake_migration_pending','emp-review','Review','review','deterministic/fake',true),
 ('wake_migration_pending','emp-planning','Planning','planning','deterministic/fake',true),
 ('wake_migration_paused','emp-backend','Backend','backend','deterministic/fake',true),
 ('wake_migration_terminal','emp-backend','Backend','backend','deterministic/fake',true);
INSERT INTO missions(company_id,id,state,contract,title,goal) VALUES
 ('wake_migration_pending','pending-mission','active','r03-api@1','Pending wake','Preserve pending peer work'),
 ('wake_migration_paused','paused-mission','paused','r03-api@1','Paused mission','Keep schedule paused'),
 ('wake_migration_terminal','cancelled-mission','cancelled','r03-api@1','Cancelled mission','No pending routine work after upgrade'),
 ('wake_migration_terminal','succeeded-mission','succeeded','r03-api@1','Succeeded mission','No pending routine work after upgrade');
INSERT INTO tasks(company_id,id,mission_id,owner,kind,state) VALUES
 ('wake_migration_pending','pending-backend-task','pending-mission','emp-backend','peer_backend','working'),
 ('wake_migration_pending','pending-ready-backend-task','pending-mission','emp-backend','compat','ready'),
 ('wake_migration_pending','pending-frontend-task','pending-mission','emp-frontend','peer_frontend','ready'),
 ('wake_migration_pending','observed-review-task','pending-mission','emp-review','review','ready'),
 ('wake_migration_pending','applied-planning-task','pending-mission','emp-planning','bootstrap_plan','completed'),
 ('wake_migration_paused','paused-backend-task','paused-mission','emp-backend','peer_backend','working'),
 ('wake_migration_paused','paused-ready-review-task','paused-mission','emp-backend','review','ready');
INSERT INTO contract_revisions(company_id,id,mission_id,revision,endpoint,schema,digest,state,proposer,accepter) VALUES
 ('wake_migration_pending','pending-contract','pending-mission',1,'GET /items','{}'::jsonb,repeat('a',64),'accepted','emp-backend','emp-frontend');
INSERT INTO messages(company_id,id,mission_id,task_id,sender,recipient,kind,body,contract_revision_id) VALUES
 ('wake_migration_pending','pending-message','pending-mission','pending-frontend-task','emp-backend','emp-frontend','request','Existing pending work','pending-contract'),
 ('wake_migration_pending','observed-message','pending-mission','observed-review-task','emp-backend','emp-review','request','Existing observed work','pending-contract'),
 ('wake_migration_pending','applied-message','pending-mission','applied-planning-task','emp-backend','emp-planning','request','Existing applied work','pending-contract');
INSERT INTO obligations(company_id,id,task_id,owner,state) VALUES
 ('wake_migration_pending','pending-message','pending-frontend-task','emp-frontend','pending'),
 ('wake_migration_pending','observed-message','observed-review-task','emp-review','observed'),
 ('wake_migration_pending','applied-message','applied-planning-task','emp-planning','applied');
INSERT INTO peer_work_signals(company_id,id,message_id,obligation_id,recipient,state) VALUES
 ('wake_migration_pending','pending-signal','pending-message','pending-message','emp-frontend','pending'),
 ('wake_migration_pending','observed-signal','observed-message','observed-message','emp-review','observed'),
 ('wake_migration_pending','applied-signal','applied-message','applied-message','emp-planning','applied');
INSERT INTO worker_sessions(company_id,id,employee_id,task_id,generation,epoch,incarnation,profile,state) VALUES
 ('wake_migration_paused','paused-active-session','emp-backend','paused-backend-task',1,1,'migration-fixture','offline/fake','active');
SQL
POLIS_MIGRATION_ATTEMPT_ROOT="$migration_root" POLIS_DSN="$admin_dsn" bash scripts/go.sh run ./cmd/polis migrate-to 68
pending_backfill="$("$pg/psql" "$admin_dsn" -Atc "SELECT state || '|' || work_generation || '|' || checked_generation FROM employee_schedules WHERE company_id='wake_migration_pending' AND employee_id='emp-frontend'")"
if [[ "$pending_backfill" != "wake_pending|2|0" ]]; then
	printf 'pending peer/task backfill=%s, want wake_pending|2|0\n' "$pending_backfill" >&2
	exit 1
fi
observed_backfill="$("$pg/psql" "$admin_dsn" -Atc "SELECT state || '|' || work_generation || '|' || checked_generation FROM employee_schedules WHERE company_id='wake_migration_pending' AND employee_id='emp-review'")"
if [[ "$observed_backfill" != "wake_pending|2|0" ]]; then
	printf 'observed peer/task backfill=%s, want wake_pending|2|0\n' "$observed_backfill" >&2
	exit 1
fi
applied_backfill="$("$pg/psql" "$admin_dsn" -Atc "SELECT state || '|' || work_generation || '|' || checked_generation FROM employee_schedules WHERE company_id='wake_migration_pending' AND employee_id='emp-planning'")"
if [[ "$applied_backfill" != "wake_pending|1|0" ]]; then
	printf 'applied peer signal backfill=%s, want wake_pending|1|0\n' "$applied_backfill" >&2
	exit 1
fi
task_backfill="$("$pg/psql" "$admin_dsn" -Atc "SELECT state || '|' || work_generation || '|' || checked_generation FROM employee_schedules WHERE company_id='wake_migration_pending' AND employee_id='emp-backend'")"
if [[ "$task_backfill" != "wake_pending|1|0" ]]; then
	printf 'ready Task backfill=%s, want wake_pending|1|0\n' "$task_backfill" >&2
	exit 1
fi
paused_backfill="$("$pg/psql" "$admin_dsn" -Atc "SELECT state || '|' || work_generation || '|' || checked_generation || '|' || pause_reason FROM employee_schedules WHERE company_id='wake_migration_paused' AND employee_id='emp-backend'")"
if [[ "$paused_backfill" != "paused|1|0|mission_paused" ]]; then
	printf 'paused mission/task backfill=%s, want paused|1|0|mission_paused\n' "$paused_backfill" >&2
	exit 1
fi
"$pg/psql" "$admin_dsn" -v ON_ERROR_STOP=1 <<'SQL'
INSERT INTO routines(company_id,id,mission_id,employee_id,timezone,local_time,next_logical_day,catch_up_policy,max_catch_up)
VALUES
 ('wake_migration_pending','active-legacy-routine','pending-mission','emp-review','UTC','09:00','2026-09-29','skip',1),
 ('wake_migration_terminal','cancelled-routine','cancelled-mission','emp-backend','UTC','09:00','2026-09-29','skip',1),
 ('wake_migration_terminal','succeeded-routine','succeeded-mission','emp-backend','UTC','09:00','2026-09-29','skip',1);
INSERT INTO routine_materializations(company_id,routine_id,request_id,planned_at,next_logical_day,occurrence_count)
VALUES
 ('wake_migration_pending','active-legacy-routine','active-legacy-materialize','2026-09-30T00:00:00Z','2026-09-30',1),
 ('wake_migration_terminal','cancelled-routine','cancelled-materialize','2026-09-30T00:00:00Z','2026-09-29',1),
 ('wake_migration_terminal','succeeded-routine','succeeded-materialize','2026-09-30T00:00:00Z','2026-09-29',1);
INSERT INTO routine_occurrences(company_id,routine_id,occurrence_key,logical_day,scheduled_at,materialization_request_id,state)
VALUES
 ('wake_migration_pending','active-legacy-routine','active-legacy-routine:2026-09-29','2026-09-29','2026-09-29T09:00:00Z','active-legacy-materialize','pending'),
 ('wake_migration_terminal','cancelled-routine','cancelled-routine:2026-09-29','2026-09-29','2026-09-29T09:00:00Z','cancelled-materialize','pending'),
 ('wake_migration_terminal','succeeded-routine','succeeded-routine:2026-09-29','2026-09-29','2026-09-29T09:00:00Z','succeeded-materialize','pending');
SQL
POLIS_MIGRATION_ATTEMPT_ROOT="$migration_root" POLIS_DSN="$admin_dsn" bash scripts/go.sh run ./cmd/polis migrate
terminal_occurrences="$("$pg/psql" "$admin_dsn" -Atc "SELECT string_agg(r.id || ':' || o.state, ',' ORDER BY r.id) FROM routine_occurrences o JOIN routines r ON r.company_id=o.company_id AND r.id=o.routine_id WHERE o.company_id='wake_migration_terminal'")"
if [[ "$terminal_occurrences" != "cancelled-routine:cancelled,succeeded-routine:cancelled" ]]; then
	printf 'terminal Mission occurrence upgrade backfill=%s, want cancelled-routine:cancelled,succeeded-routine:cancelled\n' "$terminal_occurrences" >&2
	exit 1
fi
legacy_instruction_state="$("$pg/psql" "$admin_dsn" -Atc "SELECT state FROM routine_occurrences WHERE company_id='wake_migration_pending' AND routine_id='active-legacy-routine'")"
legacy_task_count="$("$pg/psql" "$admin_dsn" -Atc "SELECT count(*) FROM tasks WHERE company_id='wake_migration_pending' AND plan->>'routine_id'='active-legacy-routine'")"
legacy_link_count="$("$pg/psql" "$admin_dsn" -Atc "SELECT count(*) FROM routine_occurrences WHERE company_id='wake_migration_pending' AND routine_id='active-legacy-routine' AND (task_id IS NOT NULL OR task_instruction_snapshot IS NOT NULL)")"
if [[ "$legacy_instruction_state" != "needs_instruction" || "$legacy_task_count" != "0" || "$legacy_link_count" != "0" ]]; then
	printf 'active legacy Routine upgrade state=%s tasks=%s links=%s, want needs_instruction/0/0\n' "$legacy_instruction_state" "$legacy_task_count" "$legacy_link_count" >&2
	exit 1
fi
"$pg/psql" "$admin_dsn" -v ON_ERROR_STOP=1 -c 'GRANT USAGE ON SCHEMA public TO polis_runtime; GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA public TO polis_runtime; GRANT USAGE,SELECT ON ALL SEQUENCES IN SCHEMA public TO polis_runtime;'
migration_status="$(POLIS_DSN="$admin_dsn" bash scripts/go.sh run ./cmd/polis migration-status)"
case "$migration_status" in
	*'"versionId":72'*) ;;
	*) printf 'migration-status did not report Schema 72\n' >&2; exit 1 ;;
esac

POLIS_TEST_ADMIN_DSN="$admin_dsn" POLIS_TEST_DSN="$runtime_dsn" bash scripts/go.sh test ./internal/kernel -run '^(TestCreateDailyRoutineRequiresTaskInstruction|TestMigratedActiveLegacyRoutineRequiresInstructionThenDelivers|TestLegacyRoutineWithoutInstructionWaitsThenDeliversOnceSupplemented|TestRoutineMaterializationRollsBackTaskAndManifestWhenOccurrenceInsertFails|TestNewCompanyCreatesSleepingEmployeeSchedules|TestActionablePeerWorkCannotBeLostAcrossEmployeeSleepReconciliation|TestFYIPeerMessageDoesNotWakeEmployeeSchedule|TestMissionPauseBlocksPeerWorkAndResumeRestoresPendingWake|TestObservedPeerObligationKeepsStoppedEmployeeWakePending|TestReadyTasksAdvanceEmployeeScheduleOnMissionStartAndResume|TestKernelStartupScanRestoresReadyTaskWakeAfterLostHint|TestCancellingPausedMissionClearsScheduleBarrierForNextMission|TestDailyRoutineMaterializationPersistsAndWakesOwner|TestPausedMissionPersistsDailyRoutineWithoutWakingUntilResume|TestEmployeeScheduleReconcilerListensReconnectsAndScans|TestEmployeeScheduleWakeHintStaysWithinCompanyScope|TestEmployeeScheduleReconcilerMaterializesDueDailyRoutine|TestCancelledMissionRoutineOccurrenceDoesNotRewakeEmployee|TestTXNewWorkerHonorsEmployeeAdmissionBarriers|TestTXNewWorkerMarksEmployeeScheduleAdmitted|TestPeerDirectMessageCanBeSentAndResolvedInBothDirections|TestPeerDirectMessageCannotCrossMissionWithinCompany|TestPeerDirectMessageCannotTargetFinalizedTask|TestPeerFYIMessageCanBeReadAndAcknowledgedByEitherPeer|TestPeerFYIInboxAdvancesAfterAcknowledgement|TestPeerResolveRequiresObligationTaskBoundToSession)$' -count=1
POLIS_TEST_DSN="$runtime_dsn" bash scripts/go.sh test ./internal/workbench -run '^(TestPostgresReadStoreProjectsEmployeeSchedule|TestPostgresReadStoreProjectsMissionDailyRoutines|TestHTTPDailyRoutineCreateRepairAndReadback)$' -count=1
printf 'R1_EMPLOYEE_SCHEDULE_WAKE_PROTOCOL=PASSED\n'
