// pattern: Imperative Shell
package kernel

import (
	"context"
	"polis/internal/runner"
	"testing"
)

// The deterministic analysis enters through the real Planning WorkerSession
// tool boundary. It is not a model turn or an operator-inserted assessment row.
func assessPGTestMissionChange(t *testing.T, k *Kernel, scope Scope, missionID string) string {
	t.Helper()
	check := func(phase string, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("Planning fixture %s: %v", phase, err)
		}
	}
	ctx := context.Background()
	_, err := k.TXSetMissionPausedCommand(ctx, scope, missionID, false, "planning-resume-"+newID())
	check("resume", err)
	// The product bootstrap is already completed. Seed a separate ready review
	// fixture, not a successful assessment or a reopened completed Task. This
	// exercises the assessment transaction, not production Planning dispatch.
	taskID := newID()
	workspaceSHA, err := k.putBlobWithClaim(ctx, scope.company, []byte("Trusted offline Planning review fixture\n"))
	check("review workspace", err)
	_, err = k.pool.Exec(ctx, "INSERT INTO tasks(company_id,id,mission_id,owner,kind) VALUES($1,$2,$3,'emp-planning','review')", scope.company, taskID, missionID)
	check("ready review fixture", err)
	_, err = k.pool.Exec(ctx, "INSERT INTO worker_workspaces(company_id,task_id,digest) VALUES($1,$2,$3)", scope.company, taskID, workspaceSHA)
	check("review workspace fixture", err)
	binding, err := k.TXNewWorkerWithToolBudget(ctx, scope, taskID, "fake/planning", 16)
	check("worker admission", err)
	p, err := runner.Start(binding.SessionID(), []string{"/bin/sleep", "60"}, nil)
	check("trusted sleeper start", err)
	defer p.Stop()
	check("attach", k.TXAttachWorker(ctx, binding, p))
	check("validate", k.TXValidateWorker(ctx, binding))
	check("activate", k.TXActivateWorker(ctx, binding, "fake/planning"))
	view, err := k.MissionChangePlanningContext(ctx, binding)
	check("read context", err)
	affected := []string{}
	for _, task := range view.ChangeRequest.Impact.Tasks {
		affected = append(affected, task.TaskID)
	}
	_, err = k.TXAssessMissionChangeRequest(ctx, binding, MissionChangePlanningAssessmentInput{
		ChangeRequestID: view.ChangeRequest.ID, AnalysisBasisSHA256: view.AnalysisBasisSHA256,
		RiskLevel: "high", Summary: "Trusted offline fixture: the revised requirement affects all source tasks.",
		AffectedTaskIDs: affected, UnaffectedTaskIDs: []string{}, UncertainTaskIDs: []string{}, Questions: []string{},
		RecommendedControls: []string{"Block prior results and freeze all writers before the successor is applied."},
	}, "planning-assessment-"+newID())
	check("submit assessment", err)
	must(t, k.TXBeginStop(ctx, binding))
	proof, err := p.Stop()
	must(t, err)
	must(t, k.TXConfirmStopped(ctx, binding, proof))
	_, err = k.TXSetMissionPausedCommand(ctx, scope, missionID, true, "planning-pause-"+newID())
	must(t, err)
	return workspaceSHA
}
