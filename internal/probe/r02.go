// pattern: Imperative Shell
package probe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"polis/internal/core"
	"time"

	"polis/internal/codex"
	"polis/internal/fixture"
	"polis/internal/kernel"
	"polis/internal/runner"
)

type R02Result struct {
	Status             string    `json:"status"`
	Model              string    `json:"model"`
	SubjectRevision    string    `json:"subject_revision,omitempty"`
	Preflight          string    `json:"contamination_preflight,omitempty"`
	Version            string    `json:"codex_version"`
	SchemaDigest       string    `json:"schema_digest"`
	CapabilityDigest   string    `json:"capability_digest"`
	Started            time.Time `json:"started"`
	Finished           time.Time `json:"finished"`
	Turns              []R02Turn `json:"turns"`
	SingleWorker       string    `json:"single_worker"`
	Handover           string    `json:"handover"`
	Behavior           string    `json:"behavior"`
	ReviewerVerdict    string    `json:"reviewer_verdict"`
	ReviewerConfidence string    `json:"reviewer_confidence,omitempty"`
	HiddenVerifier     string    `json:"hidden_verifier_result"`
	ReviewIsolation    string    `json:"review_isolation_result"`
	OldWriter          string    `json:"old_writer"`
	Error              string    `json:"error,omitempty"`
	Usage              R02Usage  `json:"usage"`
}
type R02Usage struct {
	Medium int    `json:"medium_turns"`
	High   int    `json:"high_turns"`
	Money  string `json:"money"`
	Limit  string `json:"limit"`
}
type R02Turn struct {
	Number                    int       `json:"number"`
	Purpose                   string    `json:"purpose"`
	Profile                   string    `json:"profile"`
	Status                    string    `json:"status"`
	Started                   time.Time `json:"started"`
	Finished                  time.Time `json:"finished"`
	DurationMS                int64     `json:"duration_ms"`
	ThreadID                  string    `json:"thread_id,omitempty"`
	SessionID                 string    `json:"session_id,omitempty"`
	ToolEvents                []string  `json:"tool_events"`
	Receipts                  []string  `json:"receipts"`
	StopReceipt               string    `json:"stop_receipt,omitempty"`
	StopConfirmed             bool      `json:"stop_confirmed"`
	ArtifactID                string    `json:"artifact_id,omitempty"`
	CheckpointID              string    `json:"checkpoint_id,omitempty"`
	CheckPassed               bool      `json:"check_passed"`
	ReviewerVerdict           string    `json:"reviewer_verdict,omitempty"`
	ReviewerConfidence        string    `json:"reviewer_confidence,omitempty"`
	ReviewerSubmissionReceipt string    `json:"review_submission_receipt,omitempty"`
	Error                     string    `json:"error,omitempty"`
}
type r02Session struct {
	binding              kernel.Binding
	artifact, capability string
	turn                 R02Turn
}

func RunR02(cfg Config) (result R02Result, err error) {
	if cfg.Model != "gpt-5.6-luna" {
		return result, errors.New("R0.2 requires gpt-5.6-luna")
	}
	if cfg.MediumLimit != 2 || cfg.HighLimit != 1 {
		return result, errors.New("R0.2 limits are exactly 2 medium and 1 high")
	}
	if cfg.Evidence == "" || cfg.DSN == "" || cfg.AuthFile == "" {
		return result, errors.New("R0.2 requires dedicated DSN, evidence directory and authorized auth file")
	}
	if e := os.MkdirAll(cfg.Evidence, 0700); e != nil {
		return result, e
	}
	result = R02Result{Status: "inconclusive", Model: cfg.Model, Version: runner.NativeVersion, SchemaDigest: cfg.SchemaDigest, Started: time.Now().UTC(), Usage: R02Usage{Money: "not_estimated", Limit: "2 Medium + 1 High; 10 minutes; concurrency 1"}}
	defer func() {
		result.Finished = time.Now().UTC()
		if err != nil {
			result.Error = err.Error()
		}
		if e := writeJSON(filepath.Join(cfg.Evidence, "result.json"), result); e != nil {
			err = errors.Join(err, e)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	budget, e := codex.NewBudget(filepath.Join(cfg.Evidence, "allowance.json"), 2, 1)
	if e != nil {
		return result, e
	}
	k, e := kernel.Open(ctx, cfg.DSN, filepath.Join(cfg.Root, "blobs"))
	if e != nil {
		return result, e
	}
	defer k.Close()
	checker := runner.Verifier{GoRoot: cfg.GoRoot, Scratch: cfg.Root, Context: ctx}
	scope, e := k.TXCreateCompany(ctx, "r02-company")
	if e != nil {
		return result, e
	}
	task, e := k.TXCreateProbe(ctx, scope, "mission-r02")
	if e != nil {
		return result, e
	}

	// Gate 1: commit a partial milestone with a mediated edit, executable check,
	// and durable checkpoint. Artifact submission belongs to the successor.
	if e = budget.Reserve("medium"); e != nil {
		return result, e
	}
	first, e := runR02Session(ctx, cfg, k, scope, task.ID, "medium", fixture.PartialInstructions, "single", true, false, checker, 1, "single real worker: mediated partial task")
	result.CapabilityDigest = first.capability
	result.Turns = append(result.Turns, first.turn)
	result.Usage.Medium = budget.Medium
	result.Usage.High = budget.High
	if e != nil {
		return result, e
	}
	if first.turn.CheckpointID == "" || first.turn.ArtifactID != "" {
		return result, errors.New("R0.2 gate 1 did not produce exactly a partial checkpoint")
	}
	if ok, e := k.CheckpointEvidence(ctx, first.binding, first.turn.CheckpointID); e != nil || !ok {
		return result, errors.New("R0.2 gate 1 checkpoint evidence is not bound to its session and digest")
	}
	result.SingleWorker = "passed"

	// Gate 2: replace the old session only after the process stop receipt; the
	// new binding carries the same EmployeeId and a new epoch/session.
	second, e := k.TXNewWorker(ctx, scope, task.ID, cfg.Model+"/medium")
	if e != nil {
		return result, e
	}
	bundle, e := k.Handover(ctx, second)
	if e != nil {
		return result, e
	}
	if len(bundle.Checkpoints) == 0 || len(bundle.Obligations) != 1 {
		return result, errors.New("neutral handover bundle lacks checkpoint or obligation")
	}
	if e = writeJSON(filepath.Join(cfg.Evidence, "handover-bundle.json"), bundle); e != nil {
		return result, e
	}
	before, e := k.Workspace(ctx, second)
	if e != nil {
		return result, e
	}
	_, e = k.TXReplace(ctx, first.binding, "old-writer-after-stop", bundle.Workspace.Digest, "stale old writer")
	if !errors.Is(e, core.StaleEpoch) && !errors.Is(e, core.Denied) {
		return result, fmt.Errorf("old writer did not receive an authorization rejection: %w", e)
	}
	after, e := k.Workspace(ctx, second)
	if e != nil {
		return result, e
	}
	if after.Digest != before.Digest || after.Content != before.Content {
		return result, errors.New("old writer changed successor workspace")
	}
	result.OldWriter = "rejected"
	if e = budget.Reserve("medium"); e != nil {
		return result, e
	}
	secondRun, e := runR02SessionWithBinding(ctx, cfg, k, second, bundle, "medium", fixture.FinalInstructions, "full", false, false, checker, 2, "same EmployeeId: neutral handover continuation")
	result.CapabilityDigest = secondRun.capability
	result.Turns = append(result.Turns, secondRun.turn)
	result.Usage.Medium = budget.Medium
	result.Usage.High = budget.High
	if e != nil {
		return result, e
	}
	if secondRun.artifact == "" {
		return result, errors.New("R0.2 gate 2 produced no continuation artifact")
	}
	if !secondRun.turn.CheckPassed {
		return result, errors.New("R0.2 gate 2 continuation did not produce a passed deterministic workspace check")
	}
	if secondRun.turn.CheckpointID == "" {
		return result, errors.New("R0.2 gate 2 continuation did not produce a checkpoint")
	}
	if ok, e := k.CheckpointEvidence(ctx, secondRun.binding, secondRun.turn.CheckpointID); e != nil || !ok {
		return result, errors.New("R0.2 gate 2 checkpoint evidence is not bound to its session and digest")
	}
	result.Handover = "passed"

	// Gate 3: the independent High reviewer receives read-only worker tools.
	reviewTask, e := k.TXCreateReviewProbe(ctx, scope, "mission-r02", secondRun.artifact)
	if e != nil {
		return result, e
	}
	if e = budget.Reserve("high"); e != nil {
		return result, e
	}
	reviewer, e := runR02ReviewSession(ctx, cfg, k, scope, reviewTask.ID, secondRun.artifact, 3, "independent behavior acceptance")
	result.CapabilityDigest = reviewer.capability
	result.Turns = append(result.Turns, reviewer.turn)
	result.ReviewerVerdict = reviewer.turn.ReviewerVerdict
	result.Usage.Medium = budget.Medium
	result.Usage.High = budget.High
	if e != nil {
		return result, e
	}
	report, verifyErr := k.VerifyProbe(ctx, scope, secondRun.artifact, "full", checker)
	if report.Passed {
		result.HiddenVerifier = "passed"
	} else {
		result.HiddenVerifier = "failed"
	}
	if reviewer.turn.ReviewerVerdict != "" && reviewer.turn.ReviewerSubmissionReceipt != "" {
		result.ReviewIsolation = "passed"
		if !hasAll(reviewer.turn.ToolEvents, "workspace_read", "work_current", "context_read", "review_submit") || containsAny(reviewer.turn.ToolEvents, "workspace_replace", "artifact_submit", "workspace_check", "work_checkpoint") {
			result.ReviewIsolation = "failed"
		}
	} else {
		result.ReviewIsolation = "failed"
	}
	if verifyErr != nil {
		return result, verifyErr
	}
	if reviewer.turn.ReviewerVerdict != "passed" || result.HiddenVerifier != "passed" || result.ReviewIsolation != "passed" {
		result.Behavior = "failed"
		result.Status = "failed"
		return result, errors.New("independent review, hidden verifier and isolation did not all pass")
	}
	result.Behavior = "passed"
	result.Status = "passed"
	return result, nil
}

func runR02Session(ctx context.Context, cfg Config, k *kernel.Kernel, s kernel.Scope, task, effort, prompt, phase string, stopAtCheckpoint, readOnly bool, checker kernel.CheckRunner, num int, purpose string) (out r02Session, err error) {
	b, e := k.TXNewWorker(ctx, s, task, cfg.Model+"/"+effort)
	if e != nil {
		return out, e
	}
	return runR02SessionWithBinding(ctx, cfg, k, b, kernel.HandoverBundle{}, effort, prompt, phase, stopAtCheckpoint, readOnly, checker, num, purpose)
}
func runR02SessionWithBinding(ctx context.Context, cfg Config, k *kernel.Kernel, b kernel.Binding, bundle kernel.HandoverBundle, effort, prompt, phase string, stopAtCheckpoint, readOnly bool, checker kernel.CheckRunner, num int, purpose string) (out r02Session, err error) {
	out.binding = b
	out.turn = R02Turn{Number: num, Purpose: purpose, Profile: cfg.Model + "/" + effort}
	start := time.Now().UTC()
	out.turn.Started = start
	root := filepath.Join(cfg.Root, b.SessionID())
	if e := os.MkdirAll(root, 0700); e != nil {
		return out, e
	}
	args, capDigest, e := runner.NativeArgs(cfg.Binary, filepath.Join(root, "home"), cfg.AuthFile, cfg.ProxyURL)
	if e != nil {
		return out, e
	}
	out.capability = capDigest
	p, e := runner.Start(b.SessionID(), args, []string{"PATH=/usr/bin:/bin"})
	if e != nil {
		return out, e
	}
	stopped := false
	var c *codex.Client
	defer func() {
		if !stopped {
			if e := k.TXBeginStop(context.Background(), b); e == nil {
				if proof, stopErr := p.Stop(); stopErr == nil {
					if e2 := k.TXConfirmStopped(context.Background(), b, proof); e2 == nil {
						out.turn.StopReceipt = proof.Description()
						out.turn.StopConfirmed = proof.For(b.SessionID())
						stopped = true
					}
				}
			}
		}
		if c != nil {
			c.Close()
		}
		p.Stop()
		out.turn.Finished = time.Now().UTC()
		out.turn.DurationMS = out.turn.Finished.Sub(start).Milliseconds()
		if err != nil {
			out.turn.Status = "failed"
			out.turn.Error = err.Error()
		}
		_ = writeJSON(filepath.Join(cfg.Evidence, b.SessionID(), "r02-session.json"), out.turn)
	}()
	if e = k.TXAttachWorker(ctx, b, p); e != nil {
		return out, e
	}
	c, e = codex.NewWithModel(p, filepath.Join(cfg.Evidence, b.SessionID()), cfg.Model)
	if e != nil {
		return out, e
	}
	if e = c.Initialize(ctx); e != nil {
		return out, e
	}
	thread, e := c.StartThread(ctx, effort)
	if e != nil {
		return out, e
	}
	out.turn.ThreadID = thread
	out.turn.SessionID = b.SessionID()
	if len(bundle.Checkpoints) == 0 {
		bundle, e = k.Handover(ctx, b)
		if e != nil {
			return out, e
		}
	}
	if e = k.TXValidateWorker(ctx, b); e != nil {
		return out, e
	}
	if e = k.TXActivateWorker(ctx, b, capDigest); e != nil {
		return out, e
	}
	if raw, e := json.Marshal(bundle); e == nil {
		prompt += "\nNeutral Polis handover bundle:\n" + string(raw)
	}
	tools := kernel.EmployeeTools{Kernel: k, Binding: b, Checker: checker, Phase: phase, ReadOnly: readOnly}
	boundary := false
	turnResult, e := c.Turn(ctx, thread, effort, prompt, func(name, callID string, raw json.RawMessage) (json.RawMessage, bool) {
		out.turn.ToolEvents = append(out.turn.ToolEvents, name)
		r := tools.Call(ctx, name, callID, raw)
		if r.Receipt != nil {
			out.turn.Receipts = append(out.turn.Receipts, r.Receipt.ID)
			if name == "work_checkpoint" {
				out.turn.CheckpointID = r.Receipt.ID
			}
			if name == "artifact_submit" {
				out.turn.ArtifactID = r.Receipt.ID
				out.artifact = r.Receipt.ID
			}
		}
		if name == "workspace_check" {
			if report, ok := r.Data.(runner.Report); ok {
				out.turn.CheckPassed = report.Passed
			}
		}
		data, _ := json.Marshal(r)
		if r.Error == "OUTCOME_UNKNOWN" {
			err = errors.New("outcome_unknown: " + r.Detail)
			return data, true
		}
		if stopAtCheckpoint && name == "work_checkpoint" && r.Error == "" && r.Receipt != nil {
			boundary = true
			return data, true
		}
		return data, false
	})
	if e != nil {
		return out, e
	}
	if stopAtCheckpoint && !boundary {
		return out, errors.New("partial turn completed without checkpoint boundary")
	}
	if !stopAtCheckpoint && turnResult.State != "completed" {
		return out, fmt.Errorf("native turn ended %s", turnResult.State)
	}
	if e = k.TXBeginStop(ctx, b); e != nil {
		return out, e
	}
	proof, e := p.Stop()
	if e != nil {
		return out, e
	}
	if e = k.TXConfirmStopped(ctx, b, proof); e != nil {
		return out, e
	}
	stopped = true
	out.turn.StopReceipt = proof.Description()
	out.turn.StopConfirmed = proof.For(b.SessionID())
	out.turn.Status = "passed"
	return out, nil
}
func runR02ReviewSession(ctx context.Context, cfg Config, k *kernel.Kernel, s kernel.Scope, task, artifact string, num int, purpose string) (out r02Session, err error) {
	b, e := k.TXNewWorker(ctx, s, task, cfg.Model+"/high")
	if e != nil {
		return out, e
	}
	out.binding = b
	out.turn = R02Turn{Number: num, Purpose: purpose, Profile: cfg.Model + "/high"}
	start := time.Now().UTC()
	out.turn.Started = start
	root := filepath.Join(cfg.Root, b.SessionID())
	if e = os.MkdirAll(root, 0700); e != nil {
		return out, e
	}
	args, capDigest, e := runner.NativeArgs(cfg.Binary, filepath.Join(root, "home"), cfg.AuthFile, cfg.ProxyURL)
	if e != nil {
		return out, e
	}
	out.capability = capDigest
	p, e := runner.Start(b.SessionID(), args, []string{"PATH=/usr/bin:/bin"})
	if e != nil {
		return out, e
	}
	stopped := false
	var c *codex.Client
	defer func() {
		if !stopped {
			if e := k.TXBeginStop(context.Background(), b); e == nil {
				if proof, stopErr := p.Stop(); stopErr == nil {
					if e2 := k.TXConfirmStopped(context.Background(), b, proof); e2 == nil {
						out.turn.StopReceipt = proof.Description()
						out.turn.StopConfirmed = proof.For(b.SessionID())
						stopped = true
					}
				}
			}
		}
		if c != nil {
			c.Close()
		}
		p.Stop()
		out.turn.Finished = time.Now().UTC()
		out.turn.DurationMS = out.turn.Finished.Sub(start).Milliseconds()
		if err != nil {
			out.turn.Status = "failed"
			out.turn.Error = err.Error()
		}
		_ = writeJSON(filepath.Join(cfg.Evidence, b.SessionID(), "r02-session.json"), out.turn)
	}()
	if e = k.TXAttachWorker(ctx, b, p); e != nil {
		return out, e
	}
	c, e = codex.NewWithModel(p, filepath.Join(cfg.Evidence, b.SessionID()), cfg.Model)
	if e != nil {
		return out, e
	}
	if e = c.Initialize(ctx); e != nil {
		return out, e
	}
	thread, e := c.StartReviewerThread(ctx, "high")
	if e != nil {
		return out, e
	}
	out.turn.ThreadID = thread
	out.turn.SessionID = b.SessionID()
	if e = k.TXValidateWorker(ctx, b); e != nil {
		return out, e
	}
	if e = k.TXActivateWorker(ctx, b, capDigest); e != nil {
		return out, e
	}
	workspace, e := k.Workspace(ctx, b)
	if e != nil {
		return out, e
	}
	pack := reviewerEvidencePack{SubjectRevision: R02SubjectRevision, ArtifactID: artifact, CandidateDigest: workspace.Digest, Contract: "signed-zero@1", TaskInput: "Preserve the signed-zero compatibility contract and implement the authorized formatting milestones for the frozen formatter candidate.", AllowedPath: "formatter.go", CandidateContent: workspace.Content}
	prompt, e := buildReviewerPrompt(pack)
	if e != nil {
		return out, e
	}
	tools := kernel.ReviewerTools{Kernel: k, Binding: b, Evidence: kernel.ReviewerEvidence{SubjectRevision: pack.SubjectRevision, ArtifactID: pack.ArtifactID, CandidateDigest: pack.CandidateDigest, Contract: pack.Contract, TaskInput: pack.TaskInput, AllowedPath: pack.AllowedPath}}
	turnResult, e := c.Turn(ctx, thread, "high", prompt, func(name, callID string, raw json.RawMessage) (json.RawMessage, bool) {
		out.turn.ToolEvents = append(out.turn.ToolEvents, name)
		r := tools.Call(ctx, name, callID, raw)
		if r.Receipt != nil {
			out.turn.Receipts = append(out.turn.Receipts, r.Receipt.ID)
		}
		if name == "review_submit" && r.Error == "" {
			if verdict, ok := r.Data.(kernel.ReviewerSubmission); ok && r.Receipt != nil {
				out.turn.ReviewerVerdict = verdict.Verdict
				out.turn.ReviewerConfidence = verdict.Confidence
				out.turn.ReviewerSubmissionReceipt = r.Receipt.ID
			}
		}
		data, _ := json.Marshal(r)
		if r.Error == "OUTCOME_UNKNOWN" {
			err = errors.New("outcome_unknown: " + r.Detail)
			return data, true
		}
		return data, false
	})
	if e != nil {
		return out, e
	}
	if turnResult.State != "completed" {
		return out, fmt.Errorf("native reviewer turn ended %s", turnResult.State)
	}
	if out.turn.ReviewerVerdict == "" || out.turn.ReviewerSubmissionReceipt == "" {
		return out, errors.New("reviewer did not produce an explicit persisted verdict")
	}
	if e = k.TXBeginStop(ctx, b); e != nil {
		return out, e
	}
	proof, e := p.Stop()
	if e != nil {
		return out, e
	}
	if e = k.TXConfirmStopped(ctx, b, proof); e != nil {
		return out, e
	}
	stopped = true
	out.turn.StopReceipt = proof.Description()
	out.turn.StopConfirmed = proof.For(b.SessionID())
	out.turn.Status = "passed"
	return out, nil
}
