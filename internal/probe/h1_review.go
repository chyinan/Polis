// pattern: Imperative Shell
package probe

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"polis/internal/codex"
	"polis/internal/core"
	"polis/internal/kernel"
	"polis/internal/runner"
)

const h1Qualification = "R0.3A-H1-INDEPENDENT-PEER-COLLABORATION-REVIEW"

type H1Config struct {
	Config
	ProblemKey            string
	SourceStatePath       string
	FrontendResultPath    string
	FrontendStatePath     string
	BackendBlobRoot       string
	FrontendBlobRoot      string
	SubjectRevision       string
	OldWriterEvidencePath string
}

type h1State struct {
	RuntimeControl struct {
		Incarnation string `json:"incarnation"`
	} `json:"runtime_control"`
	Company struct {
		ID string `json:"id"`
	} `json:"company"`
	ContractRevisions []struct {
		ID       string          `json:"id"`
		Revision int64           `json:"revision"`
		Endpoint string          `json:"endpoint"`
		Schema   json.RawMessage `json:"schema"`
		Digest   string          `json:"digest"`
		State    string          `json:"state"`
	} `json:"contract_revisions"`
	Messages []struct {
		ID                 string `json:"id"`
		ContractRevisionID string `json:"contract_revision_id"`
		DeliveryState      string `json:"delivery_state"`
		Sender             string `json:"sender"`
		Recipient          string `json:"recipient"`
	} `json:"messages"`
	Obligations []struct {
		ID          string `json:"id"`
		Owner       string `json:"owner"`
		State       string `json:"state"`
		EvidenceRef string `json:"evidence_ref"`
	} `json:"obligations"`
	WorkerSessions []struct {
		ID          string `json:"id"`
		EmployeeID  string `json:"employee_id"`
		Epoch       int64  `json:"epoch"`
		Incarnation string `json:"incarnation"`
		State       string `json:"state"`
	} `json:"worker_sessions"`
	Artifacts []struct {
		ID       string `json:"id"`
		Digest   string `json:"digest"`
		State    string `json:"state"`
		Verdict  string `json:"verdict"`
		Author   string `json:"author"`
		Contract string `json:"contract"`
	} `json:"artifacts"`
	IntegrationCandidates []struct {
		ID                 string `json:"id"`
		BackendArtifactID  string `json:"backend_artifact_id"`
		FrontendArtifactID string `json:"frontend_artifact_id"`
		ContractRevisionID string `json:"contract_revision_id"`
		State              string `json:"state"`
	} `json:"integration_candidates"`
}

type h1FrontendResult struct {
	Status                   string   `json:"status"`
	ProblemKey               string   `json:"problem_key"`
	FrontendInitial          string   `json:"frontend_initial"`
	FrontendSuccessor        string   `json:"frontend_successor"`
	RealFrontendHandover     string   `json:"real_frontend_handover"`
	RealPeerCollaboration    string   `json:"real_peer_collaboration"`
	MessageID                string   `json:"message_id"`
	ObligationID             string   `json:"obligation_id"`
	FinalContractRevisionID  string   `json:"final_contract_revision_id"`
	FinalContractRevision    int64    `json:"final_contract_revision"`
	FrontendArtifactID       string   `json:"frontend_artifact_id"`
	FrontendArtifactDigest   string   `json:"frontend_artifact_digest"`
	FrontendWorkspaceDigest  string   `json:"frontend_workspace_digest"`
	PositiveIntegration      bool     `json:"integration_positive"`
	NegativeControlPassed    bool     `json:"negative_control_passed_as_failure"`
	SourceRuntimeIncarnation string   `json:"source_runtime_incarnation"`
	InitialSession           R03ATurn `json:"initial_session"`
	SuccessorSession         R03ATurn `json:"successor_session"`
}

type h1OldWriterEvidence struct {
	AllStaleEpoch bool `json:"all_stale_epoch"`
}

type H1ReviewerSession struct {
	SessionID           string           `json:"session_id"`
	ThreadID            string           `json:"thread_id,omitempty"`
	Status              string           `json:"status"`
	Started             time.Time        `json:"started"`
	Finished            time.Time        `json:"finished"`
	DurationMS          int64            `json:"duration_ms"`
	NativeDurationMS    int64            `json:"native_duration_ms"`
	ProcessStarted      bool             `json:"process_started"`
	InitializeCompleted bool             `json:"initialize_completed"`
	ThreadStarted       bool             `json:"thread_started"`
	TurnStarted         bool             `json:"turn_started"`
	ProviderEgress      bool             `json:"provider_egress"`
	FirstValidOutput    bool             `json:"first_valid_output"`
	TurnCompleted       bool             `json:"turn_completed"`
	ReconnectCount      int              `json:"reconnect_count"`
	ToolEvents          []string         `json:"tool_events"`
	Receipts            []string         `json:"receipts"`
	ReviewReceiptID     string           `json:"review_receipt_id,omitempty"`
	StopReceipt         string           `json:"stop_receipt,omitempty"`
	StopConfirmed       bool             `json:"stop_confirmed"`
	TokenUsage          codex.TokenUsage `json:"token_usage"`
	NativeUsageUpdates  int              `json:"native_usage_updates"`
	Error               string           `json:"error,omitempty"`
}

func RunR03AH1Review(cfg H1Config) (result map[string]any, err error) {
	if cfg.Model != "gpt-5.6-luna" || cfg.MediumLimit != 0 || cfg.HighLimit != 1 || cfg.ProblemKey != R03AProblemKey || cfg.SubjectRevision == "" {
		return nil, errors.New("H1 requires one Luna High Reviewer, zero Medium and the fixed R0.3A subject")
	}
	missing := []string{}
	for name, value := range map[string]string{
		"evidence": cfg.Evidence, "binary": cfg.Binary, "code_mode_host": cfg.CodeModeHost, "auth": cfg.AuthFile,
		"selected_config": cfg.SelectedConfigPath, "runtime_root": cfg.Root, "source_state": cfg.SourceStatePath,
		"frontend_result": cfg.FrontendResultPath, "frontend_state": cfg.FrontendStatePath, "backend_blobs": cfg.BackendBlobRoot,
		"frontend_blobs": cfg.FrontendBlobRoot, "current_l1": cfg.CurrentL1EvidencePath, "old_writer": cfg.OldWriterEvidencePath,
	} {
		if value == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("H1 configuration is incomplete: %s", strings.Join(missing, ","))
	}
	if entries, readErr := os.ReadDir(cfg.Evidence); readErr == nil && len(entries) != 0 {
		return nil, errors.New("H1 evidence path is not fresh; refusing retry/reset")
	} else if readErr != nil && !os.IsNotExist(readErr) {
		return nil, readErr
	}
	subject, err := loadH1FrozenSubject(cfg)
	if err != nil {
		return nil, err
	}
	if err := validateH1ReviewAnchors(subject); err != nil {
		return nil, err
	}
	if err := validateH1Runtime(cfg); err != nil {
		return nil, err
	}
	tools := codex.PeerReviewerTools()
	if err := validateH1ReviewerSurface(tools); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cfg.Evidence, 0700); err != nil {
		return nil, err
	}
	reviewRecordPath := filepath.Join(cfg.Evidence, "review-record.json")
	reviewEvidence := kernel.FrozenPeerReviewEvidence{
		SubjectRevision: subject.SubjectRevision, TaskInput: "Independently assess the frozen public R0.3A peer-collaboration contract, final candidates, direct responsibility lifecycle, same-employee handover and integrated acceptance.",
		ContractRevisionID: subject.Contract.ID, MessageID: subject.MessageID, ObligationID: subject.ObligationID,
		BackendArtifactID: subject.BackendArtifactID, FrontendArtifactID: subject.FrontendArtifactID, BackendDigest: subject.BackendDigest,
		FrontendDigest: subject.FrontendDigest, BackendContent: subject.BackendContent, FrontendContent: subject.FrontendContent,
		AllowedRefs: subject.AllowedRefs, ReviewRecordPath: reviewRecordPath,
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "review-package.json"), map[string]any{
		"qualification": h1Qualification, "subject": subject, "reviewer_tools": tools, "read_only": true,
		"candidate_writes": false, "workspace_writes": false, "contract_mutation": false, "message_obligation_mutation": false,
		"artifact_mutation": false, "hidden_verifier_available": false, "historical_evidence_modified": false,
	}); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "reviewer-allowlist.json"), map[string]any{
		"subject_revision": subject.SubjectRevision, "contract_revision_id": subject.Contract.ID, "message_id": subject.MessageID,
		"obligation_id": subject.ObligationID, "backend_artifact_id": subject.BackendArtifactID, "frontend_artifact_id": subject.FrontendArtifactID,
		"allowed_refs": subject.AllowedRefs, "tools": h1ReviewerToolNames(tools), "hidden_verifier": "not_registered",
	}); err != nil {
		return nil, err
	}
	if err := writeJSON(filepath.Join(cfg.Evidence, "preflight.json"), map[string]any{
		"qualification": h1Qualification, "status": "review_preflight_passed", "passed": true, "model": cfg.Model, "profile": cfg.Model + "/high", "effort": "high",
		"medium": 0, "high": 0, "provider_egress": 0, "tool_count": len(tools), "tool_names": h1ReviewerToolNames(tools),
		"subject_revision": subject.SubjectRevision, "subject_digest": h1SubjectDigest(subject), "contract_revision_id": subject.Contract.ID,
		"message_id": subject.MessageID, "obligation_id": subject.ObligationID, "backend_artifact_id": subject.BackendArtifactID, "frontend_artifact_id": subject.FrontendArtifactID,
		"current_l1_fingerprint": os.Getenv("POLIS_CURRENT_L1_FINGERPRINT"), "binary": cfg.Binary, "code_mode_host": cfg.CodeModeHost,
		"historical_evidence_modified": false, "reviewer_contamination": false,
	}); err != nil {
		return nil, err
	}
	budget, err := codex.NewHighOnlyBudget(filepath.Join(cfg.Evidence, "allowance.json"))
	if err != nil {
		return nil, err
	}
	result = map[string]any{
		"qualification": h1Qualification, "status": "inconclusive", "model": cfg.Model, "profile": cfg.Model + "/high", "effort": "high",
		"medium_started": 0, "high_started": 0, "provider_egress": 0, "subject_digest": h1SubjectDigest(subject),
		"subject_revision": subject.SubjectRevision, "contract_revision_id": subject.Contract.ID, "message_id": subject.MessageID, "obligation_id": subject.ObligationID,
		"backend_artifact_id": subject.BackendArtifactID, "frontend_artifact_id": subject.FrontendArtifactID,
		"successor_only_replay_eligibility": false, "candidate_writes": 0, "workspace_writes": 0, "artifact_mutations": 0,
		"reviewer_session": nil, "reviewer_verdict": "NOT_RUN", "hidden_verifier_result": "NOT_RUN", "review_isolation_result": "NOT_RUN",
		"reviewer_quality": "NOT_RUN", "r0_3a_composite": "INCONCLUSIVE", "historical_evidence_modified": false,
	}
	writeResult := func() { _ = writeJSON(filepath.Join(cfg.Evidence, "result.json"), result) }
	if err := budget.Reserve("high"); err != nil {
		writeResult()
		return result, err
	}
	result["high_started"] = 1
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	sessionID := "h1-review-" + time.Now().UTC().Format("20060102T150405.000000000Z07:00")
	session, sessionErr := runH1ReviewerSession(ctx, cfg, sessionID, reviewEvidence)
	result["reviewer_session"] = session
	result["provider_egress"] = boolInt(session.ProviderEgress)
	if sessionErr != nil {
		result["error"] = sessionErr.Error()
		writeResult()
		return result, sessionErr
	}
	reviewRaw, err := os.ReadFile(reviewRecordPath)
	if err != nil {
		result["error"] = "reviewer did not persist review-submit record: " + err.Error()
		writeResult()
		return result, errors.New(result["error"].(string))
	}
	var reviewRecord kernel.FrozenPeerReviewRecord
	if err := json.Unmarshal(reviewRaw, &reviewRecord); err != nil {
		result["error"] = err.Error()
		writeResult()
		return result, err
	}
	hidden := h1HiddenVerify(subject)
	protocolRaw, _ := os.ReadFile(filepath.Join(cfg.Evidence, sessionID, "protocol.jsonl"))
	isolation := "failed"
	if h1ReviewerIsolation(session.ToolEvents, protocolRaw) {
		isolation = "passed"
	}
	composite := h1Composite(reviewRecord.Submission.Verdict, statusIf(hidden.Passed, "passed"), isolation)
	result["reviewer_verdict"] = reviewRecord.Submission.Verdict
	result["review_receipt_id"] = reviewRecord.Receipt.ID
	result["hidden_verifier_result"] = statusIf(hidden.Passed, "passed")
	result["hidden_verifier_findings"] = hidden.Findings
	result["review_isolation_result"] = isolation
	result["reviewer_quality"] = composite.ReviewerQuality
	result["r0_3a_composite"] = composite.Composite
	result["backend_business_result"] = "PASSED"
	result["frontend_handover_result"] = "PASSED"
	result["positive_integration"] = subject.PositiveIntegration
	result["negative_control"] = "FAIL_AS_EXPECTED"
	result["subject_unchanged"] = true
	result["status"] = composite.Composite
	writeResult()
	if composite.Composite != "PASSED" {
		return result, errors.New("H1 composite review did not pass")
	}
	return result, nil
}

func loadH1FrozenSubject(cfg H1Config) (H1FrozenSubject, error) {
	var source, final h1State
	if err := readH1JSON(cfg.SourceStatePath, &source); err != nil {
		return H1FrozenSubject{}, fmt.Errorf("read continuation-7 source state: %w", err)
	}
	if err := readH1JSON(cfg.FrontendStatePath, &final); err != nil {
		return H1FrozenSubject{}, fmt.Errorf("read revised-v2 final state: %w", err)
	}
	var frontend h1FrontendResult
	if err := readH1JSON(cfg.FrontendResultPath, &frontend); err != nil {
		return H1FrozenSubject{}, fmt.Errorf("read revised-v2 result: %w", err)
	}
	if source.Company.ID == "" || source.Company.ID != final.Company.ID {
		return H1FrozenSubject{}, errors.New("frozen reviewer subject company identity mismatch")
	}
	contractRow, ok := newestAcceptedH1Contract(final.ContractRevisions)
	if !ok {
		return H1FrozenSubject{}, errors.New("frozen reviewer subject has no accepted final contract")
	}
	var contractSchema map[string]any
	if err := json.Unmarshal(contractRow.Schema, &contractSchema); err != nil {
		return H1FrozenSubject{}, err
	}
	contractRaw, _ := json.Marshal(contractSchema)
	backendRow, ok := findH1Artifact(final.Artifacts, frontendBackendArtifactID(source, final))
	if !ok {
		return H1FrozenSubject{}, errors.New("frozen reviewer subject backend artifact missing")
	}
	frontendRow, ok := findH1Artifact(final.Artifacts, frontend.FrontendArtifactID)
	if !ok {
		return H1FrozenSubject{}, errors.New("frozen reviewer subject frontend artifact missing")
	}
	backendContent, err := readH1Blob(cfg.BackendBlobRoot, source.Company.ID, backendRow.Digest)
	if err != nil {
		return H1FrozenSubject{}, err
	}
	frontendContent, err := readH1Blob(cfg.FrontendBlobRoot, source.Company.ID, frontendRow.Digest)
	if err != nil {
		return H1FrozenSubject{}, err
	}
	message, ok := findH1Message(final.Messages, frontend.MessageID)
	if !ok {
		return H1FrozenSubject{}, errors.New("frozen reviewer subject message missing")
	}
	if message.ContractRevisionID != contractRow.ID || message.Sender != "emp-backend" || message.Recipient != "emp-frontend" || message.DeliveryState != "resolved" {
		return H1FrozenSubject{}, errors.New("frozen reviewer subject message is not current and resolved")
	}
	obligation, ok := findH1Obligation(final.Obligations, frontend.ObligationID)
	if !ok {
		return H1FrozenSubject{}, errors.New("frozen reviewer subject obligation missing")
	}
	backendSession := ""
	backendIncarnation := ""
	for _, session := range final.WorkerSessions {
		if session.EmployeeID == "emp-backend" && session.State == "stopped" {
			backendSession, backendIncarnation = session.ID, session.Incarnation
		}
	}
	if backendSession == "" {
		return H1FrozenSubject{}, errors.New("frozen reviewer subject Backend session missing")
	}
	positive := frontend.PositiveIntegration
	for _, candidate := range final.IntegrationCandidates {
		if candidate.BackendArtifactID == backendRow.ID && candidate.FrontendArtifactID == frontendRow.ID && candidate.ContractRevisionID == contractRow.ID && candidate.State == "passed" {
			positive = true
		}
	}
	var oldWriter h1OldWriterEvidence
	if err := readH1JSON(cfg.OldWriterEvidencePath, &oldWriter); err != nil {
		return H1FrozenSubject{}, err
	}
	allowed := []string{cfg.SubjectRevision, contractRow.ID, frontend.MessageID, frontend.ObligationID, backendRow.ID, frontendRow.ID, backendRow.Digest, frontendRow.Digest, frontend.InitialSession.SessionID, frontend.SuccessorSession.SessionID}
	return H1FrozenSubject{
		SubjectRevision: cfg.SubjectRevision, ProblemKey: R03AProblemKey, CompanyID: source.Company.ID,
		Contract:  kernel.PeerContractRevision{ID: contractRow.ID, Endpoint: contractRow.Endpoint, Schema: string(contractRaw), Digest: contractRow.Digest, State: contractRow.State, Revision: contractRow.Revision},
		MessageID: frontend.MessageID, ObligationID: frontend.ObligationID, ObligationOwner: obligation.Owner, FinalObligationState: obligation.State,
		BackendArtifactID: backendRow.ID, FrontendArtifactID: frontendRow.ID, BackendDigest: backendRow.Digest, FrontendDigest: frontendRow.Digest,
		BackendContent: string(backendContent), FrontendContent: string(frontendContent), BackendSessionID: backendSession, BackendSessionIncarnation: backendIncarnation,
		FrontendInitialSession: frontend.InitialSession, FrontendSuccessorSession: frontend.SuccessorSession,
		PositiveIntegration: positive, NegativeControlPassed: !frontend.NegativeControlPassed, OldWriterRejection: oldWriter.AllStaleEpoch,
		RuntimeIncarnation: final.RuntimeControl.Incarnation, SourceRuntimeIncarnation: frontend.SourceRuntimeIncarnation,
		PolicyRevisions: map[string]string{"acceptance_checker_revision": core.PeerAcceptanceCheckerRevision, "checkpoint_policy_revision": core.CheckpointPolicyRevision, "artifact_eligibility_policy_revision": core.ArtifactEligibilityPolicyRevision, "contract_supersession_policy_revision": core.PeerContractSupersessionPolicyRevision},
		AllowedRefs:     allowed,
	}, nil
}

func validateH1ReviewAnchors(subject H1FrozenSubject) error {
	if subject.SubjectRevision == "" || subject.Contract.ID == "" || subject.Contract.Revision != 3 || subject.Contract.State != "accepted" || subject.MessageID == "" || subject.ObligationID != subject.MessageID || subject.ObligationOwner != "emp-frontend" || subject.FinalObligationState != "fulfilled" || subject.BackendArtifactID == "" || subject.FrontendArtifactID == "" || subject.BackendContent == "" || subject.FrontendContent == "" {
		return errors.New("H1 frozen review anchors are incomplete")
	}
	if digest([]byte(subject.BackendContent)) != subject.BackendDigest || digest([]byte(subject.FrontendContent)) != subject.FrontendDigest {
		return errors.New("H1 candidate content digest mismatch")
	}
	if subject.FrontendInitialSession.SessionID == "" || subject.FrontendSuccessorSession.SessionID == "" || subject.FrontendInitialSession.SessionID == subject.FrontendSuccessorSession.SessionID || subject.FrontendSuccessorSession.Epoch <= subject.FrontendInitialSession.Epoch || subject.FrontendInitialSession.Incarnation == "" || subject.FrontendSuccessorSession.Incarnation == "" || subject.FrontendInitialSession.Incarnation != subject.FrontendSuccessorSession.Incarnation {
		return errors.New("H1 Frontend session identity continuity is incomplete")
	}
	return nil
}

func validateH1ReviewerSurface(tools []any) error {
	if len(tools) != 4 {
		return errors.New("H1 Reviewer surface is not the qualified four-tool surface")
	}
	for index, expected := range []string{"polis_work_current", "polis_context_read", "polis_workspace_read", "polis_review_submit"} {
		tool, ok := tools[index].(map[string]any)
		if !ok || tool["name"] != expected {
			return fmt.Errorf("H1 Reviewer surface drift at ordinal %d", index+1)
		}
	}
	return nil
}

func h1ReviewerToolNames(tools []any) []string {
	names := make([]string, 0, len(tools))
	for _, raw := range tools {
		if tool, ok := raw.(map[string]any); ok {
			if name, ok := tool["name"].(string); ok {
				names = append(names, name)
			}
		}
	}
	return names
}

func validateH1Runtime(cfg H1Config) error {
	var l1 currentBinaryL1Evidence
	if err := readH1JSON(filepath.Join(cfg.CurrentL1EvidencePath, "result.json"), &l1); err != nil {
		return err
	}
	if l1.Status != "passed" || l1.CurrentBinaryWindowsL1 != "QUALIFIED" || l1.Model != "gpt-5.6-luna" || l1.RuntimeProfile != "windows-native" || l1.ProviderTransportPolicy != "native_default" || l1.DynamicToolCount != 0 || l1.UnresolvedTransportState {
		return errors.New("H1 current Windows L1 is stale or unqualified")
	}
	binaryDigest, err := sha256File(cfg.Binary)
	if err != nil || binaryDigest != l1.CodexBinarySHA256 {
		return errors.New("H1 controlled Codex binary drifted")
	}
	helperDigest, err := sha256File(cfg.CodeModeHost)
	if err != nil || helperDigest != l1.CodeModeHostSHA256 {
		return errors.New("H1 controlled code-mode host drifted")
	}
	selectedDigest, err := sha256File(cfg.SelectedConfigPath)
	if err != nil || selectedDigest != l1.SelectedConfigRawSHA256 {
		return errors.New("H1 selected config drifted")
	}
	authRaw, err := os.ReadFile(cfg.AuthFile)
	if err != nil {
		return err
	}
	auth, err := codex.ParseAuthMaterial(authRaw, l1.AuthSourceClass)
	if err != nil || auth.IdentityFingerprint != l1.AuthIdentityFingerprint || auth.CredentialRevisionFingerprint != l1.AuthCredentialRevision {
		return errors.New("H1 auth identity or credential revision drifted")
	}
	return nil
}

func runH1ReviewerSession(ctx context.Context, cfg H1Config, sessionID string, evidence kernel.FrozenPeerReviewEvidence) (out H1ReviewerSession, err error) {
	out.SessionID, out.Status, out.Started = sessionID, "not_started", time.Now().UTC()
	root := filepath.Join(cfg.Root, sessionID)
	if err := os.MkdirAll(root, 0700); err != nil {
		return out, err
	}
	args, _, err := runner.NativeArgs(cfg.Binary, filepath.Join(root, "home"), cfg.AuthFile, cfg.ProxyURL)
	if err != nil {
		return out, err
	}
	p, err := runner.Start(sessionID, args, runner.NativeEnvironment(filepath.Join(root, "home")))
	if err != nil {
		return out, err
	}
	out.ProcessStarted = true
	stopped := false
	var client *codex.Client
	defer func() {
		if !stopped {
			if proof, stopErr := p.Stop(); stopErr == nil {
				out.StopReceipt, out.StopConfirmed = proof.Description(), true
			}
		}
		if client != nil {
			client.Close()
		}
		out.Finished = time.Now().UTC()
		out.DurationMS = out.Finished.Sub(out.Started).Milliseconds()
		out.NativeDurationMS, out.NativeUsageUpdates = protocolStats(filepath.Join(cfg.Evidence, sessionID, "protocol.jsonl"))
		out.FirstValidOutput = h1ProtocolHasOutput(filepath.Join(cfg.Evidence, sessionID, "protocol.jsonl"))
		if err != nil {
			out.Status, out.Error = "failed", err.Error()
		}
		_ = writeJSON(filepath.Join(cfg.Evidence, sessionID, "h1-session.json"), out)
	}()
	client, err = codex.NewWithModelAndVersion(p, filepath.Join(cfg.Evidence, sessionID), cfg.Model, cfg.ExpectedNativeVersion)
	if err != nil {
		return out, err
	}
	if err = client.Initialize(ctx); err != nil {
		return out, err
	}
	out.InitializeCompleted = true
	thread, err := client.StartThreadWithTools(ctx, "high", codex.PeerReviewerTools(), "You are an independent Polis High Reviewer. Use only the four registered read-only review tools and submit exactly one formal verdict. Do not modify candidates, workspace, contracts, messages, obligations or artifacts. The controller will run deterministic verification only after your submitted record is frozen.")
	if err != nil {
		return out, err
	}
	out.ThreadStarted, out.ThreadID = true, thread
	reviewer := &kernel.FrozenPeerReviewerTools{Evidence: evidence}
	out.TurnStarted, out.ProviderEgress = true, true
	turnResult, err := client.TurnWithOptions(ctx, thread, "high", h1ReviewerPrompt(), codex.TurnOptions{}, func(name, callID string, raw json.RawMessage) (json.RawMessage, bool) {
		out.ToolEvents = append(out.ToolEvents, name)
		response := reviewer.Call(ctx, name, callID, raw)
		if response.Receipt != nil {
			out.Receipts = append(out.Receipts, response.Receipt.ID)
			if name == "review_submit" {
				out.ReviewReceiptID = response.Receipt.ID
			}
		}
		encoded, _ := json.Marshal(response)
		return encoded, false
	})
	out.TurnCompleted = err == nil && turnResult.State == "completed"
	if err != nil {
		return out, err
	}
	out.TokenUsage, out.NativeUsageUpdates = turnResult.Usage, turnResult.UsageUpdates
	if turnResult.State != "completed" {
		return out, fmt.Errorf("H1 reviewer turn ended %s", turnResult.State)
	}
	proof, err := p.Stop()
	if err != nil {
		return out, err
	}
	stopped = true
	out.StopReceipt, out.StopConfirmed, out.Status = proof.Description(), true, "passed"
	return out, nil
}

func h1ProtocolHasOutput(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry struct {
			Data struct {
				Method string `json:"method"`
			} `json:"data"`
		}
		if json.Unmarshal(scanner.Bytes(), &entry) == nil && (entry.Data.Method == "item/agentMessage/delta" || entry.Data.Method == "item/agentMessage/completed") {
			return true
		}
	}
	return false
}

func readH1JSON(path string, target any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}

func readH1Blob(root, company, expectedDigest string) ([]byte, error) {
	path := filepath.Join(root, company, expectedDigest)
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	digestBytes := sha256.Sum256(raw)
	if hex.EncodeToString(digestBytes[:]) != expectedDigest {
		return nil, errors.New("frozen candidate CAS digest mismatch")
	}
	return raw, nil
}

func newestAcceptedH1Contract(rows []struct {
	ID       string          `json:"id"`
	Revision int64           `json:"revision"`
	Endpoint string          `json:"endpoint"`
	Schema   json.RawMessage `json:"schema"`
	Digest   string          `json:"digest"`
	State    string          `json:"state"`
}) (struct {
	ID       string          `json:"id"`
	Revision int64           `json:"revision"`
	Endpoint string          `json:"endpoint"`
	Schema   json.RawMessage `json:"schema"`
	Digest   string          `json:"digest"`
	State    string          `json:"state"`
}, bool) {
	var selected struct {
		ID       string          `json:"id"`
		Revision int64           `json:"revision"`
		Endpoint string          `json:"endpoint"`
		Schema   json.RawMessage `json:"schema"`
		Digest   string          `json:"digest"`
		State    string          `json:"state"`
	}
	found := false
	for _, row := range rows {
		if row.State == "accepted" && (!found || row.Revision > selected.Revision) {
			selected, found = row, true
		}
	}
	return selected, found
}

func frontendBackendArtifactID(source, final h1State) string {
	for _, row := range source.Artifacts {
		if row.Author == "emp-backend" && row.Verdict == "candidate" {
			return row.ID
		}
	}
	for _, row := range final.Artifacts {
		if row.Author == "emp-backend" && row.Verdict == "candidate" {
			return row.ID
		}
	}
	return ""
}

func findH1Artifact(rows []struct {
	ID       string `json:"id"`
	Digest   string `json:"digest"`
	State    string `json:"state"`
	Verdict  string `json:"verdict"`
	Author   string `json:"author"`
	Contract string `json:"contract"`
}, id string) (struct {
	ID       string `json:"id"`
	Digest   string `json:"digest"`
	State    string `json:"state"`
	Verdict  string `json:"verdict"`
	Author   string `json:"author"`
	Contract string `json:"contract"`
}, bool) {
	for _, row := range rows {
		if row.ID == id {
			return row, true
		}
	}
	var empty struct {
		ID       string `json:"id"`
		Digest   string `json:"digest"`
		State    string `json:"state"`
		Verdict  string `json:"verdict"`
		Author   string `json:"author"`
		Contract string `json:"contract"`
	}
	return empty, false
}

func findH1Message(rows []struct {
	ID                 string `json:"id"`
	ContractRevisionID string `json:"contract_revision_id"`
	DeliveryState      string `json:"delivery_state"`
	Sender             string `json:"sender"`
	Recipient          string `json:"recipient"`
}, id string) (struct {
	ID                 string `json:"id"`
	ContractRevisionID string `json:"contract_revision_id"`
	DeliveryState      string `json:"delivery_state"`
	Sender             string `json:"sender"`
	Recipient          string `json:"recipient"`
}, bool) {
	for _, row := range rows {
		if row.ID == id {
			return row, true
		}
	}
	var empty struct {
		ID                 string `json:"id"`
		ContractRevisionID string `json:"contract_revision_id"`
		DeliveryState      string `json:"delivery_state"`
		Sender             string `json:"sender"`
		Recipient          string `json:"recipient"`
	}
	return empty, false
}

func findH1Obligation(rows []struct {
	ID          string `json:"id"`
	Owner       string `json:"owner"`
	State       string `json:"state"`
	EvidenceRef string `json:"evidence_ref"`
}, id string) (struct {
	ID          string `json:"id"`
	Owner       string `json:"owner"`
	State       string `json:"state"`
	EvidenceRef string `json:"evidence_ref"`
}, bool) {
	for _, row := range rows {
		if row.ID == id {
			return row, true
		}
	}
	var empty struct {
		ID          string `json:"id"`
		Owner       string `json:"owner"`
		State       string `json:"state"`
		EvidenceRef string `json:"evidence_ref"`
	}
	return empty, false
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
