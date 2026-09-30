// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"polis/internal/core"
	"polis/internal/fixture"
	"polis/internal/runner"
	"strings"
	"testing"
)

func TestWorkerLifecycleAndMediation(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PG required")
	}
	ctx := context.Background()
	k, e := Open(ctx, dsn, t.TempDir())
	must(t, e)
	defer k.Close()
	s, e := k.TXCreateCompany(ctx, "worker-company")
	must(t, e)
	task, e := k.TXCreateProbe(ctx, s, "worker-mission")
	must(t, e)
	fake, e := k.BindFake(ctx, s, "emp-backend")
	must(t, e)
	_, e = k.TXClaim(ctx, fake)
	wantCode(t, e, core.Denied)
	first, e := k.TXNewWorker(ctx, s, task.ID, "gpt-5.6-sol/medium")
	must(t, e)
	p, e := runner.Start(first.SessionID(), []string{"/bin/sleep", "60"}, nil)
	must(t, e)
	defer p.Stop()
	must(t, k.TXAttachWorker(ctx, first, p))
	must(t, k.TXAttachWorker(ctx, first, p))
	_, e = k.Workspace(ctx, first)
	must(t, e)
	_, e = k.TXReplace(ctx, first, "replace", "not-current", "bad")
	wantCode(t, e, core.Denied)
	_, e = k.TXNewWorker(ctx, s, task.ID, "gpt-5.6-sol/high")
	wantCode(t, e, core.Denied)
	must(t, k.TXValidateWorker(ctx, first))
	must(t, k.TXActivateWorker(ctx, first, "measured-capability-digest"))
	unknown := EmployeeTools{Kernel: k, Binding: first, Checker: unknownCheck{}, Phase: "single"}.Call(ctx, "workspace_check", "unknown-result", []byte(`{}`))
	if unknown.Error != "OUTCOME_UNKNOWN" {
		t.Fatalf("uncertain result was not classified: %s", unknown.Error)
	}
	ws, e := k.Workspace(ctx, first)
	must(t, e)
	must(t, k.TXSetPaused(ctx, s, "worker-mission", true))
	_, e = k.TXReplace(ctx, first, "paused", ws.Digest, ws.Content+"\n// paused\n")
	wantCode(t, e, core.Denied)
	must(t, k.TXSetPaused(ctx, s, "worker-mission", false))
	_, e = k.TXReplace(ctx, first, "replace", ws.Digest, ws.Content+"\n// checkpoint\n")
	must(t, e)
	must(t, k.TXBeginStop(ctx, first))
	_, e = k.TXReplace(ctx, first, "late", ws.Digest, "bad")
	wantCode(t, e, core.Denied)
	proof, e := p.Stop()
	must(t, e)
	must(t, k.TXConfirmStopped(ctx, first, proof))
	must(t, k.TXConfirmStopped(ctx, first, proof))
	second, e := k.TXNewWorker(ctx, s, task.ID, "gpt-5.6-sol/high")
	must(t, e)
	p2, e := runner.Start(second.SessionID(), []string{"/bin/sleep", "60"}, nil)
	must(t, e)
	defer p2.Stop()
	must(t, k.TXAttachWorker(ctx, second, p2))
	_, e = k.TXReplace(ctx, first, "stale", ws.Digest, "bad")
	wantCode(t, e, core.StaleEpoch)
	bundle, e := k.Handover(ctx, second)
	must(t, e)
	if bundle.EmployeeID != "emp-backend" || bundle.Task.ID != task.ID || len(bundle.Obligations) != 1 {
		t.Fatal("lost neutral state")
	}
	_, e = k.TXReplace(ctx, second, "restore-write", bundle.Workspace.Digest, "bad")
	wantCode(t, e, core.Denied)
	must(t, k.TXValidateWorker(ctx, second))
	must(t, k.TXSetPaused(ctx, s, "worker-mission", true))
	wantCode(t, k.TXActivateWorker(ctx, second, "paused-capability"), core.Denied)
	must(t, k.TXSetPaused(ctx, s, "worker-mission", false))
	must(t, k.TXActivateWorker(ctx, second, "measured-capability-digest"))
	_, e = k.TXReplace(ctx, second, "valid-neighbor", bundle.Workspace.Digest, bundle.Workspace.Content+"\n// next\n")
	must(t, e)
}

func TestTXNewWorkerHonorsEmployeeAdmissionBarriers(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PG required")
	}
	ctx := context.Background()
	k, err := Open(ctx, dsn, t.TempDir())
	must(t, err)
	defer k.Close()
	for _, state := range []string{"paused", "waiting_quota"} {
		t.Run(state, func(t *testing.T) {
			scope, err := k.TXCreateCompany(ctx, "worker-admission-barrier-"+state)
			must(t, err)
			task, err := k.TXCreateProbe(ctx, scope, "worker-admission-barrier-mission")
			must(t, err)
			_, err = k.pool.Exec(ctx, `UPDATE employee_schedules SET state=$2,pause_reason=$3
WHERE company_id=$1 AND employee_id='emp-backend'`, scope.company, state, map[string]string{"paused": "mission_paused", "waiting_quota": "provider_quota_exhausted"}[state])
			must(t, err)

			if _, err = k.TXNewWorker(ctx, scope, task.ID, "offline-model/medium"); err != core.Denied {
				t.Fatalf("TXNewWorker error=%v, want denied while schedule is %s", err, state)
			}
			var sessions int
			if err = k.pool.QueryRow(ctx, `SELECT count(*) FROM worker_sessions WHERE company_id=$1 AND employee_id='emp-backend'`, scope.company).Scan(&sessions); err != nil {
				t.Fatal(err)
			}
			if sessions != 0 {
				t.Fatalf("WorkerSessions after denied %s admission=%d, want 0", state, sessions)
			}
			var persisted string
			if err = k.pool.QueryRow(ctx, `SELECT state FROM employee_schedules WHERE company_id=$1 AND employee_id='emp-backend'`, scope.company).Scan(&persisted); err != nil {
				t.Fatal(err)
			}
			if persisted != state {
				t.Fatalf("schedule after denied admission=%q, want %q", persisted, state)
			}
		})
	}
}

func TestTXNewWorkerMarksEmployeeScheduleAdmitted(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PG required")
	}
	ctx := context.Background()
	k, err := Open(ctx, dsn, t.TempDir())
	must(t, err)
	defer k.Close()
	scope, err := k.TXCreateCompany(ctx, "worker-admitted-schedule-company")
	must(t, err)
	task, err := k.TXCreateProbe(ctx, scope, "worker-admitted-schedule-mission")
	must(t, err)

	if _, err = k.TXNewWorker(ctx, scope, task.ID, "offline-model/medium"); err != nil {
		t.Fatal(err)
	}
	var scheduleState, workerState string
	if err = k.pool.QueryRow(ctx, `SELECT s.state,w.state FROM employee_schedules s
JOIN worker_sessions w ON w.company_id=s.company_id AND w.employee_id=s.employee_id
WHERE s.company_id=$1 AND s.employee_id='emp-backend'`, scope.company).Scan(&scheduleState, &workerState); err != nil {
		t.Fatal(err)
	}
	if scheduleState != "admitted" || workerState != "restoring" {
		t.Fatalf("schedule/WorkerSession reservation=(%q,%q), want (admitted,restoring)", scheduleState, workerState)
	}
	if _, err = k.TXReconcileEmployeeSchedule(ctx, scope, "emp-backend", "worker-admitted-reconcile"); err != nil {
		t.Fatal(err)
	}
	if err = k.pool.QueryRow(ctx, `SELECT state FROM employee_schedules WHERE company_id=$1 AND employee_id='emp-backend'`, scope.company).Scan(&scheduleState); err != nil {
		t.Fatal(err)
	}
	if scheduleState != "admitted" {
		t.Fatalf("schedule after reconciling a restoring WorkerSession=%q, want admitted", scheduleState)
	}
}

type unknownCheck struct{}

func (unknownCheck) Check(string, string) (runner.Report, error) {
	return runner.Report{}, errors.New("unconfirmed checker transport result")
}

func TestEmployeeOpsBehavioralHandover(t *testing.T) {
	dsn, goRoot := os.Getenv("POLIS_TEST_DSN"), os.Getenv("POLIS_GO_ROOT")
	if dsn == "" || goRoot == "" {
		t.Skip("dedicated PG and compiler required")
	}
	ctx := context.Background()
	root := t.TempDir()
	k, e := Open(ctx, dsn, root)
	must(t, e)
	defer k.Close()
	s, e := k.TXCreateCompany(ctx, "behavior-company")
	must(t, e)
	task, e := k.TXCreateProbe(ctx, s, "behavior-mission")
	must(t, e)
	start := func(profile string) (Binding, *runner.Process) {
		b, e := k.TXNewWorker(ctx, s, task.ID, profile)
		must(t, e)
		p, e := runner.Start(b.SessionID(), []string{"/bin/sleep", "120"}, nil)
		must(t, e)
		must(t, k.TXAttachWorker(ctx, b, p))
		must(t, k.TXValidateWorker(ctx, b))
		must(t, k.TXActivateWorker(ctx, b, "sandbox-measured"))
		return b, p
	}
	stop := func(b Binding, p *runner.Process) {
		must(t, k.TXBeginStop(ctx, b))
		proof, e := p.Stop()
		must(t, e)
		must(t, k.TXConfirmStopped(ctx, b, proof))
	}
	b, p := start("gpt-5.6-sol/medium")
	defer p.Stop()
	checker := runner.Verifier{GoRoot: goRoot, Scratch: root}
	tools := EmployeeTools{Kernel: k, Binding: b, Checker: checker, Phase: "single"}
	denied := tools.Call(ctx, "work_current", "spoof", []byte(`{"employee_id":"emp-review"}`))
	if denied.Error != string(core.Malformed) {
		t.Fatal("model supplied identity accepted")
	}
	denied = tools.Call(ctx, "artifact_submit", "null", []byte(`null`))
	if denied.Error != string(core.Malformed) {
		t.Fatal("null arguments accepted")
	}
	w, e := k.Workspace(ctx, b)
	must(t, e)
	source := strings.Replace(fixture.Source, "return strconv.FormatFloat(v, 'f', -1, 64)", "if v > 0 { return \"+\" + strconv.FormatFloat(v, 'f', -1, 64) }; return strconv.FormatFloat(v, 'f', -1, 64)", 1)
	raw, _ := json.Marshal(map[string]string{"expected_digest": w.Digest, "content": source})
	r := tools.Call(ctx, "workspace_replace", "edit", raw)
	if r.Error != "" {
		t.Fatal(r.Error)
	}
	r = tools.Call(ctx, "workspace_check", "check", []byte(`{}`))
	if r.Error != "" {
		t.Fatal(r.Error)
	}
	cp := Checkpoint{Summary: "positive sign added; unit milestone remains", Facts: []string{"negative zero encodes debit direction"}, Decisions: []string{"retain signed-zero branch"}, Rejected: []string{"v >= 0 would classify negative zero as positive"}, EvidenceRefs: []string{r.Receipt.ID}}
	raw, _ = json.Marshal(cp)
	r = tools.Call(ctx, "work_checkpoint", "checkpoint", raw)
	if r.Error != "" {
		t.Fatal(r.Error)
	}
	stop(b, p)
	b2, p2 := start("gpt-5.6-sol/high")
	defer p2.Stop()
	bundle, e := k.Handover(ctx, b2)
	must(t, e)
	if len(bundle.Checkpoints) != 1 || len(bundle.Obligations) != 1 {
		t.Fatal("neutral bundle lost knowledge or responsibility")
	}
	reviewerTools := ReviewerTools{Kernel: k, Binding: b2, Evidence: ReviewerEvidence{SubjectRevision: "e8c48b1b8536d9c5c55c8469bfed0c60b5cdad0c", ArtifactID: "candidate", CandidateDigest: bundle.Workspace.Digest, Contract: "signed-zero@1", TaskInput: "Preserve the signed-zero compatibility contract for the frozen candidate.", AllowedPath: "formatter.go"}}
	current := reviewerTools.Call(ctx, "work_current", "review-current", []byte(`{}`))
	if current.Error != "" {
		t.Fatal(current.Error)
	}
	currentJSON, e := json.Marshal(current.Data)
	must(t, e)
	if strings.Contains(string(currentJSON), "Checkpoints") || strings.Contains(string(currentJSON), "Obligations") {
		t.Fatalf("reviewer context exposed handover internals: %s", currentJSON)
	}
	workspaceRead := reviewerTools.Call(ctx, "workspace_read", "review-workspace", []byte(`{}`))
	if workspaceRead.Error != "" {
		t.Fatal(workspaceRead.Error)
	}
	if _, ok := workspaceRead.Data.(ReviewerWorkspaceView); !ok {
		t.Fatalf("reviewer workspace returned unexpected type %T", workspaceRead.Data)
	}
	if write := reviewerTools.Call(ctx, "workspace_replace", "review-write", []byte(`{"expected_digest":"x","content":"bad"}`)); write.Error != string(core.Denied) {
		t.Fatalf("reviewer write was not denied: %s", write.Error)
	}
	late := tools.Call(ctx, "workspace_replace", "late", []byte(`{"expected_digest":"x","content":"bad"}`))
	if late.Error != string(core.StaleEpoch) {
		t.Fatal("stale write accepted")
	}
	var historical int
	must(t, k.pool.QueryRow(ctx, "SELECT count(*) FROM worker_observations WHERE company_id=$1", s.company).Scan(&historical))
	if historical < 1 {
		t.Fatal("late observation not retained")
	}
	full := source + "\nfunc RenderWithUnit(v float64, unit string) string { if unit == \"\" {return Render(v)}; return Render(v)+\" \"+unit }\n"
	broken := strings.Replace(full, "if math.Signbit(v)", "if math.Signbit(v) && false", 1)
	bad, e := checker.Check(broken, "full")
	must(t, e)
	if bad.Passed {
		t.Fatal("behavioral checker accepted lost signed-zero compatibility")
	}
	tools = EmployeeTools{Kernel: k, Binding: b2, Checker: checker, Phase: "full"}
	raw, _ = json.Marshal(map[string]string{"expected_digest": bundle.Workspace.Digest, "content": full})
	r = tools.Call(ctx, "workspace_replace", "second-edit", raw)
	if r.Error != "" {
		t.Fatal(r.Error)
	}
	r = tools.Call(ctx, "workspace_check", "second-check", []byte(`{}`))
	if r.Error != "" {
		t.Fatal(r.Error)
	}
	cp.EvidenceRefs = []string{r.Receipt.ID}
	raw, _ = json.Marshal(cp)
	r = tools.Call(ctx, "work_checkpoint", "second-checkpoint", raw)
	if r.Error != "" {
		t.Fatal(r.Error)
	}
	r = tools.Call(ctx, "artifact_submit", "submit", []byte(`{}`))
	if r.Error != "" {
		t.Fatal(r.Error)
	}
	stop(b2, p2)
	if _, e := k.TXCreateReviewProbe(ctx, s, "behavior-mission", r.Receipt.ID); e != nil {
		t.Fatalf("review task could not coexist with compat task: %v", e)
	}
	legacyReviewer, e := k.BindFake(ctx, s, "emp-review")
	must(t, e)
	_, e = k.TXVerify(ctx, legacyReviewer, r.Receipt.ID, "wrong-verifier")
	wantCode(t, e, core.Denied)
	report, e := k.VerifyProbe(ctx, s, r.Receipt.ID, "full", checker)
	must(t, e)
	if !report.Passed {
		t.Fatal("valid neighbor rejected")
	}
}
