// pattern: Imperative Shell
package workbench_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"polis/internal/kernel"
	"polis/internal/taskvalidation"
	"polis/internal/workbench"
)

func TestPostgresReadStoreProjectsQualifiedCheckpointWithoutLifecycleFilters(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	blobRoot := os.Getenv("POLIS_B17_BLOB_ROOT")
	if blobRoot == "" {
		blobRoot = t.TempDir()
	}
	k, err := kernel.Open(ctx, dsn, blobRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()

	companyID := os.Getenv("POLIS_B17_COMPANY_ID")
	if companyID == "" {
		companyID = fmt.Sprintf("r0-5b17-checkpoint-%d", time.Now().UnixNano())
	}
	scope, err := k.TXCreateCompany(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	acceptance := &taskvalidation.AcceptanceContract{Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{"Task ID: {{task_id}}"}}
	mission, err := k.TXCreateMissionGoalWithAcceptance(ctx, scope, "checkpoint projection", "project the authoritative checkpoint", acceptance, "r0-5b17-create")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXStartMissionCommand(ctx, scope, mission.ID, "r0-5b17-start"); err != nil {
		t.Fatal(err)
	}
	compatTask, err := k.TXPrepareProductTask(ctx, scope, mission.ID, "project the authoritative checkpoint", "r0-5b17-prepare")
	if err != nil {
		t.Fatal(err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	var bootstrapTaskID string
	if err = pool.QueryRow(ctx, "SELECT id FROM tasks WHERE company_id=$1 AND mission_id=$2 AND kind='bootstrap_plan'", companyID, mission.ID).Scan(&bootstrapTaskID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "INSERT INTO tasks(company_id,id,mission_id,owner,kind,state) VALUES($1,'task-no-checkpoint',$2,'emp-backend','compute','completed')", companyID, mission.ID); err != nil {
		t.Fatal(err)
	}
	var digest string
	if err = pool.QueryRow(ctx, "SELECT digest FROM worker_workspaces WHERE company_id=$1 AND task_id=$2", companyID, compatTask.ID).Scan(&digest); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "UPDATE worker_workspaces SET revision=4 WHERE company_id=$1 AND task_id=$2", companyID, compatTask.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "UPDATE tasks SET state='candidate' WHERE company_id=$1 AND id=$2", companyID, compatTask.ID); err != nil {
		t.Fatal(err)
	}

	insertSession := func(id, employeeID, taskID string, epoch int64) {
		t.Helper()
		_, execErr := pool.Exec(ctx, `INSERT INTO worker_sessions(company_id,id,employee_id,task_id,generation,epoch,incarnation,profile,state,stop_receipt)
VALUES($1,$2,$3,$4,1,$5,$6,'offline-model/medium','stopped',$7)`, companyID, id, employeeID, taskID, epoch, "incarnation-"+id, "stop-"+id)
		if execErr != nil {
			t.Fatal(execErr)
		}
	}
	insertSession("session-compat-old", "emp-backend", compatTask.ID, 1)
	insertSession("dfc5dd021ec9550ca299903458a63e3b", "emp-backend", compatTask.ID, 2)
	insertSession("session-bootstrap", "emp-planning", bootstrapTaskID, 1)

	checkpointData := func(kind, validation, finalization string, revision int64) []byte {
		t.Helper()
		data, marshalErr := json.Marshal(map[string]any{
			"kind":               kind,
			"validation_status":  validation,
			"finalization_state": finalization,
			"workspace_digest":   digest,
			"workspace_revision": revision,
			"summary":            "checkpoint projection fixture",
		})
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		return data
	}
	insertCheckpoint := func(id, sessionID string, data []byte) {
		t.Helper()
		if _, execErr := pool.Exec(ctx, "INSERT INTO worker_checkpoints(company_id,id,session_id,digest,data) VALUES($1,$2,$3,$4,$5)", companyID, id, sessionID, digest, data); execErr != nil {
			t.Fatal(execErr)
		}
	}
	insertCheckpoint("checkpoint-progress-old", "session-compat-old", checkpointData("progress", "", "", 1))
	insertCheckpoint("checkpoint-qualified-old", "session-compat-old", checkpointData("qualified", "PASS", "current", 9))
	insertCheckpoint("checkpoint-progress-current", "dfc5dd021ec9550ca299903458a63e3b", checkpointData("progress", "", "", 3))
	insertCheckpoint("e1053ac73bab502e04bfa3bb8b35e39e", "dfc5dd021ec9550ca299903458a63e3b", checkpointData("qualified", "PASS", "current", 4))
	insertCheckpoint("checkpoint-other-task", "session-bootstrap", checkpointData("qualified", "PASS", "current", 1))

	if _, err = pool.Exec(ctx, "INSERT INTO worker_checks(company_id,id,session_id,digest,phase,passed,report) VALUES($1,'check-current',$2,$3,'product',true,'{}'::jsonb)", companyID, "dfc5dd021ec9550ca299903458a63e3b", digest); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO artifacts(company_id,id,task_id,author,digest,bytes,state,contract)
VALUES($1,'9ba046913a0d58ca0ad58df867f297b9',$2,'emp-backend',$3,12,'ready','r0-arithmetic@1')`, companyID, compatTask.ID, digest); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO artifact_staging(company_id,id,task_id,digest)
VALUES($1,'9ba046913a0d58ca0ad58df867f297b9',$2,$3)`, companyID, compatTask.ID, digest); err != nil {
		t.Fatal(err)
	}
	var bindingDigest string
	if err = pool.QueryRow(ctx, "SELECT configuration_digest FROM task_validation_bindings WHERE company_id=$1 AND task_id=$2", companyID, compatTask.ID).Scan(&bindingDigest); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO task_validation_artifact_qualifications(company_id,task_id,artifact_id,checkpoint_id,check_id,session_id,validation_binding_digest,workspace_digest,workspace_revision,runner_revision)
VALUES($1,$2,'9ba046913a0d58ca0ad58df867f297b9','e1053ac73bab502e04bfa3bb8b35e39e','check-current','dfc5dd021ec9550ca299903458a63e3b',$3,$4,4,'text-contains-all@1')`, companyID, compatTask.ID, bindingDigest, digest); err != nil {
		t.Fatal(err)
	}
	var companySequence int64
	if err = pool.QueryRow(ctx, "SELECT company_seq FROM companies WHERE id=$1", companyID).Scan(&companySequence); err != nil {
		t.Fatal(err)
	}
	for index, event := range []struct {
		kind string
		id   string
	}{
		{kind: "work.checkpoint", id: "e1053ac73bab502e04bfa3bb8b35e39e"},
		{kind: "task.delivery", id: compatTask.ID},
		{kind: "provider.turn.completed", id: "dfc5dd021ec9550ca299903458a63e3b"},
	} {
		payload, marshalErr := json.Marshal(map[string]string{"id": event.id, "task_id": compatTask.ID})
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		if _, execErr := pool.Exec(ctx, "INSERT INTO events(company_id,company_seq,kind,payload) VALUES($1,$2,$3,$4)", companyID, companySequence+int64(index)+1, event.kind, payload); execErr != nil {
			t.Fatal(execErr)
		}
	}
	if _, err = pool.Exec(ctx, "UPDATE companies SET company_seq=$2 WHERE id=$1", companyID, companySequence+3); err != nil {
		t.Fatal(err)
	}

	store, err := workbench.NewPostgresReadStore(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	overview, err := store.GetCompanyOverview(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}

	var selected *workbench.CheckpointSummary
	for index := range overview.Checkpoints {
		checkpoint := &overview.Checkpoints[index]
		if checkpoint.TaskID == compatTask.ID {
			selected = checkpoint
		}
	}
	if selected == nil || selected.CheckpointID != "e1053ac73bab502e04bfa3bb8b35e39e" || selected.Kind != "qualified" || selected.QualificationState != "qualified" || selected.SessionState != "stopped" {
		t.Fatalf("qualified checkpoint projection = %+v", selected)
	}
	if selected.ArtifactID == nil || *selected.ArtifactID != "9ba046913a0d58ca0ad58df867f297b9" {
		t.Fatalf("checkpoint artifact relation = %v, want frozen LIVE_5 Artifact", selected.ArtifactID)
	}
	if len(overview.Artifacts) != 1 || overview.Artifacts[0].CheckpointID == nil || *overview.Artifacts[0].CheckpointID != selected.CheckpointID {
		t.Fatalf("artifact/checkpoint coherence failed: artifacts=%+v selected=%+v", overview.Artifacts, selected)
	}
	for _, checkpoint := range overview.Checkpoints {
		if checkpoint.TaskID == compatTask.ID && strings.Contains(checkpoint.CheckpointID, "old") {
			t.Fatalf("stale checkpoint overrode current checkpoint: %+v", checkpoint)
		}
		if checkpoint.TaskID == compatTask.ID && checkpoint.CheckpointID == "checkpoint-other-task" {
			t.Fatalf("checkpoint from another task leaked into compat task: %+v", checkpoint)
		}
	}
	var noCheckpoint bool
	for _, task := range overview.Tasks {
		if task.TaskID == "task-no-checkpoint" {
			noCheckpoint = true
			for _, checkpoint := range overview.Checkpoints {
				if checkpoint.TaskID == task.TaskID {
					t.Fatalf("task without checkpoint received checkpoint: %+v", checkpoint)
				}
			}
		}
	}
	if !noCheckpoint {
		t.Fatal("fixture task without checkpoint was not projected")
	}
}
