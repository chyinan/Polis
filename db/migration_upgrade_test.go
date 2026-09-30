// pattern: Imperative Shell
package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
)

func TestSchema35LegacyUpgradeAndRollback(t *testing.T) {
	dsn := os.Getenv("POLIS_MIGRATION_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated disposable migration database required")
	}

	ctx := context.Background()
	database, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.SetMaxOpenConns(1)
	goose.SetBaseFS(migrations)
	if err = goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}

	if current := migrationVersion(t, ctx, database); current != 39 {
		t.Fatalf("test database starts at schema %d, want fresh schema 39", current)
	}
	if err = goose.DownToContext(ctx, database, "migrations", 33); err != nil {
		t.Fatalf("schema 34/35 rollback failed: %v", err)
	}
	if err = goose.UpToContext(ctx, database, "migrations", 35); err != nil {
		t.Fatalf("schema 34/35 restore failed: %v", err)
	}
	if err = goose.DownToContext(ctx, database, "migrations", 34); err != nil {
		t.Fatalf("schema 35 rollback failed before legacy upgrade cases: %v", err)
	}

	companyID := fmt.Sprintf("migration35-%d", time.Now().UnixNano())
	if _, err = database.ExecContext(ctx, "INSERT INTO companies(id,name,workspace_root) VALUES($1,$1,'.')", companyID); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, `INSERT INTO employees(company_id,id,display_name,role_name) VALUES
($1,'emp-backend','Backend','backend'),($1,'emp-frontend','Frontend','frontend'),($1,'emp-review','Review','review')`, companyID); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, `INSERT INTO missions(company_id,id,state,contract)
VALUES($1,'migration-mission','active','r03-api@1')`, companyID); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, `INSERT INTO tasks(company_id,id,mission_id,owner,kind) VALUES
($1,'backend-task','migration-mission','emp-backend','compat'),
($1,'frontend-task','migration-mission','emp-frontend','review')`, companyID); err != nil {
		t.Fatal(err)
	}

	assertRejected := func(instructionID, taskID, employeeID, want string) {
		t.Helper()
		if _, insertErr := database.ExecContext(ctx, `INSERT INTO operator_instructions(company_id,id,mission_id,task_id,employee_id,content)
VALUES($1,$2,'migration-mission',NULLIF($3,''),NULLIF($4,''),'Legacy pending guidance')`, companyID, instructionID, taskID, employeeID); insertErr != nil {
			t.Fatal(insertErr)
		}
		upgradeErr := goose.UpToContext(ctx, database, "migrations", 35)
		if upgradeErr == nil || !strings.Contains(upgradeErr.Error(), want) {
			t.Fatalf("schema 35 upgrade error=%v, want rejection containing %q", upgradeErr, want)
		}
		if current := migrationVersion(t, ctx, database); current != 34 {
			t.Fatalf("failed upgrade advanced schema to %d, want 34", current)
		}
		if _, deleteErr := database.ExecContext(ctx, "DELETE FROM operator_instructions WHERE company_id=$1 AND id=$2", companyID, instructionID); deleteErr != nil {
			t.Fatal(deleteErr)
		}
	}

	assertRejected("mismatch", "backend-task", "emp-frontend", "cannot migrate mismatched employee/task")
	assertRejected("unassigned", "", "emp-review", "employee without a task in that Mission")
	if _, err = database.ExecContext(ctx, "UPDATE employees SET enabled=false WHERE company_id=$1", companyID); err != nil {
		t.Fatal(err)
	}
	assertRejected("no-recipients", "", "", "without an eligible recipient")
	if _, err = database.ExecContext(ctx, "UPDATE employees SET enabled=(id='emp-backend') WHERE company_id=$1", companyID); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, `INSERT INTO operator_instructions(company_id,id,mission_id,content)
VALUES($1,'valid-mission-broadcast','migration-mission','Legacy mission guidance')`, companyID); err != nil {
		t.Fatal(err)
	}
	if err = goose.UpToContext(ctx, database, "migrations", 35); err != nil {
		t.Fatalf("valid legacy upgrade failed: %v", err)
	}
	var recipientCount int
	var recipientID string
	if err = database.QueryRowContext(ctx, `SELECT count(*),min(employee_id) FROM operator_instruction_recipients
WHERE company_id=$1 AND instruction_id='valid-mission-broadcast'`, companyID).Scan(&recipientCount, &recipientID); err != nil {
		t.Fatal(err)
	}
	if recipientCount != 1 || recipientID != "emp-backend" {
		t.Fatalf("enabled-recipient snapshot=(%d,%q), want one enabled owner", recipientCount, recipientID)
	}
	if err = goose.DownToContext(ctx, database, "migrations", 34); err == nil || !strings.Contains(err.Error(), "cannot roll back guidance recipient assignments") {
		t.Fatalf("schema 35 rollback with recipient history error=%v, want guarded refusal", err)
	}
	if current := migrationVersion(t, ctx, database); current != 35 {
		t.Fatalf("guarded schema 35 rollback changed schema to %d, want 35", current)
	}
	if _, err = database.ExecContext(ctx, "ALTER TABLE operator_instruction_recipients DISABLE TRIGGER operator_instruction_recipients_immutable"); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, "DELETE FROM operator_instruction_recipients WHERE company_id=$1", companyID); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, "ALTER TABLE operator_instruction_recipients ENABLE TRIGGER operator_instruction_recipients_immutable"); err != nil {
		t.Fatal(err)
	}
	if err = goose.DownToContext(ctx, database, "migrations", 34); err != nil {
		t.Fatalf("schema 35 rollback without scoped recipient history failed: %v", err)
	}
	if _, err = database.ExecContext(ctx, `UPDATE operator_instructions SET state='needs_clarification'
WHERE company_id=$1 AND id='valid-mission-broadcast'`, companyID); err != nil {
		t.Fatal(err)
	}
	if err = goose.DownToContext(ctx, database, "migrations", 33); err == nil || !strings.Contains(err.Error(), "cannot roll back operator response history") {
		t.Fatalf("schema 34 rollback with clarification history error=%v, want guarded refusal", err)
	}
	if current := migrationVersion(t, ctx, database); current != 34 {
		t.Fatalf("guarded schema 34 rollback changed schema to %d, want 34", current)
	}
	if _, err = database.ExecContext(ctx, `UPDATE operator_instructions SET state='pending'
WHERE company_id=$1 AND id='valid-mission-broadcast'`, companyID); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, `INSERT INTO worker_sessions(company_id,id,employee_id,task_id,generation,epoch,incarnation,profile,state)
VALUES($1,'migration-response-session','emp-backend','backend-task',1,1,'migration-incarnation','deterministic/fake','active')`, companyID); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, `INSERT INTO operator_instruction_responses(company_id,response_id,instruction_id,employee_id,session_id,task_id,outcome,summary,request_id)
VALUES($1,'migration-response','valid-mission-broadcast','emp-backend','migration-response-session','backend-task','applied','Applied.','migration-response-request')`, companyID); err != nil {
		t.Fatal(err)
	}
	if err = goose.DownToContext(ctx, database, "migrations", 33); err == nil || !strings.Contains(err.Error(), "cannot roll back operator response history") {
		t.Fatalf("schema 34 rollback with stored response error=%v, want guarded refusal", err)
	}
	if current := migrationVersion(t, ctx, database); current != 34 {
		t.Fatalf("stored-response rollback guard changed schema to %d, want 34", current)
	}
	if _, err = database.ExecContext(ctx, "ALTER TABLE operator_instruction_responses DISABLE TRIGGER operator_instruction_responses_immutable"); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, "DELETE FROM operator_instruction_responses WHERE company_id=$1", companyID); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, "ALTER TABLE operator_instruction_responses ENABLE TRIGGER operator_instruction_responses_immutable"); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, "DELETE FROM worker_sessions WHERE company_id=$1 AND id='migration-response-session'", companyID); err != nil {
		t.Fatal(err)
	}
	if err = goose.UpToContext(ctx, database, "migrations", 35); err != nil {
		t.Fatalf("schema 35 restore after guarded rollback checks failed: %v", err)
	}
	if err = goose.UpToContext(ctx, database, "migrations", 36); err != nil {
		t.Fatalf("schema 36 restore after schema 34/35 legacy checks failed: %v", err)
	}
	if err = goose.UpToContext(ctx, database, "migrations", 39); err != nil {
		t.Fatalf("schema 39 restore after schema 34/35 legacy checks failed: %v", err)
	}
}

func TestSchema36ChangeRequestDownGuardWithHistory(t *testing.T) {
	dsn := os.Getenv("POLIS_MIGRATION_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated disposable migration database required")
	}
	database, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if current := migrationVersion(t, context.Background(), database); current != 39 {
		t.Fatalf("test database starts at schema %d, want 39", current)
	}
	ctx := context.Background()
	companyID := fmt.Sprintf("migration36-%d", time.Now().UnixNano())
	if _, err = database.ExecContext(ctx, "INSERT INTO companies(id,name,workspace_root) VALUES($1,$1,'.')", companyID); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, `INSERT INTO employees(company_id,id,display_name,role_name) VALUES($1,'emp-backend','Backend','backend')`, companyID); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, `INSERT INTO missions(company_id,id,state,contract) VALUES($1,'migration-mission','active','r03-api@1')`, companyID); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, `INSERT INTO tasks(company_id,id,mission_id,owner,kind) VALUES($1,'migration-task','migration-mission','emp-backend','compat')`, companyID); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, `INSERT INTO artifacts(company_id,id,task_id,author,digest,bytes,state,verdict,contract)
VALUES($1,'migration-artifact','migration-task','emp-backend',$2,10,'ready','candidate','r0-arithmetic@1')`, companyID, strings.Repeat("a", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, `INSERT INTO mission_change_requests(company_id,change_request_id,mission_id,client_request_id,base_requirements_sha256,change_summary,proposed_title,proposed_goal,block_previous_results)
VALUES($1,'change-1','migration-mission','change-client-1',$2,'Change requirements','Next','Next goal',true)`, companyID, strings.Repeat("b", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, `INSERT INTO mission_change_request_impacts(company_id,change_request_id,revision,impact_sha256,impact,created_by)
VALUES($1,'change-1',1,$2,'{}'::jsonb,'operator')`, companyID, strings.Repeat("c", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, `INSERT INTO mission_change_request_events(company_id,event_id,change_request_id,state,impact_revision,reason_code,actor,command_request_id)
VALUES($1,'event-1','change-1','received',1,'request_received','operator','event-command-1')`, companyID); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, `INSERT INTO mission_change_request_output_blocks(company_id,change_request_id,artifact_id)
VALUES($1,'change-1','migration-artifact')`, companyID); err != nil {
		t.Fatal(err)
	}
	if err = goose.DownToContext(ctx, database, "migrations", 35); err == nil || !strings.Contains(err.Error(), "cannot roll back mission change history") {
		t.Fatalf("schema 36 rollback with change history error=%v, want guarded refusal", err)
	}
	if current := migrationVersion(t, ctx, database); current != 36 {
		t.Fatalf("schema 37 rollback succeeded before the schema 36 guard: current version=%d, want 36", current)
	}
	if err = goose.UpToContext(ctx, database, "migrations", 39); err != nil {
		t.Fatalf("schema 39 restore after schema 36 history guard failed: %v", err)
	}
}

func TestSchema37TakeoverLeaseTablesAndDownGuard(t *testing.T) {
	dsn := os.Getenv("POLIS_MIGRATION_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated disposable migration database required")
	}
	database, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	ctx := context.Background()
	if current := migrationVersion(t, ctx, database); current != 39 {
		t.Fatalf("test database starts at schema %d, want 39", current)
	}
	for _, table := range []string{"mission_change_requests", "mission_change_request_events", "mission_change_request_impacts", "mission_change_request_output_blocks", "task_takeover_leases", "task_takeover_active_slots", "task_takeover_lease_events"} {
		var relation sql.NullString
		if err = database.QueryRowContext(ctx, "SELECT to_regclass('public."+table+"')::text").Scan(&relation); err != nil {
			t.Fatal(err)
		}
		if !relation.Valid {
			t.Fatalf("Schema 37 relation %s is missing", table)
		}
	}
	companyID := fmt.Sprintf("migration37-%d", time.Now().UnixNano())
	if _, err = database.ExecContext(ctx, "INSERT INTO companies(id,name,workspace_root) VALUES($1,$1,'.')", companyID); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, `INSERT INTO employees(company_id,id,display_name,role_name) VALUES($1,'emp-backend','Backend','backend')`, companyID); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, `INSERT INTO missions(company_id,id,state,contract) VALUES($1,'migration-mission','paused','r03-api@1')`, companyID); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, `INSERT INTO tasks(company_id,id,mission_id,owner,kind) VALUES($1,'migration-task','migration-mission','emp-backend','compat')`, companyID); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, `INSERT INTO worker_workspaces(company_id,task_id,digest) VALUES($1,'migration-task',$2)`, companyID, strings.Repeat("a", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, `INSERT INTO task_takeover_leases(company_id,lease_id,mission_id,task_id,client_request_id,base_requirements_sha256,base_workspace_digest,base_workspace_revision,created_by)
VALUES($1,'lease-1','migration-mission','migration-task','lease-client-1',$2,$3,1,'operator')`, companyID, strings.Repeat("b", 64), strings.Repeat("a", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, `INSERT INTO task_takeover_active_slots(company_id,task_id,lease_id,mission_id)
VALUES($1,'migration-task','lease-1','migration-mission')`, companyID); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, `INSERT INTO task_takeover_lease_events(company_id,event_id,lease_id,state,reason_code,actor,command_request_id)
VALUES($1,'lease-event-1','lease-1','granted','process_tree_stopped','operator','lease-event-command-1')`, companyID); err != nil {
		t.Fatal(err)
	}
	if err = goose.DownToContext(ctx, database, "migrations", 36); err == nil || !strings.Contains(err.Error(), "cannot roll back human takeover leases") {
		t.Fatalf("schema 37 rollback with lease history error=%v, want guarded refusal", err)
	}
	if current := migrationVersion(t, ctx, database); current != 37 {
		t.Fatalf("guarded schema 37 rollback changed schema to %d, want 37", current)
	}
	if err = goose.UpToContext(ctx, database, "migrations", 39); err != nil {
		t.Fatalf("schema 39 restore after schema 37 guard failed: %v", err)
	}
}

func TestSchema38SubstantiveAssessmentDownGuard(t *testing.T) {
	dsn := os.Getenv("POLIS_MIGRATION_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated disposable migration database required")
	}
	database, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	ctx := context.Background()
	if current := migrationVersion(t, ctx, database); current != 39 {
		t.Fatalf("test database starts at schema %d, want 39", current)
	}
	for _, table := range []string{"domain_workflow_substantive_assessments", "domain_workflow_substantive_area_assessments"} {
		var relation sql.NullString
		if err = database.QueryRowContext(ctx, "SELECT to_regclass('public."+table+"')::text").Scan(&relation); err != nil || !relation.Valid {
			t.Fatalf("Schema 38 relation %s missing: relation=%+v err=%v", table, relation, err)
		}
	}
	companyID := fmt.Sprintf("migration38-%d", time.Now().UnixNano())
	if _, err = database.ExecContext(ctx, "INSERT INTO companies(id,name,workspace_root) VALUES($1,$1,'.')", companyID); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, `INSERT INTO employees(company_id,id,display_name,role_name) VALUES($1,'emp-review','Reviewer','review')`, companyID); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, `INSERT INTO domain_workflow_evidence_submissions(company_id,record_id,profile_id,profile_revision,readiness_status,qualification_status,execution_enabled,evidence_digest,evidence,reason_codes,request_id)
VALUES($1,'evidence-1','content-operations-reference','content-operations@1','incomplete','not_run',false,$2,$3,'[]'::jsonb,'evidence-request')`, companyID, strings.Repeat("a", 64), []byte(`{"profileId":"content-operations-reference","profileRevision":"content-operations@1","evidence":[]}`)); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, "ALTER TABLE domain_workflow_substantive_assessments DISABLE TRIGGER ALL"); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, `INSERT INTO domain_workflow_substantive_assessments(company_id,assessment_id,record_id,evidence_digest,outcome,reviewer_employee_id,area_assessments,previewed_evidence,request_id)
VALUES($1,'assessment-1','evidence-1',$2,'more_evidence_required','emp-review','[]'::jsonb,'[]'::jsonb,'assessment-request')`, companyID, strings.Repeat("a", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, "ALTER TABLE domain_workflow_substantive_assessments ENABLE TRIGGER ALL"); err != nil {
		t.Fatal(err)
	}
	if err = goose.DownToContext(ctx, database, "migrations", 37); err == nil || !strings.Contains(err.Error(), "cannot roll back substantive domain evidence assessments") {
		t.Fatalf("schema 38 rollback with assessment history error=%v, want guarded refusal", err)
	}
	if current := migrationVersion(t, ctx, database); current != 38 {
		t.Fatalf("guarded schema 38 rollback changed schema to %d, want 38", current)
	}
	if err = goose.UpToContext(ctx, database, "migrations", 39); err != nil {
		t.Fatalf("schema 39 restore after schema 38 guard failed: %v", err)
	}
}

func TestSchema39CrossBackendHandoverMigrationRoundTripsWithoutHistory(t *testing.T) {
	dsn := os.Getenv("POLIS_MIGRATION_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated disposable migration database required")
	}
	database, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	ctx := context.Background()
	if current := migrationVersion(t, ctx, database); current != 39 {
		t.Fatalf("test database starts at schema %d, want 39", current)
	}
	var relation sql.NullString
	if err = database.QueryRowContext(ctx, "SELECT to_regclass('public.cross_backend_handovers')::text").Scan(&relation); err != nil || !relation.Valid {
		t.Fatalf("Schema 39 handover relation missing: relation=%+v err=%v", relation, err)
	}
	var column bool
	if err = database.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='job_runs' AND column_name='handover_id')`).Scan(&column); err != nil || !column {
		t.Fatalf("Schema 39 JobRun handover binding missing: exists=%t err=%v", column, err)
	}
	if _, err = database.ExecContext(ctx, "ALTER TABLE cross_backend_handovers DISABLE TRIGGER ALL"); err != nil {
		t.Fatal(err)
	}
	_, err = database.ExecContext(ctx, `INSERT INTO cross_backend_handovers(company_id,handover_id,mission_id,task_id,source_job_id,source_session_id,source_runtime_incarnation,
source_environment_revision_id,source_profile_id,target_environment_revision_id,target_profile_id,project_source_sha256,package_json_sha256,lockfile_sha256,
workspace_digest,workspace_revision,task_input_manifest_sha256,target_policy_sha256,target_toolchain_sha256,request_id,record_sha256,created_by)
VALUES($1,'handover-1','migration-mission','migration-task','source-job','source-session','runtime-1','windows-env','windows-node-npm@1','linux-env','linux-node-npm@1',
$2,$3,$4,$5,1,$6,$7,$8,'handover-request',$9,'migration-test')`,
		"migration-schema39", strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("c", 64), strings.Repeat("d", 64), strings.Repeat("e", 64), strings.Repeat("f", 64), strings.Repeat("1", 64), strings.Repeat("2", 64))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, "ALTER TABLE cross_backend_handovers ENABLE TRIGGER ALL"); err != nil {
		t.Fatal(err)
	}
	if err = goose.DownToContext(ctx, database, "migrations", 38); err == nil || !strings.Contains(err.Error(), "cannot roll back persisted cross-backend handovers") {
		t.Fatalf("Schema 39 rollback with handover history error=%v, want guarded refusal", err)
	}
	if current := migrationVersion(t, ctx, database); current != 39 {
		t.Fatalf("guarded Schema 39 rollback changed schema to %d, want 39", current)
	}
	if _, err = database.ExecContext(ctx, "ALTER TABLE cross_backend_handovers DISABLE TRIGGER ALL"); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, "DELETE FROM cross_backend_handovers WHERE company_id='migration-schema39'"); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, "ALTER TABLE cross_backend_handovers ENABLE TRIGGER ALL"); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, "ALTER TABLE worker_sessions DISABLE TRIGGER ALL"); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, `INSERT INTO worker_sessions(company_id,id,employee_id,task_id,generation,epoch,incarnation,profile,state,execution_mode)
VALUES('migration-schema39','project-job-session','emp-backend','migration-task',1,1,'runtime-1','project-job:linux-node-npm@1','active','project_job')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, "ALTER TABLE worker_sessions ENABLE TRIGGER ALL"); err != nil {
		t.Fatal(err)
	}
	if err = goose.DownToContext(ctx, database, "migrations", 38); err == nil || !strings.Contains(err.Error(), "cannot roll back persisted cross-backend handovers") {
		t.Fatalf("Schema 39 rollback with project-job session error=%v, want guarded refusal", err)
	}
	if current := migrationVersion(t, ctx, database); current != 39 {
		t.Fatalf("guarded Schema 39 rollback with project-job session changed schema to %d, want 39", current)
	}
	if _, err = database.ExecContext(ctx, "ALTER TABLE worker_sessions DISABLE TRIGGER ALL"); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, "DELETE FROM worker_sessions WHERE company_id='migration-schema39' AND id='project-job-session'"); err != nil {
		t.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, "ALTER TABLE worker_sessions ENABLE TRIGGER ALL"); err != nil {
		t.Fatal(err)
	}
	if err = goose.DownToContext(ctx, database, "migrations", 38); err != nil {
		t.Fatalf("Schema 39 down migration failed without persisted handovers: %v", err)
	}
	if current := migrationVersion(t, ctx, database); current != 38 {
		t.Fatalf("Schema 39 down migration version=%d, want 38", current)
	}
	if err = goose.UpToContext(ctx, database, "migrations", 39); err != nil {
		t.Fatalf("Schema 39 up migration failed after empty rollback: %v", err)
	}
}

func migrationVersion(t *testing.T, ctx context.Context, database *sql.DB) int64 {
	t.Helper()
	var version int64
	if err := database.QueryRowContext(ctx, "SELECT max(version_id) FROM goose_db_version WHERE is_applied").Scan(&version); err != nil {
		t.Fatal(err)
	}
	return version
}
