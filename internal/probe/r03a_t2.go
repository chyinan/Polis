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
	"polis/internal/kernel"
	"polis/internal/runner"
	"strings"
	"time"
)

const R03AT2ProblemKey = R03AProblemKey
const R03AT2SubjectRevision = R03ASubjectRevision

type R03AT2Config struct {
	Config
	ProblemKey               string
	T1Evidence               string
	OldEvidence              string
	RunPurpose               string
	AuthorizationBindingPath string
	ExecutionFingerprint     string
}

type R03AT2Result struct {
	Status                 string                              `json:"status"`
	ProblemKey             string                              `json:"problem_key"`
	SubjectRevision        string                              `json:"subject_revision"`
	Model                  string                              `json:"model"`
	ExecutionFingerprint   string                              `json:"execution_fingerprint"`
	Qualification          *R03AT2QualificationReport          `json:"qualification_revalidation,omitempty"`
	AuthorizationBinding   *codex.BusinessAuthorizationBinding `json:"authorization_binding,omitempty"`
	BlobDurability         *R03AT2BlobDurabilityQualification  `json:"blob_durability_qualification,omitempty"`
	Started                time.Time                           `json:"started"`
	Finished               time.Time                           `json:"finished"`
	MediumStarted          int                                 `json:"medium_started"`
	HighStarted            int                                 `json:"high_started"`
	Turn                   *R03ATurn                           `json:"turn,omitempty"`
	TransportState         string                              `json:"transport_state"`
	BackendRealExecution   string                              `json:"backend_real_execution"`
	ContractRevisionV2     string                              `json:"contract_revision_v2"`
	PeerMessage            string                              `json:"peer_message"`
	Obligation             string                              `json:"obligation"`
	PlannerRelay           string                              `json:"planner_relay"`
	BackendCandidate       string                              `json:"backend_candidate"`
	BackendArtifactID      string                              `json:"backend_artifact_id,omitempty"`
	BackendArtifactDigest  string                              `json:"backend_artifact_digest,omitempty"`
	BackendWorkspaceDigest string                              `json:"backend_workspace_digest,omitempty"`
	MessageID              string                              `json:"message_id,omitempty"`
	ObligationID           string                              `json:"obligation_id,omitempty"`
	ContractRevisionID     string                              `json:"contract_revision_id,omitempty"`
	UnresolvedSideEffect   string                              `json:"unresolved_side_effect"`
	Error                  string                              `json:"error,omitempty"`
}

func InspectR03AT2(cfg R03AT2Config) error {
	if cfg.Model != "gpt-5.6-luna" || cfg.MediumLimit != 1 || cfg.HighLimit != 0 || cfg.ToolCallLimit < 1 {
		return errors.New("R0.3A-T2 requires exactly one Medium, zero High, and an explicit tool-call limit")
	}
	if cfg.ProblemKey == "" {
		cfg.ProblemKey = R03AT2ProblemKey
	}
	if cfg.RunPurpose == "" {
		cfg.RunPurpose = "real_backend_peer_collaboration"
	}
	if cfg.OldEvidence == "" {
		cfg.OldEvidence = filepath.Join("evidence", "development", "r0.3a-real", "luna-1")
	}
	if cfg.T1Evidence == "" {
		cfg.T1Evidence = filepath.Join("evidence", "development", "r0.3a-t1")
	}
	if _, e := os.Stat(filepath.Join(cfg.Evidence, "preflight.json")); e == nil {
		return errors.New("R0.3A-T2 preflight already exists; refusing automatic rerun")
	}
	if e := assertT1AndHistory(cfg); e != nil {
		return e
	}
	qualification, e := RequireR03AT2Qualification(cfg)
	if e != nil {
		return e
	}
	if _, e := os.Stat(filepath.Join(cfg.Evidence, "allowance.json")); e == nil {
		return errors.New("R0.3A-T2 allowance already exists")
	}
	if e := os.MkdirAll(cfg.Evidence, 0700); e != nil {
		return e
	}
	return writeJSON(filepath.Join(cfg.Evidence, "preflight.json"), map[string]any{
		"passed":                     true,
		"problem_key":                cfg.ProblemKey,
		"parent_problem_key":         R03AProblemKey,
		"t1":                         "passed",
		"old_allowance":              "sealed",
		"model_calls":                0,
		"medium_limit":               1,
		"high_limit":                 0,
		"tool_call_limit":            cfg.ToolCallLimit,
		"qualification_revalidation": qualification,
	})
}

func RecordR03AT2ContinuationFreshness(cfg R03AT2Config, orchestrationRevision string) (R03AT2QualificationReport, error) {
	var empty R03AT2QualificationReport
	path := filepath.Join(cfg.Evidence, "continuation-freshness.json")
	if _, err := os.Stat(path); err == nil {
		return empty, errors.New("R0.3A continuation freshness already exists; refusing rerun")
	}
	preflightRaw, err := os.ReadFile(filepath.Join(cfg.Evidence, "preflight.json"))
	if err != nil {
		return empty, errors.New("R0.3A continuation requires the preserved preflight")
	}
	var preflight struct {
		Passed        bool   `json:"passed"`
		ProblemKey    string `json:"problem_key"`
		ToolCallLimit int    `json:"tool_call_limit"`
	}
	if err := json.Unmarshal(preflightRaw, &preflight); err != nil || !preflight.Passed || preflight.ProblemKey != cfg.ProblemKey || preflight.ToolCallLimit != cfg.ToolCallLimit {
		return empty, errors.New("R0.3A preserved preflight is invalid or belongs to another ProblemKey")
	}
	if orchestrationRevision == "" {
		orchestrationRevision = "r0.3a-real-backend-windows-orchestration-v2"
	}
	report, err := RequireR03AT2Qualification(cfg)
	if err != nil {
		return empty, err
	}
	blobQualification, err := RequireR03AT2BlobDurabilityQualification(cfg.BlobDurabilityQualificationPath)
	if err != nil {
		return empty, err
	}
	binding, err := LoadBusinessAuthorization(cfg.AuthorizationBindingPath, r03aT2BusinessAuthorizationContext(cfg))
	if err != nil {
		return empty, err
	}
	record := map[string]any{"record_type": "r0.3a-real-backend-continuation-freshness-v1", "orchestration_revision": orchestrationRevision, "previous_attempt": map[string]any{"backend": "NOT_STARTED", "allowance_created": false, "medium": 0, "high": 0, "provider_egress": 0, "worker": "NOT_STARTED"}, "current": report, "authorization_binding": binding, "blob_durability_qualification": blobQualification, "allowance_created": false, "worker_started": false, "provider_egress": 0, "historical_preflight_overwritten": false, "recorded_at": time.Now().UTC()}
	if err := writeJSON(path, record); err != nil {
		return empty, err
	}
	return report, nil
}

func RunR03AT2(cfg R03AT2Config) (result R03AT2Result, err error) {
	if cfg.ToolCallLimit < 1 {
		return result, errors.New("R0.3A-T2 requires an explicitly pre-registered tool-call limit")
	}
	if cfg.ProblemKey == "" {
		cfg.ProblemKey = R03AT2ProblemKey
	}
	if cfg.RunPurpose == "" {
		cfg.RunPurpose = "real_backend_peer_collaboration"
	}
	result = R03AT2Result{Status: "inconclusive", ProblemKey: cfg.ProblemKey, SubjectRevision: R03AT2SubjectRevision, Model: cfg.Model, TransportState: "not_run", BackendRealExecution: "not_run", ContractRevisionV2: "not_run", PeerMessage: "not_run", Obligation: "not_run", PlannerRelay: "not_run", BackendCandidate: "not_run", UnresolvedSideEffect: "none"}
	if e := os.MkdirAll(cfg.Evidence, 0700); e != nil {
		return result, e
	}
	defer func() {
		result.Finished = time.Now().UTC()
		if err != nil {
			result.Error = err.Error()
		}
		_ = writeJSON(filepath.Join(cfg.Evidence, "result.json"), result)
	}()
	preflightRaw, e := os.ReadFile(filepath.Join(cfg.Evidence, "preflight.json"))
	if e != nil {
		return result, e
	}
	var preflight struct {
		Passed           bool   `json:"passed"`
		ProblemKey       string `json:"problem_key"`
		ParentProblemKey string `json:"parent_problem_key"`
		ToolCallLimit    int    `json:"tool_call_limit"`
	}
	if e = json.Unmarshal(preflightRaw, &preflight); e != nil || !preflight.Passed || preflight.ProblemKey != cfg.ProblemKey || preflight.ParentProblemKey != R03AProblemKey || preflight.ToolCallLimit != cfg.ToolCallLimit {
		return result, errors.New("R0.3A-T2 preflight failed or stale")
	}
	qualification, e := RequireR03AT2Qualification(cfg)
	if e != nil {
		return result, e
	}
	result.ExecutionFingerprint = qualification.ExecutionFingerprint
	result.Qualification = &qualification
	blobQualification, e := RequireR03AT2BlobDurabilityQualification(cfg.BlobDurabilityQualificationPath)
	if e != nil {
		return result, e
	}
	if e = VerifyR03AT2BlobDurabilityFreshness(filepath.Join(cfg.Evidence, "continuation-freshness.json"), blobQualification); e != nil {
		return result, e
	}
	result.BlobDurability = &blobQualification
	cfg.CapabilityDigest = qualification.CapabilityDigest
	if cfg.ExpectedNativeVersion == "" {
		cfg.ExpectedNativeVersion = qualification.CodexVersion
	}
	if e = writeJSON(filepath.Join(cfg.Evidence, "qualification-revalidation.json"), qualification); e != nil {
		return result, e
	}
	runtimeCtx, runtimeCancel := context.WithTimeout(context.Background(), 30*time.Second)
	k, e := kernel.Open(runtimeCtx, cfg.DSN, filepath.Join(cfg.Root, "blobs"))
	runtimeCancel()
	if e != nil {
		return result, e
	}
	defer k.Close()
	if e = writeJSON(filepath.Join(cfg.Evidence, "runtime-connection.json"), map[string]any{"status": "passed", "database_scope": "dedicated_r0_3a_disposable", "allowance_created": false, "worker_started": false, "provider_egress": 0, "verified_at": time.Now().UTC()}); e != nil {
		return result, e
	}
	binding, e := LoadBusinessAuthorization(cfg.AuthorizationBindingPath, r03aT2BusinessAuthorizationContext(cfg))
	if e != nil {
		return result, e
	}
	result.AuthorizationBinding = &binding
	if e = writeJSON(filepath.Join(cfg.Evidence, "authorization-binding.json"), binding); e != nil {
		return result, e
	}
	if e = writeJSON(filepath.Join(cfg.Evidence, "problem-key.json"), map[string]any{"problem_key": cfg.ProblemKey, "parent_problem_key": R03AProblemKey, "subject_revision": R03AT2SubjectRevision, "new_allowance": true}); e != nil {
		return result, e
	}
	budget, e := codex.NewBusinessBudget(filepath.Join(cfg.Evidence, "allowance.json"), 1, 0, cfg.ToolCallLimit)
	if e != nil {
		return result, e
	}
	result.Started = budget.Started
	ctx, cancel := context.WithDeadline(context.Background(), budget.Started.Add(10*time.Minute))
	defer cancel()
	scope, e := k.TXCreateCompany(ctx, "r03a-t2-company-"+fmt.Sprint(budget.Started.UnixNano()))
	if e != nil {
		return result, e
	}
	mission := "r03a-t2-mission-" + fmt.Sprint(budget.Started.UnixNano())
	fx, e := k.TXCreatePeerFixture(ctx, scope, mission)
	if e != nil {
		return result, e
	}
	backend, e := k.TXNewWorkerWithToolBudget(ctx, scope, fx.Backend.ID, cfg.Model+"/medium", int64(cfg.ToolCallLimit))
	if e != nil {
		return result, e
	}
	if e = budget.Reserve("medium"); e != nil {
		return result, e
	}
	result.MediumStarted = budget.Medium
	session, sessionErr := runPeerSession(ctx, R03AConfig{Config: cfg.Config, ProblemKey: cfg.ProblemKey}, k, backend, "peer_backend", backendPrompt(), false, false, 1, cfg.RunPurpose)
	result.Turn = &session.turn
	if sessionErr != nil {
		var reconnectDeadline codex.ReconnectDeadlineError
		var firstOutputDeadline codex.FirstOutputDeadlineError
		if errors.As(sessionErr, &reconnectDeadline) || errors.As(sessionErr, &firstOutputDeadline) || strings.Contains(sessionErr.Error(), "inconclusive") || strings.Contains(sessionErr.Error(), "outcome_unknown") || strings.Contains(sessionErr.Error(), "reconciliation") {
			result.Status = "inconclusive"
			result.BackendRealExecution = "inconclusive"
			if errors.As(sessionErr, &reconnectDeadline) {
				result.TransportState = "reconnect_deadline_exceeded"
			} else if errors.As(sessionErr, &firstOutputDeadline) {
				result.TransportState = "first_valid_output_deadline_exceeded"
			} else {
				result.TransportState = "outcome_requiring_reconciliation"
			}
		} else {
			result.Status = "failed"
			result.BackendRealExecution = "failed"
			result.TransportState = "terminally_reconciled"
		}
		if turnHas(session.turn, "workspace_replace") || turnHas(session.turn, "collab_send") || turnHas(session.turn, "artifact_submit") {
			result.UnresolvedSideEffect = "possible_side_effect_requires_reconciliation"
		}
		return result, sessionErr
	}
	result.TransportState = "terminally_reconciled"
	result.BackendRealExecution = "passed"
	result.ContractRevisionID = session.turn.ContractRevisionID
	result.MessageID = session.turn.MessageID
	result.ObligationID = session.turn.ObligationID
	result.BackendArtifactID = session.turn.ArtifactID
	if result.ContractRevisionID == "" || result.MessageID == "" || result.ObligationID == "" || result.BackendArtifactID == "" || session.turn.CheckpointID == "" || !session.turn.StopConfirmed {
		result.BackendRealExecution = "failed"
		return result, errors.New("Backend did not produce all required persisted receipts")
	}
	contract, e := k.PeerContractAt(ctx, scope, result.ContractRevisionID)
	if e != nil {
		return result, e
	}
	// Current peer collaboration accepts any final revision >= 2. The
	// historical continuation evaluator required ordinal 2; that ordinal is
	// not a business invariant and must not reject a later accepted revision.
	result.ContractRevisionV2 = statusIf(contract.Revision >= 2 && contract.State == "accepted", "passed")
	message, _, obligationState, e := k.PeerMessageAt(ctx, scope, result.MessageID)
	if e != nil {
		return result, e
	}
	result.PeerMessage = statusIf(message.DeliveryState == "persisted", "passed")
	result.Obligation = statusIf(obligationState == "pending", "passed")
	planner, e := k.PeerPlannerPathCount(ctx, scope)
	if e != nil {
		return result, e
	}
	result.PlannerRelay = statusIf(planner == 0, "false")
	artifact, e := k.PeerArtifactAt(ctx, scope, result.BackendArtifactID)
	if e != nil {
		return result, e
	}
	result.BackendArtifactDigest = artifact.Digest
	result.BackendCandidate = statusIf(artifact.State == "ready" && artifact.Verdict == "candidate", "frozen")
	workspace, e := k.PeerWorkspaceAt(ctx, scope, fx.Backend.ID)
	if e != nil {
		return result, e
	}
	result.BackendWorkspaceDigest = workspace.Digest
	_ = writeJSON(filepath.Join(cfg.Evidence, "backend-state.json"), map[string]any{"contract": contract, "message": message, "obligation_state": obligationState, "artifact": artifact, "workspace": workspace, "planner_path": planner, "stop_confirmed": session.turn.StopConfirmed})
	if result.ContractRevisionV2 == "passed" && result.PeerMessage == "passed" && result.Obligation == "passed" && result.PlannerRelay == "false" && result.BackendCandidate == "frozen" {
		result.Status = "passed"
		return result, nil
	}
	result.Status = "failed"
	return result, errors.New("backend-only qualification conditions failed")
}

func r03aT2BusinessAuthorizationContext(cfg R03AT2Config) codex.BusinessExecutionContext {
	return codex.BusinessExecutionContext{
		ExecutionFingerprint:    cfg.ExecutionFingerprint,
		EmployeeID:              "emp-backend",
		ProblemKey:              cfg.ProblemKey,
		Purpose:                 cfg.RunPurpose,
		Model:                   cfg.Model,
		Profile:                 cfg.Model + "/medium",
		Effort:                  "medium",
		Limits:                  codex.AllowanceLimits{Medium: cfg.MediumLimit, High: cfg.HighLimit, Concurrency: 1, ToolCallLimit: cfg.ToolCallLimit},
		TransportPolicyRevision: cfg.TransportPolicy.Revision,
		TransportPolicy:         cfg.TransportPolicy.Snapshot(),
	}
}

func assertT1AndHistory(cfg R03AT2Config) error {
	t1, e := os.ReadFile(filepath.Join(cfg.T1Evidence, "qualification.md"))
	if e != nil || !strings.Contains(string(t1), "Status: `PASSED`") {
		return errors.New("R0.3A-T1 qualification is not passed")
	}
	if _, e = os.Stat(filepath.Join("internal", "codex", "testdata", "r03a_t1_disconnect_protocol.jsonl")); e != nil {
		return errors.New("T1 disconnect fixture is missing")
	}
	oldAllowance, e := os.ReadFile(filepath.Join(cfg.OldEvidence, "allowance.json"))
	if e != nil || !strings.Contains(string(oldAllowance), `"medium_turns":1`) || !strings.Contains(string(oldAllowance), `"high_turns":0`) {
		return errors.New("old R0.3A allowance is not sealed at one Medium and zero High")
	}
	oldKey, e := os.ReadFile(filepath.Join(cfg.OldEvidence, "problem-key.json"))
	if e != nil || !strings.Contains(string(oldKey), R03AProblemKey) {
		return errors.New("parent ProblemKey changed")
	}
	raw, e := runner.Run([]string{"git", "rev-parse", "HEAD"}, nil, 5*time.Second)
	if e != nil || strings.TrimSpace(string(raw)) != R03ASubjectRevision {
		return errors.New("subject revision drift")
	}
	return nil
}

func sha256File(path string) (string, error) {
	raw, e := os.ReadFile(path)
	if e != nil {
		return "", e
	}
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:]), nil
}
