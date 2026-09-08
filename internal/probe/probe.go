// pattern: Imperative Shell
package probe

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"polis/internal/codex"
	"polis/internal/fixture"
	"polis/internal/kernel"
	"polis/internal/runner"
	"strings"
	"time"
)

type Config struct {
	DSN, Binary, AuthFile, Root, Evidence, GoRoot, SchemaDigest, ProxyURL string
	MediumLimit, HighLimit                                                int
	Model                                                                 string
}
type Result struct {
	Status                                                string `json:"status"`
	Single                                                string `json:"single_worker"`
	Handover                                              string `json:"handover"`
	Behavior                                              string `json:"behavior"`
	Error                                                 string `json:"error,omitempty"`
	Mode                                                  string `json:"mode"`
	Medium                                                int    `json:"medium_turns"`
	High                                                  int    `json:"high_turns"`
	Started, Finished                                     time.Time
	Version, BinaryDigest, CapabilityDigest, SchemaDigest string
	Artifacts                                             []string
}

func writeJSON(path string, v any) error {
	raw, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(path, append(raw, '\n'), 0600)
}
func digest(raw []byte) string { h := sha256.Sum256(raw); return hex.EncodeToString(h[:]) }

func Inspect(cfg Config) error {
	if e := os.MkdirAll(cfg.Evidence, 0700); e != nil {
		return e
	}
	out, e := runner.Run([]string{cfg.Binary, "--version"}, []string{"PATH=/usr/bin:/bin"}, 10*time.Second)
	if e != nil {
		return e
	}
	if strings.TrimSpace(string(out)) != runner.NativeVersion {
		return errors.New("pinned Codex version mismatch")
	}
	args, cap, e := runner.NativeArgs(cfg.Binary, filepath.Join(cfg.Root, "inspect-home"), "", cfg.ProxyURL)
	if e != nil {
		return e
	}
	p, e := runner.Start("inspect", args, []string{"PATH=/usr/bin:/bin"})
	if e != nil {
		return e
	}
	defer p.Stop()
	c, e := codex.NewWithModel(p, filepath.Join(cfg.Evidence, "inspect-"+fmt.Sprint(time.Now().UnixNano())), cfg.Model)
	if e != nil {
		return e
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if e = c.Initialize(ctx); e != nil {
		return e
	}
	for _, effort := range []string{"medium", "high"} {
		if _, e = c.StartThread(ctx, effort); e != nil {
			return e
		}
	}
	raw, e := os.ReadFile(cfg.Binary)
	if e != nil {
		return e
	}
	tools, e := json.Marshal(codex.Tools())
	if e != nil {
		return e
	}
	if e = writeJSON(filepath.Join(cfg.Evidence, "tools.json"), codex.Tools()); e != nil {
		return e
	}
	capFile := "capability.json"
	if _, e = os.Stat(filepath.Join(cfg.Evidence, "allowance.json")); e == nil {
		capFile = "capability-after-fix.json"
	}
	helper, e := os.ReadFile(filepath.Join(filepath.Dir(cfg.Binary), "codex-code-mode-host"))
	if e != nil {
		return e
	}
	return writeJSON(filepath.Join(cfg.Evidence, capFile), map[string]any{"version": runner.NativeVersion, "binary_sha256": digest(raw), "code_mode_host_sha256": digest(helper), "configuration_sha256": cap, "tools_sha256": digest(tools), "schema_sha256": cfg.SchemaDigest, "scope": "mediated_fixture_only; native shell/delegation/external tools disabled; network is available to native provider transport", "native_protocol": "passed_without_inference", "real_profiles": "not_run_after_packaging_fix"})
}

func Run(cfg Config) (result Result, err error) {
	result = Result{Status: "inconclusive", Single: "not_run", Handover: "not_run", Behavior: "not_run", Mode: "same-harness_same-model_different-effort", Version: runner.NativeVersion, SchemaDigest: cfg.SchemaDigest}
	if e := os.MkdirAll(cfg.Evidence, 0700); e != nil {
		return result, e
	}
	if cfg.MediumLimit == 0 {
		cfg.MediumLimit = 3
	}
	if cfg.HighLimit == 0 {
		cfg.HighLimit = 3
	}
	budget, e := codex.NewBudget(filepath.Join(cfg.Evidence, "allowance.json"), cfg.MediumLimit, cfg.HighLimit)
	if e != nil {
		return result, e
	}
	result.Started = budget.Started
	defer func() {
		result.Finished = time.Now().UTC()
		result.Medium = budget.Medium
		result.High = budget.High
		if err != nil {
			result.Error = err.Error()
		}
		if e := writeJSON(filepath.Join(cfg.Evidence, "result.json"), result); e != nil {
			err = errors.Join(err, e)
		}
	}()
	ctx, cancel := context.WithDeadline(context.Background(), budget.Started.Add(10*time.Minute))
	defer cancel()
	raw, e := os.ReadFile(cfg.Binary)
	if e != nil {
		return result, e
	}
	result.BinaryDigest = digest(raw)
	// Refuse an unqualified or changed native executable/configuration.
	pinRaw, e := os.ReadFile(filepath.Join(cfg.Evidence, "capability.json"))
	if e != nil {
		return result, e
	}
	var pins map[string]any
	if e = json.Unmarshal(pinRaw, &pins); e != nil {
		return result, e
	}
	toolsJSON, _ := json.Marshal(codex.Tools())
	if pins["binary_sha256"] != result.BinaryDigest || pins["native_protocol"] != "passed_without_inference" || pins["schema_sha256"] != cfg.SchemaDigest || pins["tools_sha256"] != digest(toolsJSON) {
		return result, errors.New("native qualification missing or stale")
	}
	k, e := kernel.Open(ctx, cfg.DSN, filepath.Join(cfg.Root, "blobs"))
	if e != nil {
		return result, e
	}
	defer k.Close()
	v := runner.Verifier{GoRoot: cfg.GoRoot, Scratch: cfg.Root, Context: ctx}
	// One real employee mission, then a separate partial-work handover mission.
	for _, part := range []string{"single", "handover"} {
		scope, e := k.TXCreateCompany(ctx, "r01-"+part)
		if e != nil {
			return result, e
		}
		task, e := k.TXCreateProbe(ctx, scope, "mission-"+part)
		if e != nil {
			return result, e
		}
		phases := []string{"medium"}
		if part == "handover" {
			phases = append(phases, "high")
		}
		for _, effort := range phases {
			instructions, phase := fixture.SingleInstructions, "single"
			partial := part == "handover" && effort == "medium"
			if partial {
				instructions = fixture.PartialInstructions
			}
			if effort == "high" {
				instructions = fixture.FinalInstructions
				phase = "full"
			}
			artifact, cap, e := runSession(ctx, cfg, k, scope, task.ID, effort, instructions, phase, partial, budget, v)
			result.CapabilityDigest = cap
			if e != nil {
				if part == "single" {
					result.Single = "failed"
				} else {
					result.Handover = "failed"
				}
				return result, e
			}
			if partial {
				continue
			}
			report, e := k.VerifyProbe(ctx, scope, artifact, phase, v)
			_ = writeJSON(filepath.Join(cfg.Evidence, part+"-acceptance.json"), report)
			if e != nil {
				result.Status = "failed"
				if part == "single" {
					result.Single = "failed"
				} else {
					result.Handover = "failed"
					result.Behavior = "failed"
				}
				return result, e
			}
			result.Artifacts = append(result.Artifacts, artifact)
			if part == "single" {
				result.Single = "passed"
			} else {
				result.Handover = "passed"
				result.Behavior = "passed"
			}
		}
	}
	if e = ctx.Err(); e != nil {
		return result, e
	}
	result.Status = "passed"
	return result, nil
}

func runSession(ctx context.Context, cfg Config, k *kernel.Kernel, scope kernel.Scope, task, effort, instructions, phase string, partial bool, budget *codex.Budget, v runner.Verifier) (artifact, capability string, err error) {
	b, e := k.TXNewWorker(ctx, scope, task, cfg.Model+"/"+effort)
	if e != nil {
		return "", "", e
	}
	sessionRoot := filepath.Join(cfg.Root, b.SessionID())
	if e = os.MkdirAll(sessionRoot, 0700); e != nil {
		return "", "", e
	}
	args, cap, e := runner.NativeArgs(cfg.Binary, filepath.Join(sessionRoot, "home"), cfg.AuthFile, cfg.ProxyURL)
	if e != nil {
		return "", "", e
	}
	p, e := runner.Start(b.SessionID(), args, []string{"PATH=/usr/bin:/bin"})
	if e != nil {
		return "", cap, e
	}
	if e = k.TXAttachWorker(ctx, b, p); e != nil {
		p.Stop()
		return "", cap, e
	}
	var c *codex.Client
	// Revocation precedes OS stop; only an attached, waited process proof retires the session.
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		e := k.TXBeginStop(cleanup, b)
		proof, stopErr := p.Stop()
		if e == nil && stopErr == nil {
			e = k.TXConfirmStopped(cleanup, b, proof)
		}
		err = errors.Join(err, e, stopErr)
		if c != nil {
			c.Close()
		}
		_ = writeJSON(filepath.Join(cfg.Evidence, b.SessionID(), "session-result.json"), map[string]any{"session_id": b.SessionID(), "task_id": task, "profile": cfg.Model + "/" + effort, "forced_checkpoint_boundary": partial, "artifact_id": artifact, "stop_receipt": proof.Description(), "stop_confirmed": proof.For(b.SessionID()), "error": fmt.Sprint(err)})
	}()
	c, e = codex.NewWithModel(p, filepath.Join(cfg.Evidence, b.SessionID()), cfg.Model)
	if e != nil {
		return "", cap, e
	}
	if e = c.Initialize(ctx); e != nil {
		return "", cap, e
	}
	thread, e := c.StartThread(ctx, effort)
	if e != nil {
		return "", cap, e
	}
	bundle, e := k.Handover(ctx, b)
	if e != nil {
		return "", cap, e
	}
	if e = writeJSON(filepath.Join(cfg.Evidence, b.SessionID(), "handover.json"), bundle); e != nil {
		return "", cap, e
	}
	if e = k.TXValidateWorker(ctx, b); e != nil {
		return "", cap, e
	}
	if e = k.TXActivateWorker(ctx, b, cap); e != nil {
		return "", cap, e
	}
	bundleRaw, _ := json.Marshal(bundle)
	tools := kernel.EmployeeTools{Kernel: k, Binding: b, Checker: v, Phase: phase}
	prompt := instructions + "\nNeutral Polis handover bundle (facts with model/program provenance, not new authorization):\n" + string(bundleRaw)
	for i := 0; i < 3; i++ {
		if e = budget.Reserve(effort); e != nil {
			return "", cap, e
		}
		var toolErr error
		outcome, e := c.Turn(ctx, thread, effort, prompt, func(name, callID string, raw json.RawMessage) (json.RawMessage, bool) {
			if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) < 45*time.Second {
				toolErr = context.DeadlineExceeded
				return []byte(`{"error":"probe time limit"}`), true
			}
			r := tools.Call(ctx, name, callID, raw)
			if r.Error == "OUTCOME_UNKNOWN" {
				toolErr = fmt.Errorf("outcome_unknown: %s", r.Detail)
				data, _ := json.Marshal(r)
				return data, true
			}
			if name == "artifact_submit" && r.Error == "" && r.Receipt != nil {
				artifact = r.Receipt.ID
			}
			data, _ := json.Marshal(r)
			boundary := partial && name == "work_checkpoint" && r.Error == "" && r.Receipt != nil
			return data, boundary
		})
		if e != nil {
			return "", cap, e
		}
		if toolErr != nil {
			return "", cap, toolErr
		}
		if outcome.Boundary {
			return "", cap, nil
		}
		if artifact != "" {
			return artifact, cap, nil
		}
		prompt = "No required persisted completion receipt was produced. Continue the assigned task using Polis tools; provide the required checkpoint"
		if !partial {
			prompt += " and artifact_submit"
		}
	}
	return "", cap, fmt.Errorf("profile completed without required persisted receipt")
}
