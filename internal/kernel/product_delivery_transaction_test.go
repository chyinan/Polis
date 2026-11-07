// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"polis/internal/core"
	"polis/internal/runner"
	"polis/internal/taskvalidation"
)

func TestProductTaskSubmitCommitsQualifiedDeliveryAndReplaysIdempotently(t *testing.T) {
	k, scope, task := prepareProductDeliveryTask(t)
	defer k.Close()
	binding, process := activateProductDeliveryWorker(t, k, scope, task)
	defer stopProductDeliveryWorker(t, k, binding, process)

	tools := EmployeeTools{Kernel: k, Binding: binding, ProductSurface: true}
	handover, err := k.Handover(context.Background(), binding)
	if err != nil {
		t.Fatal(err)
	}
	content := strings.Join(handover.Task.ValidationBinding.Contract.RequiredText, "\n") + "\n"
	replaceArgs, err := json.Marshal(map[string]any{"expected_digest": handover.Workspace.Digest, "expected_revision": handover.Workspace.Revision, "content": content})
	if err != nil {
		t.Fatal(err)
	}
	if result := tools.Call(context.Background(), "workspace_replace", "delivery-write", replaceArgs); result.Error != "" {
		t.Fatalf("workspace replacement failed: %+v", result)
	}
	check := tools.Call(context.Background(), "workspace_check", "delivery-check", []byte(`{}`))
	if check.Error != "" || check.Receipt == nil {
		t.Fatalf("workspace check failed: %+v", check)
	}
	first := tools.Call(context.Background(), "task_submit", "delivery-submit", []byte(`{}`))
	if first.Error != "" || first.Receipt == nil || first.Receipt.Status != "candidate" {
		t.Fatalf("task_submit failed: %+v", first)
	}
	var delivery ProductDeliveryResult
	if err = json.Unmarshal(mustJSON(t, first.Data), &delivery); err != nil {
		t.Fatal(err)
	}
	if delivery.ArtifactID != first.Receipt.ID || delivery.TaskID != task.ID || delivery.TaskState != "candidate" || delivery.DeliveryState != "committed" || delivery.CheckpointID == "" || delivery.CheckID != check.Receipt.ID || delivery.WorkspaceRevision != 2 {
		t.Fatalf("unexpected delivery result: %+v", delivery)
	}
	var checkpoints, artifacts, qualifications int
	if err = k.pool.QueryRow(context.Background(), "SELECT count(*) FROM worker_checkpoints WHERE company_id=$1 AND session_id=$2 AND data->>'kind'='qualified'", scope.company, binding.SessionID()).Scan(&checkpoints); err != nil {
		t.Fatal(err)
	}
	if err = k.pool.QueryRow(context.Background(), "SELECT count(*) FROM artifacts WHERE company_id=$1 AND task_id=$2", scope.company, task.ID).Scan(&artifacts); err != nil {
		t.Fatal(err)
	}
	if err = k.pool.QueryRow(context.Background(), "SELECT count(*) FROM task_validation_artifact_qualifications WHERE company_id=$1 AND task_id=$2", scope.company, task.ID).Scan(&qualifications); err != nil {
		t.Fatal(err)
	}
	if checkpoints != 1 || artifacts != 1 || qualifications != 1 {
		t.Fatalf("delivery created duplicate or missing authoritative rows: checkpoints=%d artifacts=%d qualifications=%d", checkpoints, artifacts, qualifications)
	}
	second := tools.Call(context.Background(), "task_submit", "delivery-submit-replay", []byte(`{}`))
	if second.Error != "" || second.Receipt == nil || second.Receipt.ID != first.Receipt.ID || second.Receipt.Status != "candidate" {
		t.Fatalf("duplicate task_submit was not idempotent: first=%+v second=%+v", first, second)
	}
	if err = k.pool.QueryRow(context.Background(), "SELECT count(*) FROM artifacts WHERE company_id=$1 AND task_id=$2", scope.company, task.ID).Scan(&artifacts); err != nil {
		t.Fatal(err)
	}
	if artifacts != 1 {
		t.Fatalf("duplicate task_submit created %d Artifacts", artifacts)
	}
	postWrite, err := json.Marshal(map[string]any{"expected_digest": handover.Workspace.Digest, "expected_revision": 2, "content": content + "late mutation\n"})
	if err != nil {
		t.Fatal(err)
	}
	if result := tools.Call(context.Background(), "workspace_replace", "delivery-after-candidate", postWrite); result.Error != core.Denied.Error() {
		t.Fatalf("candidate Task accepted a post-delivery write: %+v", result)
	}
	if result := tools.Call(context.Background(), "workspace_check", "delivery-check-after-candidate", []byte(`{}`)); result.Error != core.Denied.Error() {
		t.Fatalf("candidate Task accepted a post-delivery check: %+v", result)
	}
}

func TestProductTaskSubmitRequiresCurrentPassingValidation(t *testing.T) {
	k, scope, task := prepareProductDeliveryTask(t)
	defer k.Close()
	binding, process := activateProductDeliveryWorker(t, k, scope, task)
	defer stopProductDeliveryWorker(t, k, binding, process)

	tools := EmployeeTools{Kernel: k, Binding: binding, ProductSurface: true}
	result := tools.Call(context.Background(), "task_submit", "delivery-before-check", []byte(`{}`))
	if result.Error != core.Denied.Error() {
		t.Fatalf("task_submit without workspace_check error=%q want=%s result=%+v", result.Error, core.Denied, result)
	}
	rejection, ok := result.Data.(PeerToolRejection)
	if !ok || rejection.ReasonCode != "validation_required" || rejection.TransactionOutcome != "not_started" {
		t.Fatalf("task_submit without validation lacked structured feedback: %+v", result)
	}
}

func TestProductTaskDeliveryFaultBoundariesRecoverWithoutDuplicatePublication(t *testing.T) {
	points := []productDeliveryFaultPoint{
		productDeliveryAfterStagePersist,
		productDeliveryAfterCASPublish,
		productDeliveryAfterCheckpointPersist,
		productDeliveryAfterArtifactPersist,
		productDeliveryAfterTaskTransition,
		productDeliveryAfterCommit,
	}
	for _, point := range points {
		t.Run(string(point), func(t *testing.T) {
			k, scope, task := prepareProductDeliveryTask(t)
			defer k.Close()
			binding, process := activateProductDeliveryWorker(t, k, scope, task)
			defer stopProductDeliveryWorker(t, k, binding, process)
			tools := EmployeeTools{Kernel: k, Binding: binding, ProductSurface: true}
			handover, err := k.Handover(context.Background(), binding)
			if err != nil {
				t.Fatal(err)
			}
			content := strings.Join(handover.Task.ValidationBinding.Contract.RequiredText, "\n") + "\n"
			replaceArgs, err := json.Marshal(map[string]any{"expected_digest": handover.Workspace.Digest, "expected_revision": handover.Workspace.Revision, "content": content})
			if err != nil {
				t.Fatal(err)
			}
			if result := tools.Call(context.Background(), "workspace_replace", "fault-write", replaceArgs); result.Error != "" {
				t.Fatalf("workspace replacement failed: %+v", result)
			}
			if result := tools.Call(context.Background(), "workspace_check", "fault-check", []byte(`{}`)); result.Error != "" {
				t.Fatalf("workspace check failed: %+v", result)
			}
			failure := errors.New("injected product delivery fault")
			_, _, err = k.txSubmitTaskDelivery(context.Background(), binding, handover.Task, "fault-delivery", []byte(content), func(got productDeliveryFaultPoint) error {
				if got == point {
					return failure
				}
				return nil
			})
			if !errors.Is(err, failure) {
				t.Fatalf("fault point %s returned %v, want injected error", point, err)
			}
			checkpoints, artifacts, qualifications, state := productDeliveryCounts(t, k, scope, task)
			if point == productDeliveryAfterCommit {
				if checkpoints != 1 || artifacts != 1 || qualifications != 1 || state != "candidate" {
					t.Fatalf("post-commit uncertainty did not commit exactly once: checkpoints=%d artifacts=%d qualifications=%d state=%s", checkpoints, artifacts, qualifications, state)
				}
			} else if checkpoints != 0 || artifacts != 0 || qualifications != 0 || state != "working" {
				t.Fatalf("fault point %s left a partial final publication: checkpoints=%d artifacts=%d qualifications=%d state=%s", point, checkpoints, artifacts, qualifications, state)
			}
			replay := tools.Call(context.Background(), "task_submit", "fault-delivery", []byte(`{}`))
			if replay.Error != "" || replay.Receipt == nil || replay.Receipt.Status != "candidate" {
				t.Fatalf("delivery recovery failed after %s: %+v", point, replay)
			}
			checkpoints, artifacts, qualifications, state = productDeliveryCounts(t, k, scope, task)
			if checkpoints != 1 || artifacts != 1 || qualifications != 1 || state != "candidate" {
				t.Fatalf("delivery recovery after %s duplicated or missed publication: checkpoints=%d artifacts=%d qualifications=%d state=%s", point, checkpoints, artifacts, qualifications, state)
			}
		})
	}
}

func TestFrozenLive2WorkspaceCounterfactualUsesSingleTaskSubmit(t *testing.T) {
	k, scope, task := prepareFrozenLive2CounterfactualTask(t)
	defer k.Close()
	binding, process := activateProductDeliveryWorker(t, k, scope, task)
	defer stopProductDeliveryWorker(t, k, binding, process)

	tools := EmployeeTools{Kernel: k, Binding: binding, ProductSurface: true}
	handover, err := k.Handover(context.Background(), binding)
	if err != nil {
		t.Fatal(err)
	}
	const frozenContent = "# Mission fed04f193f1bc6afebf9c152832ccd76\n\nGoal: Create one small product integration smoke artifact through the Polis real-provider product path.\n\nMission ID: fed04f193f1bc6afebf9c152832ccd76\nTask ID: 5b39875f8dc6463f36395b04e4504407\nAcknowledgement: This artifact was produced through the Polis real-provider product path.\nTask summary: This artifact records one small product integration smoke.\n"
	const frozenWorkspaceDigest = "562b5c53a66e9e5c8e326d8d5d13a55cbf56504ea70ac1b085b5a7a4efd805f2"
	const frozenBindingDigest = "2bbaa0948f92f21cb0b7ba7c167bacdce61c91e5ee42821dad2177ba1e91a721"
	if handover.Task.ValidationBinding.ConfigurationDigest != frozenBindingDigest {
		t.Fatalf("counterfactual binding digest=%s want frozen LIVE_2 digest=%s", handover.Task.ValidationBinding.ConfigurationDigest, frozenBindingDigest)
	}
	replaceArgs, err := json.Marshal(map[string]any{"expected_digest": handover.Workspace.Digest, "expected_revision": handover.Workspace.Revision, "content": frozenContent})
	if err != nil {
		t.Fatal(err)
	}
	if result := tools.Call(context.Background(), "workspace_replace", "live2-counterfactual-write", replaceArgs); result.Error != "" {
		t.Fatalf("frozen LIVE_2 workspace replacement failed: %+v", result)
	}
	check := tools.Call(context.Background(), "workspace_check", "live2-counterfactual-check", []byte(`{}`))
	if check.Error != "" || check.Receipt == nil {
		t.Fatalf("frozen LIVE_2 workspace check failed: %+v", check)
	}
	report, ok := check.Data.(taskvalidation.Result)
	if !ok || report.Status != taskvalidation.StatusPass || report.WorkspaceDigest != frozenWorkspaceDigest || report.WorkspaceRevision != 2 || report.ConfigurationDigest != frozenBindingDigest {
		t.Fatalf("frozen LIVE_2 validation replay drifted: %#v", check.Data)
	}
	submit := tools.Call(context.Background(), "task_submit", "live2-counterfactual-submit", []byte(`{}`))
	if submit.Error != "" || submit.Receipt == nil || submit.Receipt.Status != "candidate" {
		t.Fatalf("single counterfactual task_submit did not commit: %+v", submit)
	}
	checkpoints, artifacts, qualifications, state := productDeliveryCounts(t, k, scope, task)
	if checkpoints != 1 || artifacts != 1 || qualifications != 1 || state != "candidate" {
		t.Fatalf("counterfactual delivery state=%d/%d/%d/%s", checkpoints, artifacts, qualifications, state)
	}
}

func prepareFrozenLive2CounterfactualTask(t *testing.T) (*Kernel, Scope, Task) {
	t.Helper()
	dsn := os.Getenv("POLIS_R05B5_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated R0.5B5 PostgreSQL required")
	}
	k, err := Open(context.Background(), dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	companyID := "r05b-live-2"
	scope, err := k.TXCreateCompany(context.Background(), companyID)
	if err != nil {
		k.Close()
		t.Fatal(err)
	}
	missionID := "fed04f193f1bc6afebf9c152832ccd76"
	taskID := "5b39875f8dc6463f36395b04e4504407"
	goal := "Create one small product integration smoke artifact through the Polis real-provider product path."
	missionAcceptance := taskvalidation.AcceptanceContract{Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{
		"Mission ID: {{mission_id}}",
		"Task ID: {{task_id}}",
		"Acknowledgement: This artifact was produced through the Polis real-provider product path.",
		"Task summary: This artifact records one small product integration smoke.",
	}}
	missionJSON, err := json.Marshal(missionAcceptance)
	if err != nil {
		k.Close()
		t.Fatal(err)
	}
	if _, err = k.pool.Exec(context.Background(), `INSERT INTO missions(company_id,id,title,goal,state,activation_id,contract,acceptance_contract)
VALUES($1,$2,$3,$4,'active','live2-counterfactual-activation',$5,$6)`, companyID, missionID, "R0.5B LIVE_2 real-provider product smoke", goal, core.Contract, missionJSON); err != nil {
		k.Close()
		t.Fatal(err)
	}
	if _, err = k.pool.Exec(context.Background(), "INSERT INTO tasks(company_id,id,mission_id,owner,kind,state) VALUES($1,$2,$3,$4,$5,'completed')", companyID, "live2-counterfactual-bootstrap", missionID, core.EmployeePlanningID, core.TaskKindBootstrapPlan); err != nil {
		k.Close()
		t.Fatal(err)
	}
	initialContent := "# Mission " + missionID + "\n\nGoal: " + goal + "\n"
	initialDigest, err := putBlob(k.root, companyID, []byte(initialContent))
	if err != nil {
		k.Close()
		t.Fatal(err)
	}
	if _, err = k.pool.Exec(context.Background(), "INSERT INTO tasks(company_id,id,mission_id,owner,kind,state) VALUES($1,$2,$3,$4,$5,'ready')", companyID, taskID, missionID, core.EmployeeBackendID, core.TaskKindCompat); err != nil {
		k.Close()
		t.Fatal(err)
	}
	bindingContract := taskvalidation.AcceptanceContract{Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{
		"Mission ID: " + missionID,
		"Task ID: " + taskID,
		"Acknowledgement: This artifact was produced through the Polis real-provider product path.",
		"Task summary: This artifact records one small product integration smoke.",
	}}
	bindingJSON, err := json.Marshal(bindingContract)
	if err != nil {
		k.Close()
		t.Fatal(err)
	}
	if _, err = k.pool.Exec(context.Background(), `INSERT INTO task_validation_bindings(company_id,task_id,mission_id,acceptance_revision,runner_kind,runner_revision,configuration_digest,contract)
VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, companyID, taskID, missionID, taskvalidation.AcceptanceContractRevision, taskvalidation.TextContainsAllRunnerKind, taskvalidation.TextContainsAllRunnerRevision, "2bbaa0948f92f21cb0b7ba7c167bacdce61c91e5ee42821dad2177ba1e91a721", bindingJSON); err != nil {
		k.Close()
		t.Fatal(err)
	}
	if _, err = k.pool.Exec(context.Background(), "INSERT INTO worker_workspaces(company_id,task_id,digest) VALUES($1,$2,$3)", companyID, taskID, initialDigest); err != nil {
		k.Close()
		t.Fatal(err)
	}
	task, err := k.taskByID(context.Background(), scope, taskID)
	if err != nil {
		k.Close()
		t.Fatal(err)
	}
	return k, scope, task
}

func productDeliveryCounts(t *testing.T, k *Kernel, scope Scope, task Task) (int, int, int, string) {
	t.Helper()
	var checkpoints, artifacts, qualifications int
	if err := k.pool.QueryRow(context.Background(), "SELECT count(*) FROM worker_checkpoints WHERE company_id=$1 AND session_id=(SELECT id FROM worker_sessions WHERE company_id=$1 AND task_id=$2)", scope.company, task.ID).Scan(&checkpoints); err != nil {
		t.Fatal(err)
	}
	if err := k.pool.QueryRow(context.Background(), "SELECT count(*) FROM artifacts WHERE company_id=$1 AND task_id=$2", scope.company, task.ID).Scan(&artifacts); err != nil {
		t.Fatal(err)
	}
	if err := k.pool.QueryRow(context.Background(), "SELECT count(*) FROM task_validation_artifact_qualifications WHERE company_id=$1 AND task_id=$2", scope.company, task.ID).Scan(&qualifications); err != nil {
		t.Fatal(err)
	}
	var state string
	if err := k.pool.QueryRow(context.Background(), "SELECT state FROM tasks WHERE company_id=$1 AND id=$2", scope.company, task.ID).Scan(&state); err != nil {
		t.Fatal(err)
	}
	return checkpoints, artifacts, qualifications, state
}

func prepareProductDeliveryTask(t *testing.T) (*Kernel, Scope, Task) {
	t.Helper()
	k, scope, _, task := prepareR05B5ProductTask(t)
	return k, scope, task
}

func activateProductDeliveryWorker(t *testing.T, k *Kernel, scope Scope, task Task) (Binding, *runner.Process) {
	t.Helper()
	binding, err := k.TXNewProductProviderWorkerWithToolBudget(context.Background(), scope, task.ID, "offline-model/medium", 32)
	if err != nil {
		t.Fatal(err)
	}
	process, err := runner.Start(binding.SessionID(), []string{"/bin/sleep", "60"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = k.TXAttachWorker(context.Background(), binding, process); err != nil {
		process.Stop()
		t.Fatal(err)
	}
	if err = k.TXValidateWorker(context.Background(), binding); err != nil {
		process.Stop()
		t.Fatal(err)
	}
	if err = k.TXActivateWorker(context.Background(), binding, "offline-model/medium"); err != nil {
		process.Stop()
		t.Fatal(err)
	}
	return binding, process
}

func stopProductDeliveryWorker(t *testing.T, k *Kernel, binding Binding, process *runner.Process) {
	t.Helper()
	if err := k.TXBeginStop(context.Background(), binding); err != nil {
		t.Fatal(err)
	}
	proof, err := process.Stop()
	if err != nil {
		t.Fatal(err)
	}
	if err = k.TXConfirmStopped(context.Background(), binding, proof); err != nil {
		t.Fatal(err)
	}
}

func mustDeliveryError(t *testing.T, err error, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("error=%v want=%v", err, want)
	}
}
