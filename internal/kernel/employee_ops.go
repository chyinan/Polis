// pattern: Imperative Shell
package kernel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"io"
	"polis/internal/core"
	"polis/internal/runner"
)

type ToolResult struct {
	Receipt *Receipt `json:"receipt,omitempty"`
	Data    any      `json:"data,omitempty"`
	Error   string   `json:"error,omitempty"`
	Detail  string   `json:"detail,omitempty"`
}
type CheckRunner interface {
	Check(string, string) (runner.Report, error)
}

// EmployeeTools is constructed by the trusted adapter. No payload can select
// company, employee, attempt, epoch or task. callID comes from the native bridge.
type EmployeeTools struct {
	Kernel  *Kernel
	Binding Binding
	Checker CheckRunner
	Phase   string
}

func strictArgs(raw []byte, v any) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return core.Malformed
	}
	if len(raw) > 8192 {
		return core.TooLarge
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if e := dec.Decode(v); e != nil {
		return core.Malformed
	}
	if e := dec.Decode(new(any)); e != io.EOF {
		return core.Malformed
	}
	return nil
}
func (t EmployeeTools) Call(ctx context.Context, name, callID string, raw []byte) ToolResult {
	if len(callID) == 0 || len(callID) > 256 {
		return ToolResult{Error: string(core.Malformed)}
	}
	key := "tool-" + fingerprint([]string{t.Binding.session, callID})[:60]
	result, e := t.call(ctx, name, key, raw)
	if e != nil {
		if errors.Is(e, core.StaleEpoch) || errors.Is(e, core.Denied) {
			_ = t.Kernel.RecordHistorical(ctx, t.Binding, name, raw, e.Error())
		}
		var known core.Code
		if errors.As(e, &known) {
			return ToolResult{Error: known.Error()}
		}
		return ToolResult{Error: "OUTCOME_UNKNOWN", Detail: e.Error()}
	}
	return result
}
func (t EmployeeTools) call(ctx context.Context, name, key string, raw []byte) (ToolResult, error) {
	k, b := t.Kernel, t.Binding
	switch name {
	case "work_current", "context_read", "workspace_read":
		var args struct{}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		h, e := k.Handover(ctx, b)
		if name == "workspace_read" {
			return ToolResult{Data: h.Workspace}, e
		}
		return ToolResult{Data: h}, e
	case "workspace_replace":
		var args struct {
			ExpectedDigest string `json:"expected_digest"`
			Content        string `json:"content"`
		}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		r, e := k.TXReplace(ctx, b, key, args.ExpectedDigest, args.Content)
		return ToolResult{Receipt: &r}, e
	case "workspace_check":
		var args struct{}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		w, e := k.Workspace(ctx, b)
		if e != nil {
			return ToolResult{}, e
		}
		report, e := t.Checker.Check(w.Content, t.Phase)
		if e != nil {
			return ToolResult{}, e
		}
		r, e := k.TXWrite(ctx, b.scope, &b, key, "workspace.check", w.Digest, func(tx pgx.Tx) (Receipt, error) {
			id := newID()
			body, e := json.Marshal(report)
			if e != nil {
				return Receipt{}, e
			}
			_, e = tx.Exec(ctx, "INSERT INTO worker_checks VALUES($1,$2,$3,$4,$5,$6,$7)", b.scope.company, id, b.session, w.Digest, t.Phase, report.Passed, body)
			return Receipt{id, "persisted"}, e
		})
		return ToolResult{Receipt: &r, Data: report}, e
	case "work_checkpoint":
		var args Checkpoint
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		if args.Summary == "" || len(args.Facts) == 0 || len(args.Decisions) == 0 || len(args.Rejected) == 0 || len(args.Evidence) == 0 {
			return ToolResult{}, core.Malformed
		}
		r, e := k.TXCheckpoint(ctx, b, key, args)
		return ToolResult{Receipt: &r}, e
	case "artifact_submit":
		var args struct{}
		if e := strictArgs(raw, &args); e != nil {
			return ToolResult{}, e
		}
		h, e := k.Handover(ctx, b)
		if e != nil {
			return ToolResult{}, e
		}
		if len(h.Checkpoints) == 0 {
			return ToolResult{}, core.Denied
		}
		r, e := k.TXSubmit(ctx, b, h.Task, key, []byte(h.Workspace.Content))
		return ToolResult{Receipt: &r}, e
	default:
		return ToolResult{}, core.Denied
	}
}

func (k *Kernel) TXCheckpoint(ctx context.Context, b Binding, key string, c Checkpoint) (Receipt, error) {
	return k.TXWrite(ctx, b.scope, &b, key, "work.checkpoint", c, func(tx pgx.Tx) (Receipt, error) {
		var digest string
		e := tx.QueryRow(ctx, "SELECT w.digest FROM worker_workspaces w JOIN worker_sessions s ON w.company_id=s.company_id AND w.task_id=s.task_id WHERE s.company_id=$1 AND s.id=$2", b.scope.company, b.session).Scan(&digest)
		if e != nil {
			return Receipt{}, e
		}
		for _, id := range c.Evidence {
			var valid bool
			e = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM worker_checks WHERE company_id=$1 AND id=$2 AND session_id=$3 AND digest=$4 AND passed)", b.scope.company, id, b.session, digest).Scan(&valid)
			if e != nil {
				return Receipt{}, e
			}
			if !valid {
				return Receipt{}, core.Denied
			}
		}
		raw, e := json.Marshal(c)
		if e != nil {
			return Receipt{}, e
		}
		if len(raw) > 4096 {
			return Receipt{}, core.TooLarge
		}
		id := newID()
		_, e = tx.Exec(ctx, "INSERT INTO worker_checkpoints VALUES($1,$2,$3,$4,$5)", b.scope.company, id, b.session, digest, raw)
		return Receipt{id, "persisted"}, e
	})
}
func (k *Kernel) RecordHistorical(ctx context.Context, b Binding, name string, raw []byte, reason string) error {
	if len(raw) > 8192 {
		return core.TooLarge
	}
	_, e := k.TXWrite(ctx, b.scope, nil, newID(), "worker.historical", b.session, func(tx pgx.Tx) (Receipt, error) {
		data, e := json.Marshal(struct {
			Name      string
			Arguments json.RawMessage
		}{name, raw})
		if e != nil {
			return Receipt{}, e
		}
		id := newID()
		_, e = tx.Exec(ctx, "INSERT INTO worker_observations VALUES($1,$2,$3,$4,$5)", b.scope.company, id, b.session, reason, data)
		return Receipt{id, "historical_only"}, e
	})
	return e
}

// VerifyProbe is a trusted control-side, non-author acceptance entry, never a
// dynamic tool. The supplied checker is frozen by the executable, not the model.
func (k *Kernel) VerifyProbe(ctx context.Context, s Scope, artifact, phase string, v CheckRunner) (runner.Report, error) {
	var digest, author, task string
	e := k.pool.QueryRow(ctx, "SELECT digest,author,task_id FROM artifacts WHERE company_id=$1 AND id=$2 AND state='ready' AND contract='signed-zero@1'", s.company, artifact).Scan(&digest, &author, &task)
	if e != nil {
		return runner.Report{}, e
	}
	if author == "emp-review" {
		return runner.Report{}, core.Denied
	}
	content, e := readBlob(k.root, s.company, digest)
	if e != nil {
		return runner.Report{}, e
	}
	report, e := v.Check(string(content), phase)
	if e != nil {
		return report, e
	}
	if report.Digest != digest {
		return report, core.Integrity
	}
	_, e = k.TXWrite(ctx, s, nil, newID(), "probe.independent_review", []string{artifact, digest, phase}, func(tx pgx.Tx) (Receipt, error) {
		verdict := "failed"
		if report.Passed {
			verdict = "passed"
		}
		_, e := tx.Exec(ctx, "UPDATE artifacts SET verifier='emp-review',verdict=$3 WHERE company_id=$1 AND id=$2 AND digest=$4", s.company, artifact, verdict, digest)
		if e != nil {
			return Receipt{}, e
		}
		if report.Passed {
			_, e = tx.Exec(ctx, "UPDATE tasks SET state='completed' WHERE company_id=$1 AND id=$2", s.company, task)
			if e != nil {
				return Receipt{}, e
			}
			_, e = tx.Exec(ctx, "UPDATE obligations SET state='fulfilled',evidence_id=$3 WHERE company_id=$1 AND task_id=$2", s.company, task, artifact)
			if e != nil {
				return Receipt{}, e
			}
		}
		return Receipt{artifact, verdict}, nil
	})
	if e != nil {
		return report, e
	}
	if !report.Passed {
		return report, fmt.Errorf("independent acceptance failed")
	}
	return report, nil
}
