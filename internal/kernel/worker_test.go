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
	cp := Checkpoint{Summary: "positive sign added; unit milestone remains", Facts: []string{"negative zero encodes debit direction"}, Decisions: []string{"retain signed-zero branch"}, Rejected: []string{"v >= 0 would classify negative zero as positive"}, Evidence: []string{r.Receipt.ID}}
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
	cp.Evidence = []string{r.Receipt.ID}
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
