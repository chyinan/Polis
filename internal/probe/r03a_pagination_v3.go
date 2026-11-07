// pattern: Imperative Shell
package probe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"polis/internal/codex"
	"polis/internal/core"
	"polis/internal/fixture"
	"polis/internal/kernel"
	"polis/internal/recovery"
	"reflect"
	"strings"
	"time"
)

const (
	R03APaginationV3SubjectRevision = "r03a-pagination-v3-business-subject@1"
	R03APaginationV3Purpose         = "real_backend_peer_collaboration"
	R03APaginationV3Orchestration   = "r03a-pagination-v3-orchestration@1"
)

type R03APaginationV3Config struct {
	Config
	ProblemKey                     string
	RunPurpose                     string
	AcceptanceQualificationPath    string
	BackendExecutionManifestPath   string
	BackendQualificationPath       string
	BackendL2LivePath              string
	FrontendExecutionManifestPath  string
	FrontendQualificationPath      string
	FrontendL2LivePath             string
	AuthorizationBindingPath       string
	FrontendBindingPath            string
	AcceptanceContractRevision     string
	BehaviorVerifierRevision       string
	AllowancePath                  string
	BackendResultPath              string
	FrontendResultPath             string
	PostgresSnapshotPath           string
	PostgresSnapshotDSN            string
	RestoreProofPath               string
	FrontendBaselineManifestPath   string
	FrontendBaselineManifestSHA256 string
	RuntimeCASBinding              kernel.RuntimeCASBinding
	FrontendRequiredCAS            []kernel.CASRequiredBlob
	FrontendCASManifestPath        string
	ExecutionEnvelopeFingerprint   string
}

type r03aPaginationV3Qualification struct {
	Acceptance map[string]any
	Backend    R03AT2QualificationReport
	Frontend   R03AT2QualificationReport
	Blob       R03AT2BlobDurabilityQualification
}

func validateR03APaginationV3Config(cfg R03APaginationV3Config) error {
	if cfg.ProblemKey != R03AProblemKey || cfg.RunPurpose != R03APaginationV3Purpose {
		return errors.New("R0.3A pagination V3 ProblemKey or purpose mismatch")
	}
	if cfg.Model != "gpt-5.6-luna" || cfg.MediumLimit != 3 || cfg.HighLimit != 0 || cfg.ToolCallLimit != 48 {
		return errors.New("R0.3A pagination V3 requires three Medium, zero High and tool_call_limit=48")
	}
	if cfg.Evidence == "" || cfg.Root == "" || cfg.AuthFile == "" || cfg.Binary == "" || cfg.CodeModeHost == "" || cfg.SelectedConfigPath == "" || cfg.AcceptanceQualificationPath == "" || cfg.BackendExecutionManifestPath == "" || cfg.BackendQualificationPath == "" || cfg.BackendL2LivePath == "" || cfg.FrontendExecutionManifestPath == "" || cfg.FrontendQualificationPath == "" || cfg.FrontendL2LivePath == "" || cfg.BlobDurabilityQualificationPath == "" || cfg.AuthorizationBindingPath == "" || cfg.FrontendBindingPath == "" || cfg.ExecutionEnvelopeFingerprint == "" {
		return errors.New("R0.3A pagination V3 configuration is incomplete")
	}
	if err := validateR03ATransportPolicy(cfg.TransportPolicy); err != nil {
		return err
	}
	return nil
}

func validateR03APaginationV3BackendOnlyConfig(cfg R03APaginationV3Config) error {
	if cfg.ProblemKey != R03AProblemKey || cfg.RunPurpose != R03APaginationV3Purpose {
		return errors.New("R0.3A pagination V3 Backend ProblemKey or purpose mismatch")
	}
	if cfg.Model != "gpt-5.6-luna" || cfg.MediumLimit != 1 || cfg.HighLimit != 0 || cfg.ToolCallLimit != 48 {
		return errors.New("R0.3A pagination V3 Backend requires one Medium, zero High and tool_call_limit=48")
	}
	if cfg.Evidence == "" || cfg.Root == "" || cfg.AuthFile == "" || cfg.Binary == "" || cfg.CodeModeHost == "" || cfg.SelectedConfigPath == "" || cfg.AcceptanceQualificationPath == "" || cfg.BackendExecutionManifestPath == "" || cfg.BackendQualificationPath == "" || cfg.BackendL2LivePath == "" || cfg.BlobDurabilityQualificationPath == "" || cfg.AuthorizationBindingPath == "" || cfg.AllowancePath == "" || cfg.BackendResultPath == "" || cfg.PostgresSnapshotPath == "" || cfg.ExecutionEnvelopeFingerprint == "" {
		return errors.New("R0.3A pagination V3 Backend configuration is incomplete")
	}
	if err := validateR03ATransportPolicy(cfg.TransportPolicy); err != nil {
		return err
	}
	return nil
}

func validateR03ATransportPolicy(policy codex.TransportPolicy) error {
	if err := policy.Validate(); err != nil {
		return fmt.Errorf("transport policy binding is invalid: %w", err)
	}
	if policy.Revision != codex.TransportPolicyRevision {
		return fmt.Errorf("transport policy revision mismatch: expected %s got %s", codex.TransportPolicyRevision, policy.Revision)
	}
	if policy != codex.DefaultTransportPolicy() {
		return errors.New("transport policy values do not match the registered effective policy")
	}
	return nil
}

func loadR03APaginationV3Acceptance(path string) (map[string]any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("pagination acceptance qualification unavailable: %w", err)
	}
	var record map[string]any
	if err := json.Unmarshal(raw, &record); err != nil {
		return nil, fmt.Errorf("pagination acceptance qualification is invalid: %w", err)
	}
	qualification := stringValue(record, "qualification")
	if qualification != "R0.3A-PAGINATION-ACCEPTANCE-REMEDIATION" && qualification != "R0.3A-PUBLIC-BINDING-CONTRACT-HARDENING" && qualification != "R0.3A-PUBLIC-RESPONSE-ENCODING-CONTRACT-HARDENING" || stringValue(record, "status") != "PASSED" || stringValue(record, "acceptance_remediation") != "PASSED" || stringValue(record, "eligible_for_new_peer_collaboration_run") != "YES" {
		return nil, errors.New("pagination acceptance remediation is not qualified")
	}
	future, ok := record["future_public_contract"].(map[string]any)
	if !ok || stringValue(future, "revision") != fixture.PeerPaginationContractRevision || stringValue(future, "sha256") == "" {
		return nil, errors.New("future public pagination contract binding is incomplete")
	}
	verifier, ok := record["behavioral_verifier"].(map[string]any)
	if !ok || stringValue(verifier, "revision") != fixture.PeerPaginationBehaviorVerifierRevisionV2 || stringValue(verifier, "source_digest") == "" {
		return nil, errors.New("pagination behavior verifier binding is incomplete")
	}
	if stringValue(record, "implementation_binding_contract") != fixture.BackendBindingContractRevisionV2 {
		return nil, errors.New("public backend binding contract is missing or stale")
	}
	binding, bindingOK := record["public_binding_contract"].(map[string]any)
	if !bindingOK || stringValue(binding, "revision") != fixture.BackendBindingContractRevisionV2 || stringValue(binding, "return_type") != "string" || stringValue(binding, "return_representation") != "utf8_json" || stringValue(binding, "return_schema_ref") != "r03a-pagination-contract@3#response" {
		return nil, errors.New("public response representation bridge is missing or incoherent")
	}
	if stringValue(verifier, "business_contract") != fixture.PeerPaginationContractRevision || stringValue(verifier, "implementation_binding") != fixture.BackendBindingContractRevisionV2 {
		return nil, errors.New("pagination behavior verifier does not bind the public business and implementation contracts")
	}
	return record, nil
}

func RequireR03APaginationV3Qualification(cfg R03APaginationV3Config) (r03aPaginationV3Qualification, error) {
	var qualification r03aPaginationV3Qualification
	acceptance, err := loadR03APaginationV3Acceptance(cfg.AcceptanceQualificationPath)
	if err != nil {
		return qualification, err
	}
	backendFingerprint, err := executionFingerprintFromManifest(cfg.BackendExecutionManifestPath)
	if err != nil {
		return qualification, err
	}
	backendCfg := R03AT2Config{Config: cfg.Config, ProblemKey: cfg.ProblemKey, RunPurpose: cfg.RunPurpose, AuthorizationBindingPath: cfg.AuthorizationBindingPath, ExecutionFingerprint: backendFingerprint}
	backendCfg.MediumLimit, backendCfg.HighLimit, backendCfg.ToolCallLimit = cfg.MediumLimit, cfg.HighLimit, cfg.ToolCallLimit
	backendCfg.ExecutionManifestPath, backendCfg.QualificationPath = cfg.BackendExecutionManifestPath, cfg.BackendL2LivePath
	backend, err := requireV3BackendL2Qualification(backendCfg)
	if err != nil {
		return qualification, err
	}
	frontendFingerprint, err := executionFingerprintFromManifest(cfg.FrontendExecutionManifestPath)
	if err != nil {
		return qualification, err
	}
	frontendCfg := backendCfg
	frontendCfg.ExecutionFingerprint = frontendFingerprint
	frontendCfg.ExecutionManifestPath, frontendCfg.QualificationPath = cfg.FrontendExecutionManifestPath, cfg.FrontendL2LivePath
	frontendCfg.CheckerFeedbackQualificationPath = cfg.CheckerFeedbackQualificationPath
	frontend, err := requireCurrentBinaryRevisedFrontendL2Qualification(frontendCfg)
	if err != nil {
		return qualification, err
	}
	blob, err := RequireR03AT2BlobDurabilityQualification(cfg.BlobDurabilityQualificationPath)
	if err != nil {
		return qualification, err
	}
	if blob.Status != "PASSED" || blob.MediumConsumed != 0 || blob.HighConsumed != 0 || blob.ProviderEgress != 0 {
		return qualification, errors.New("blob qualification is not zero-egress passed evidence")
	}
	qualification = r03aPaginationV3Qualification{Acceptance: acceptance, Backend: backend, Frontend: frontend, Blob: blob}
	return qualification, nil
}

func requireR03APaginationV3BackendOnlyQualification(cfg R03APaginationV3Config) (fixtureAcceptance map[string]any, backend R03AT2QualificationReport, blob R03AT2BlobDurabilityQualification, err error) {
	fixtureAcceptance, err = loadR03APaginationV3Acceptance(cfg.AcceptanceQualificationPath)
	if err != nil {
		return nil, backend, blob, err
	}
	backendFingerprint, err := executionFingerprintFromManifest(cfg.BackendExecutionManifestPath)
	if err != nil {
		return nil, backend, blob, err
	}
	backendCfg := R03AT2Config{Config: cfg.Config, ProblemKey: cfg.ProblemKey, RunPurpose: cfg.RunPurpose, AuthorizationBindingPath: cfg.AuthorizationBindingPath, ExecutionFingerprint: backendFingerprint}
	backendCfg.MediumLimit, backendCfg.HighLimit, backendCfg.ToolCallLimit = cfg.MediumLimit, cfg.HighLimit, cfg.ToolCallLimit
	backendCfg.ExecutionManifestPath, backendCfg.QualificationPath = cfg.BackendExecutionManifestPath, cfg.BackendL2LivePath
	backend, err = requireV3BackendL2Qualification(backendCfg)
	if err != nil {
		return nil, backend, blob, err
	}
	blob, err = RequireR03AT2BlobDurabilityQualification(cfg.BlobDurabilityQualificationPath)
	if err != nil {
		return nil, backend, blob, err
	}
	if blob.Status != "PASSED" || blob.MediumConsumed != 0 || blob.HighConsumed != 0 || blob.ProviderEgress != 0 {
		return nil, backend, blob, errors.New("blob qualification is not zero-egress passed evidence")
	}
	return fixtureAcceptance, backend, blob, nil
}

func InspectR03APaginationV3BackendOnly(cfg R03APaginationV3Config) error {
	if err := validateR03APaginationV3BackendOnlyConfig(cfg); err != nil {
		return err
	}
	if entries, err := os.ReadDir(cfg.Evidence); err == nil && len(entries) != 0 {
		return errors.New("pagination V3 Backend evidence path is not fresh; refusing retry/reset")
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	if _, err := os.Stat(cfg.AllowancePath); err == nil {
		return errors.New("pagination V3 Backend allowance already exists before preflight")
	}
	acceptance, backend, blob, err := requireR03APaginationV3BackendOnlyQualification(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.Evidence, 0700); err != nil {
		return err
	}
	binding := paginationV3BackendBusinessBinding(cfg.ExecutionEnvelopeFingerprint, cfg)
	return writeJSON(filepath.Join(cfg.Evidence, "preflight.json"), map[string]any{
		"qualification":                   "R0.3A-PAGINATION-V3-REAL-BACKEND",
		"status":                          "preflight_passed",
		"passed":                          true,
		"parent_problem_key":              cfg.ProblemKey,
		"subject_revision":                R03APaginationV3SubjectRevision,
		"purpose":                         cfg.RunPurpose,
		"contract_revision":               fixture.PeerPaginationContractRevision,
		"behavior_verifier":               fixture.PeerPaginationBehaviorVerifierRevisionV2,
		"implementation_binding_contract": fixture.BackendBindingContractRevisionV2,
		"backend_l2_fingerprint":          backend.ExecutionFingerprint,
		"backend_l2_manifest":             cfg.BackendExecutionManifestPath,
		"acceptance_qualification":        acceptance,
		"blob_qualification":              blob,
		"authorization_binding":           binding,
		"medium_limit":                    1,
		"high_limit":                      0,
		"tool_call_limit":                 48,
		"allowance_created":               false,
		"worker_started":                  false,
		"provider_egress":                 0,
		"historical_evidence_modified":    false,
	})
}

func RecordR03APaginationV3BackendOnlyFreshness(cfg R03APaginationV3Config) error {
	if err := validateR03APaginationV3BackendOnlyConfig(cfg); err != nil {
		return err
	}
	freshnessPath := filepath.Join(cfg.Evidence, "continuation-freshness.json")
	if _, err := os.Stat(freshnessPath); err == nil {
		return errors.New("pagination V3 Backend freshness record already exists; refusing rerun")
	}
	preflightRaw, err := os.ReadFile(filepath.Join(cfg.Evidence, "preflight.json"))
	if err != nil {
		return errors.New("pagination V3 Backend preflight is required before freshness")
	}
	var preflight map[string]any
	if err := json.Unmarshal(preflightRaw, &preflight); err != nil || preflight["passed"] != true || stringValue(preflight, "parent_problem_key") != cfg.ProblemKey {
		return errors.New("pagination V3 Backend preflight is invalid or stale")
	}
	acceptance, backend, blob, err := requireR03APaginationV3BackendOnlyQualification(cfg)
	if err != nil {
		return err
	}
	binding := paginationV3BackendBusinessBinding(backend.ExecutionFingerprint, cfg)
	context := codex.BusinessExecutionContext{ExecutionFingerprint: binding.ExecutionFingerprint, EmployeeID: binding.EmployeeID, ProblemKey: binding.ProblemKey, Purpose: binding.Purpose, Model: binding.Model, Profile: binding.Profile, Effort: binding.Effort, Limits: binding.Limits, TransportPolicyRevision: binding.TransportPolicyRevision, TransportPolicy: binding.TransportPolicy}
	if decision := codex.AuthorizeBusinessExecution(binding, context); !decision.Allowed {
		return fmt.Errorf("pagination V3 Backend authorization binding rejected: %s", decision.ReasonCode)
	}
	if err := writeJSON(cfg.AuthorizationBindingPath, binding); err != nil {
		return err
	}
	return writeJSON(freshnessPath, map[string]any{
		"record_type":                     "r0.3a-pagination-v3-backend-freshness-v1",
		"orchestration_revision":          R03APaginationV3Orchestration,
		"parent_problem_key":              cfg.ProblemKey,
		"subject_revision":                R03APaginationV3SubjectRevision,
		"purpose":                         cfg.RunPurpose,
		"contract_revision":               fixture.PeerPaginationContractRevision,
		"behavior_verifier":               fixture.PeerPaginationBehaviorVerifierRevisionV2,
		"implementation_binding_contract": fixture.BackendBindingContractRevisionV2,
		"backend_binding":                 binding,
		"execution_envelope_fingerprint":  cfg.ExecutionEnvelopeFingerprint,
		"transport_policy":                cfg.TransportPolicy.Snapshot(),
		"acceptance_qualification":        acceptance,
		"backend_l2":                      backend,
		"blob_durability":                 blob,
		"allowance_created":               false,
		"worker_started":                  false,
		"provider_egress":                 0,
		"historical_evidence_modified":    false,
		"recorded_at":                     time.Now().UTC(),
	})
}

func VerifyR03APaginationV3BackendRestore(cfg R03APaginationV3Config) error {
	if err := validateR03APaginationV3BackendOnlyConfig(cfg); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	k, err := kernel.Open(ctx, cfg.DSN, filepath.Join(cfg.Root, "blobs"))
	if err != nil {
		return fmt.Errorf("Backend restored runtime open failed: %w", err)
	}
	defer k.Close()
	anchors, err := k.PeerRecoveryAnchors(ctx, "")
	if err != nil {
		return err
	}
	message, _, obligationState, err := k.PeerMessageAt(ctx, k.LocalScope(anchors.CompanyID), anchors.MessageID)
	if err != nil {
		return err
	}
	contract, err := k.PeerContractAt(ctx, k.LocalScope(anchors.CompanyID), anchors.FinalContractRevisionID)
	if err != nil {
		return err
	}
	workspace, err := k.PeerWorkspaceAt(ctx, k.LocalScope(anchors.CompanyID), anchors.BackendTaskID)
	if err != nil {
		return err
	}
	artifact, err := k.PeerArtifactAt(ctx, k.LocalScope(anchors.CompanyID), anchors.BackendArtifactID)
	if err != nil {
		return err
	}
	qualification, err := k.PeerArtifactQualificationAt(ctx, k.LocalScope(anchors.CompanyID), anchors.BackendArtifactID)
	if err != nil {
		return err
	}
	checkpoint, err := k.PeerCheckpointAt(ctx, k.LocalScope(anchors.CompanyID), anchors.BackendCheckpointID)
	if err != nil {
		return err
	}
	cas, err := k.PeerCASAt(ctx, anchors.CompanyID)
	if err != nil {
		return err
	}
	casArtifactPresent := false
	for _, blob := range cas {
		if blob.ContentSHA256 == artifact.Digest {
			casArtifactPresent = true
			break
		}
	}
	continuityReport, continuityErr := recovery.ValidateArtifactWorkspaceContinuity(recovery.ArtifactWorkspaceContinuityInput{
		ArtifactID:                  artifact.ID,
		ArtifactDigest:              artifact.Digest,
		ArtifactState:               artifact.State,
		ArtifactVerdict:             artifact.Verdict,
		ArtifactWorkspaceTaskID:     anchors.BackendTaskID,
		ArtifactWorkspaceRevision:   qualification.WorkspaceRevision,
		ArtifactWorkspaceDigest:     qualification.WorkspaceDigest,
		CurrentWorkspaceTaskID:      anchors.BackendTaskID,
		CurrentWorkspaceRevision:    workspace.Revision,
		CurrentWorkspaceDigest:      workspace.Digest,
		CheckpointID:                anchors.BackendCheckpointID,
		CheckpointWorkspaceRevision: checkpoint.WorkspaceRevision,
		CheckpointWorkspaceDigest:   checkpoint.WorkspaceDigest,
		CheckpointContractID:        checkpoint.ContractRevisionID,
		ExpectedContractID:          anchors.FinalContractRevisionID,
		CASArtifactDigest:           artifact.Digest,
		CASBlobPresent:              casArtifactPresent,
	})
	if continuityErr != nil {
		return fmt.Errorf("restored Backend continuity check failed: artifact_workspace_continuity: %w", continuityErr)
	}
	plannerRelayCount, err := k.PeerPlannerPathCount(ctx, k.LocalScope(anchors.CompanyID))
	if err != nil {
		return err
	}
	oldWriterErr := k.VerifyHistoricalWorkerFence(ctx, anchors.CompanyID, anchors.BackendSessionID, "emp-backend", anchors.BackendSessionIncarnation, anchors.BackendSessionEpoch)
	oldWriterDenied := oldWriterErr == nil
	if !oldWriterDenied {
		return fmt.Errorf("restored old writer was not denied: %w", oldWriterErr)
	}
	checks := map[string]any{
		"company_mission_task_employee_identities": anchors.CompanyID != "" && anchors.MissionID != "" && anchors.BackendTaskID != "" && anchors.FrontendTaskID != "" && anchors.ObligationOwner == "emp-frontend",
		"contract_revision_chain_anchor":           anchors.FinalContractRevisionID != "" && contract.State == "accepted" && contract.Revision == anchors.FinalContractRevision,
		"message_obligation_continuity":            message.ID == anchors.MessageID && message.ContractRevisionID == anchors.FinalContractRevisionID && anchors.MessageContractRevisionID == anchors.FinalContractRevisionID && obligationState == "pending" && anchors.ObligationState == "pending",
		"checkpoint_anchor":                        anchors.BackendCheckpointID != "",
		"artifact_workspace_continuity":            continuityReport.Passed,
		"event_order_anchor":                       anchors.CompanySequence > 0,
		"cas_digest_anchor":                        anchors.BackendArtifactDigest != "" && anchors.BackendWorkspaceDigest != "",
		"old_writer_denied":                        oldWriterDenied,
		"planner_relay_count_zero":                 plannerRelayCount == 0,
	}
	for name, passed := range checks {
		if ok, _ := passed.(bool); !ok {
			return fmt.Errorf("restored Backend continuity check failed: %s", name)
		}
	}
	return writeJSON(filepath.Join(cfg.Evidence, "backend-restored-runtime-verification.json"), map[string]any{
		"status":                               "PASSED",
		"company_id":                           anchors.CompanyID,
		"mission_id":                           anchors.MissionID,
		"backend_task_id":                      anchors.BackendTaskID,
		"frontend_task_id":                     anchors.FrontendTaskID,
		"final_contract_revision_id":           anchors.FinalContractRevisionID,
		"message_id":                           anchors.MessageID,
		"obligation_id":                        anchors.ObligationID,
		"obligation_state":                     anchors.ObligationState,
		"backend_checkpoint_id":                anchors.BackendCheckpointID,
		"backend_artifact_id":                  anchors.BackendArtifactID,
		"backend_artifact_digest":              anchors.BackendArtifactDigest,
		"backend_workspace_revision":           anchors.BackendWorkspaceRevision,
		"backend_workspace_digest":             anchors.BackendWorkspaceDigest,
		"source_runtime_incarnation":           k.SourceIncarnation(),
		"restored_runtime_incarnation":         k.Incarnation(),
		"checks":                               checks,
		"artifact_workspace_continuity_report": continuityReport,
		"synthesized_rows":                     0,
		"historical_evidence_modified":         false,
	})
}

func InspectR03APaginationV3(cfg R03APaginationV3Config) error {
	if err := validateR03APaginationV3Config(cfg); err != nil {
		return err
	}
	if entries, err := os.ReadDir(cfg.Evidence); err == nil && len(entries) != 0 {
		return errors.New("pagination V3 evidence path is not fresh; refusing retry/reset")
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	if _, err := os.Stat(cfg.AllowancePath); err == nil {
		return errors.New("pagination V3 allowance already exists before preflight")
	}
	qualification, err := RequireR03APaginationV3Qualification(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.Evidence, 0700); err != nil {
		return err
	}
	backendBinding := paginationV3BusinessBinding(cfg.ExecutionEnvelopeFingerprint, "emp-backend", cfg)
	frontendBinding := paginationV3BusinessBinding(cfg.ExecutionEnvelopeFingerprint, "emp-frontend", cfg)
	if err := writeJSON(cfg.AuthorizationBindingPath, backendBinding); err != nil {
		return err
	}
	if err := writeJSON(cfg.FrontendBindingPath, frontendBinding); err != nil {
		return err
	}
	return writeJSON(filepath.Join(cfg.Evidence, "preflight.json"), map[string]any{
		"qualification":                   "R0.3A-PAGINATION-V3-REAL-PEER-COLLABORATION",
		"status":                          "preflight_passed",
		"passed":                          true,
		"parent_problem_key":              cfg.ProblemKey,
		"subject_revision":                R03APaginationV3SubjectRevision,
		"purpose":                         cfg.RunPurpose,
		"contract_revision":               fixture.PeerPaginationContractRevision,
		"behavior_verifier":               fixture.PeerPaginationBehaviorVerifierRevisionV2,
		"implementation_binding_contract": fixture.BackendBindingContractRevisionV2,
		"backend_l2_fingerprint":          qualification.Backend.ExecutionFingerprint,
		"frontend_l2_fingerprint":         qualification.Frontend.ExecutionFingerprint,
		"l1_fingerprint":                  qualification.Backend.ExecutionFingerprint,
		"medium_limit":                    3,
		"high_limit":                      0,
		"tool_call_limit":                 48,
		"allowance_created":               false,
		"worker_started":                  false,
		"provider_egress":                 0,
		"historical_evidence_modified":    false,
	})
}

func RecordR03APaginationV3Freshness(cfg R03APaginationV3Config) error {
	if err := validateR03APaginationV3Config(cfg); err != nil {
		return err
	}
	path := filepath.Join(cfg.Evidence, "continuation-freshness.json")
	if _, err := os.Stat(path); err == nil {
		return errors.New("pagination V3 freshness record already exists; refusing rerun")
	}
	preflightRaw, err := os.ReadFile(filepath.Join(cfg.Evidence, "preflight.json"))
	if err != nil {
		return errors.New("pagination V3 preflight is required before freshness")
	}
	var preflight map[string]any
	if err := json.Unmarshal(preflightRaw, &preflight); err != nil || preflight["passed"] != true || stringValue(preflight, "parent_problem_key") != cfg.ProblemKey {
		return errors.New("pagination V3 preflight is invalid or stale")
	}
	qualification, err := RequireR03APaginationV3Qualification(cfg)
	if err != nil {
		return err
	}
	backendBinding := paginationV3BusinessBinding(qualification.Backend.ExecutionFingerprint, "emp-backend", cfg)
	frontendBinding := paginationV3BusinessBinding(qualification.Frontend.ExecutionFingerprint, "emp-frontend", cfg)
	for _, binding := range []codex.BusinessAuthorizationBinding{backendBinding, frontendBinding} {
		context := codex.BusinessExecutionContext{ExecutionFingerprint: binding.ExecutionFingerprint, EmployeeID: binding.EmployeeID, ProblemKey: binding.ProblemKey, Purpose: binding.Purpose, Model: binding.Model, Profile: binding.Profile, Effort: binding.Effort, Limits: binding.Limits, TransportPolicyRevision: binding.TransportPolicyRevision, TransportPolicy: binding.TransportPolicy}
		if decision := codex.AuthorizeBusinessExecution(binding, context); !decision.Allowed {
			return fmt.Errorf("pagination V3 authorization binding rejected: %s", decision.ReasonCode)
		}
	}
	return writeJSON(path, map[string]any{
		"record_type":                     "r0.3a-pagination-v3-freshness-v1",
		"orchestration_revision":          R03APaginationV3Orchestration,
		"parent_problem_key":              cfg.ProblemKey,
		"subject_revision":                R03APaginationV3SubjectRevision,
		"contract_revision":               fixture.PeerPaginationContractRevision,
		"behavior_verifier":               fixture.PeerPaginationBehaviorVerifierRevisionV2,
		"implementation_binding_contract": fixture.BackendBindingContractRevisionV2,
		"backend_binding":                 backendBinding,
		"frontend_binding":                frontendBinding,
		"execution_envelope_fingerprint":  cfg.ExecutionEnvelopeFingerprint,
		"transport_policy":                cfg.TransportPolicy.Snapshot(),
		"acceptance_qualification":        qualification.Acceptance,
		"backend_l2":                      qualification.Backend,
		"frontend_l2":                     qualification.Frontend,
		"blob_durability":                 qualification.Blob,
		"allowance_created":               false,
		"worker_started":                  false,
		"provider_egress":                 0,
		"historical_evidence_modified":    false,
		"recorded_at":                     time.Now().UTC(),
	})
}

func RunR03APaginationV3Backend(cfg R03APaginationV3Config) (result map[string]any, err error) {
	result = map[string]any{
		"qualification":                   "R0.3A-PAGINATION-V3-REAL-PEER-COLLABORATION",
		"stage":                           "backend",
		"status":                          "NOT_STARTED",
		"parent_problem_key":              cfg.ProblemKey,
		"subject_revision":                R03APaginationV3SubjectRevision,
		"contract_revision":               fixture.PeerPaginationContractRevision,
		"behavior_verifier":               fixture.PeerPaginationBehaviorVerifierRevisionV2,
		"implementation_binding_contract": fixture.BackendBindingContractRevisionV2,
		"medium_started":                  0, "high_started": 0, "provider_egress": 0,
		"allowance_created": false, "worker_started": false,
		"backend_real_execution": "NOT_STARTED", "transport_state": "NOT_STARTED",
		"frontend_started": false, "high_eligible": false,
		"historical_evidence_modified": false,
	}
	writeResult := func() { _ = writeJSON(cfg.BackendResultPath, result) }
	if err = validateR03APaginationV3BackendOnlyConfig(cfg); err != nil {
		writeResult()
		return result, err
	}
	if err = requireV3Records(cfg); err != nil {
		writeResult()
		return result, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err = recordR03ARecoveryReadiness(ctx, cfg); err != nil {
		writeResult()
		return result, err
	}
	_, backendQualification, _, err := requireR03APaginationV3BackendOnlyQualification(cfg)
	if err != nil {
		writeResult()
		return result, err
	}
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if err = os.MkdirAll(cfg.Root, 0700); err != nil {
		writeResult()
		return result, err
	}
	k, err := kernel.Open(ctx, cfg.DSN, filepath.Join(cfg.Root, "blobs"))
	if err != nil {
		writeResult()
		return result, err
	}
	defer k.Close()
	if err = writeJSON(filepath.Join(cfg.Evidence, "backend-runtime-connection.json"), map[string]any{"status": "passed", "allowance_created": false, "worker_started": false, "provider_egress": 0, "verified_at": time.Now().UTC()}); err != nil {
		writeResult()
		return result, err
	}
	binding, err := LoadBusinessAuthorization(cfg.AuthorizationBindingPath, codex.BusinessExecutionContext{ExecutionFingerprint: cfg.ExecutionEnvelopeFingerprint, EmployeeID: "emp-backend", ProblemKey: cfg.ProblemKey, Purpose: cfg.RunPurpose, Model: cfg.Model, Profile: cfg.Model + "/medium", Effort: "medium", Limits: codex.AllowanceLimits{Medium: 1, High: 0, Concurrency: 1, ToolCallLimit: 48}, TransportPolicyRevision: cfg.TransportPolicy.Revision, TransportPolicy: cfg.TransportPolicy.Snapshot()})
	if err != nil {
		writeResult()
		return result, err
	}
	if _, err = os.Stat(cfg.AllowancePath); err == nil {
		writeResult()
		return result, errors.New("pagination V3 allowance already exists; refusing retry")
	}
	budget, err := codex.NewBusinessBudget(cfg.AllowancePath, 1, 0, 48)
	if err != nil {
		writeResult()
		return result, err
	}
	result["allowance_created"], result["allowance_binding"], result["medium_limit"], result["high_limit"], result["tool_call_limit"] = true, binding, 1, 0, 48
	if err = writeJSON(filepath.Join(cfg.Evidence, "allowance-binding.json"), map[string]any{"contract_revision": fixture.PeerPaginationContractRevision, "implementation_binding_contract": fixture.BackendBindingContractRevisionV2, "behavior_verifier": fixture.PeerPaginationBehaviorVerifierRevisionV2, "binding": binding, "execution_envelope_fingerprint": cfg.ExecutionEnvelopeFingerprint, "transport_policy": cfg.TransportPolicy.Snapshot(), "allowance_path": cfg.AllowancePath}); err != nil {
		writeResult()
		return result, err
	}
	scope, err := k.TXCreateCompany(ctx, "r03a-pagination-v3-company-"+fmt.Sprint(budget.Started.UnixNano()))
	if err != nil {
		writeResult()
		return result, err
	}
	mission := "r03a-pagination-v3-mission-" + fmt.Sprint(budget.Started.UnixNano())
	fx, err := k.TXCreatePeerFixture(ctx, scope, mission)
	if err != nil {
		writeResult()
		return result, err
	}
	worker, err := k.TXNewWorkerWithToolBudget(ctx, scope, fx.Backend.ID, cfg.Model+"/medium", 48)
	if err != nil {
		writeResult()
		return result, err
	}
	if err = budget.Reserve("medium"); err != nil {
		writeResult()
		return result, err
	}
	result["medium_started"] = budget.Medium
	stageCfg := v3StageConfig(cfg, backendQualification, cfg.AuthorizationBindingPath)
	session, sessionErr := runPeerSession(ctx, R03AConfig{Config: stageCfg.Config, ProblemKey: cfg.ProblemKey}, k, worker, "peer_backend", r03aPaginationV3BackendPrompt(), false, false, 1, "pagination V3 Backend employee")
	result["backend_session"] = session.turn
	result["worker_started"] = session.turn.ProcessStarted
	result["provider_egress"] = boolInt(session.turn.ProviderEgress)
	result["tool_calls_used"], result["tool_calls_remaining"] = session.turn.ToolCallsUsed, session.turn.ToolCallsRemaining
	if sessionErr != nil {
		result["status"], result["backend_real_execution"], result["transport_state"], err = "FAILED", "FAILED", "terminally_reconciled", sessionErr
		writeResult()
		return result, err
	}
	result["transport_state"] = "terminally_reconciled"
	contract, message, obligation, artifact, workspace, checkErr := backendV3Outcome(ctx, k, scope, fx, session)
	if checkErr != nil {
		result["status"], result["backend_real_execution"], result["transport_state"] = "FAILED", "FAILED", "terminally_reconciled"
		err = checkErr
		writeResult()
		return result, err
	}
	result["backend_real_execution"] = "PASSED"
	result["final_contract_revision_id"], result["final_contract_revision"] = contract.ID, contract.Revision
	result["message_id"], result["obligation_id"] = message.ID, obligation
	result["backend_artifact_id"], result["backend_artifact_digest"] = artifact.ID, artifact.Digest
	result["backend_workspace_revision"], result["backend_workspace_digest"] = workspace.Revision, workspace.Digest
	result["planner_relay"] = "absent"
	if err = writeJSON(filepath.Join(cfg.Evidence, "backend-state.json"), map[string]any{"contract": contract, "message": message, "obligation_state": "pending", "artifact": artifact, "workspace": workspace, "backend_session": session.turn}); err != nil {
		writeResult()
		return result, err
	}
	snapshot, err := k.PeerHandoverBoundarySnapshot(ctx, session.binding, session.handover)
	if err != nil {
		writeResult()
		return result, fmt.Errorf("Backend authoritative snapshot failed: %w", err)
	}
	if err = writeJSON(filepath.Join(filepath.Dir(cfg.PostgresSnapshotPath), "authoritative-snapshot.json"), snapshot); err != nil {
		writeResult()
		return result, err
	}
	if err = writeJSON(filepath.Join(filepath.Dir(cfg.PostgresSnapshotPath), "cas-manifest.json"), map[string]any{"schema_version": "r03a-pagination-v3-cas-manifest@1", "runtime_incarnation": snapshot.RuntimeIncarnation, "entries": snapshot.CAS, "manifest_digest": snapshot.CASManifestDigest, "historical_evidence_modified": false}); err != nil {
		writeResult()
		return result, err
	}
	snapshotDigest, snapshotSize, err := dumpHandoverBoundaryDatabase(ctx, cfg.PostgresDumpPath, cfg.PostgresSnapshotDSN, cfg.PostgresSnapshotPath)
	if err != nil {
		writeResult()
		return result, err
	}
	result["authoritative_snapshot"] = "PASSED"
	result["snapshot_path"], result["snapshot_sha256"], result["snapshot_size"] = cfg.PostgresSnapshotPath, snapshotDigest, snapshotSize
	result["cas_manifest_digest"] = snapshot.CASManifestDigest
	result["frontend_started"] = false
	result["status"] = "BACKEND_PASSED_FRONTEND_PENDING"
	writeResult()
	return result, nil
}

func RunR03APaginationV3Frontend(cfg R03APaginationV3Config) (result map[string]any, err error) {
	result = map[string]any{"qualification": "R0.3A-PAGINATION-V3-REAL-PEER-COLLABORATION", "stage": "frontend", "status": "NOT_STARTED", "parent_problem_key": cfg.ProblemKey, "subject_revision": R03APaginationV3SubjectRevision, "contract_revision": fixture.PeerPaginationContractRevision, "behavior_verifier": fixture.PeerPaginationBehaviorVerifierRevisionV2, "high_started": 0, "provider_egress": 0, "high_eligible": false, "historical_evidence_modified": false}
	writeResult := func() { _ = writeJSON(cfg.FrontendResultPath, result) }
	if err = validateR03APaginationV3Config(cfg); err != nil {
		writeResult()
		return result, err
	}
	if err = requireV3Records(cfg); err != nil {
		writeResult()
		return result, err
	}
	backendRaw, err := os.ReadFile(cfg.BackendResultPath)
	if err != nil {
		writeResult()
		return result, err
	}
	var backendResult map[string]any
	if err = json.Unmarshal(backendRaw, &backendResult); err != nil || stringValue(backendResult, "backend_real_execution") != "PASSED" || stringValue(backendResult, "authoritative_snapshot") != "PASSED" {
		writeResult()
		return result, errors.New("Backend V3 boundary is not passed")
	}
	if cfg.RestoreProofPath == "" {
		writeResult()
		return result, errors.New("fresh restore proof is required")
	}
	proofRaw, err := os.ReadFile(cfg.RestoreProofPath)
	if err != nil {
		writeResult()
		return result, err
	}
	var proof map[string]any
	if err = json.Unmarshal(proofRaw, &proof); err != nil || stringValue(proof, "status") != "PASSED" {
		writeResult()
		return result, errors.New("fresh restored PostgreSQL boundary is not passed")
	}
	qualification, err := RequireR03APaginationV3Qualification(cfg)
	if err != nil {
		writeResult()
		return result, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	k, err := kernel.Open(ctx, cfg.DSN, filepath.Join(cfg.Root, "blobs"))
	if err != nil {
		writeResult()
		return result, err
	}
	defer k.Close()
	if err = writeJSON(filepath.Join(cfg.Evidence, "frontend-runtime-connection.json"), map[string]any{"status": "passed", "restore_proof": proof, "verified_at": time.Now().UTC()}); err != nil {
		writeResult()
		return result, err
	}
	anchors, err := k.PeerRecoveryAnchors(ctx, "")
	if err != nil {
		writeResult()
		return result, err
	}
	if anchors.MessageState != "persisted" || anchors.ObligationState != "pending" || anchors.ObligationOwner != "emp-frontend" || anchors.MessageContractRevisionID != anchors.FinalContractRevisionID || anchors.BackendSessionState != "stopped" {
		writeResult()
		return result, errors.New("restored V3 anchors are not the expected pending frontend responsibility")
	}
	if err = k.VerifyHistoricalWorkerFence(ctx, anchors.CompanyID, anchors.BackendSessionID, "emp-backend", anchors.BackendSessionIncarnation, anchors.BackendSessionEpoch); err != nil {
		writeResult()
		return result, err
	}
	binding, err := LoadBusinessAuthorization(cfg.FrontendBindingPath, codex.BusinessExecutionContext{ExecutionFingerprint: qualification.Frontend.ExecutionFingerprint, EmployeeID: "emp-frontend", ProblemKey: cfg.ProblemKey, Purpose: cfg.RunPurpose, Model: cfg.Model, Profile: cfg.Model + "/medium", Effort: "medium", Limits: codex.AllowanceLimits{Medium: 3, High: 0, Concurrency: 1, ToolCallLimit: 48}, TransportPolicyRevision: cfg.TransportPolicy.Revision, TransportPolicy: cfg.TransportPolicy.Snapshot()})
	if err != nil {
		writeResult()
		return result, err
	}
	result["authorization_binding"] = binding
	budget, err := codex.OpenBudget(cfg.AllowancePath)
	if err != nil {
		writeResult()
		return result, err
	}
	if budget.Medium != 1 || budget.High != 0 || budget.ToolCallLimit != 48 {
		writeResult()
		return result, errors.New("pagination V3 allowance state is stale before Frontend")
	}
	initialWorkspace, err := k.PeerWorkspaceAt(ctx, k.LocalScope(anchors.CompanyID), anchors.FrontendTaskID)
	if err != nil {
		writeResult()
		return result, err
	}
	worker, err := k.TXNewWorkerWithToolBudget(ctx, k.LocalScope(anchors.CompanyID), anchors.FrontendTaskID, cfg.Model+"/medium", 48)
	if err != nil {
		writeResult()
		return result, err
	}
	if err = budget.Reserve("medium"); err != nil {
		writeResult()
		return result, err
	}
	result["medium_started"] = budget.Medium
	stageCfg := v3StageConfig(cfg, qualification.Frontend, cfg.FrontendBindingPath)
	boundaryPredicate := func(b kernel.Binding, turn R03ATurn) bool {
		return v3FrontendBoundarySatisfied(context.Background(), k, b, initialWorkspace.Revision, turn)
	}
	initial, initialErr := runPeerSession(ctx, R03AConfig{Config: stageCfg.Config, ProblemKey: cfg.ProblemKey, BoundaryPredicate: boundaryPredicate}, k, worker, "peer_frontend", r03aPaginationV3FrontendInitialPrompt(), true, true, 2, "pagination V3 Frontend initial employee")
	result["initial_session"] = initial.turn
	result["provider_egress"] = boolInt(initial.turn.ProviderEgress)
	if initialErr != nil {
		result["frontend_initial"], err = "FAILED", initialErr
		writeResult()
		return result, err
	}
	message, _, obligationState, err := k.PeerMessageAt(ctx, k.LocalScope(anchors.CompanyID), anchors.MessageID)
	if err != nil {
		writeResult()
		return result, err
	}
	initialHandover := initial.handover
	initialPass := initial.turn.StopConfirmed && !turnHas(initial.turn, "artifact_submit") && !turnHas(initial.turn, "obligation_resolve") && turnHas(initial.turn, "collab_apply") && turnHas(initial.turn, "workspace_check") && initialHandover.WorkspaceRevision > initialWorkspace.Revision && message.ID == anchors.MessageID && message.ContractRevisionID == anchors.FinalContractRevisionID && obligationState == "applied" && hasCheckpointKind(initialHandover.Checkpoints, kernel.CheckpointProgress)
	if !initialPass {
		result["frontend_initial"], err = "FAILED", errors.New("Frontend initial did not reach deterministic V3 handover boundary")
		writeResult()
		return result, err
	}
	result["frontend_initial"] = "PASSED_TO_HANDOVER_BOUNDARY"
	if err = writeJSON(filepath.Join(cfg.Evidence, "frontend-initial-boundary.json"), map[string]any{"status": "PASSED", "message": message, "obligation_state": obligationState, "handover": initialHandover, "session": initial.turn, "initial_workspace_revision": initialWorkspace.Revision}); err != nil {
		writeResult()
		return result, err
	}
	successorWorker, err := k.TXNewWorkerWithToolBudget(ctx, k.LocalScope(anchors.CompanyID), anchors.FrontendTaskID, cfg.Model+"/medium", 48)
	if err != nil {
		writeResult()
		return result, err
	}
	if err = budget.Reserve("medium"); err != nil {
		writeResult()
		return result, err
	}
	result["medium_started"] = budget.Medium
	successor, successorErr := runPeerSession(ctx, R03AConfig{Config: stageCfg.Config, ProblemKey: cfg.ProblemKey}, k, successorWorker, "peer_frontend", r03aPaginationV3FrontendSuccessorPrompt(), false, false, 3, "pagination V3 Frontend successor employee")
	result["successor_session"] = successor.turn
	result["provider_egress"] = boolInt(initial.turn.ProviderEgress) + boolInt(successor.turn.ProviderEgress)
	if successorErr != nil {
		result["successor_business_completion"], err = "FAILED", successorErr
		writeResult()
		return result, err
	}
	oldApply, oldApplyErr := k.TXPeerApply(ctx, initial.binding, kernel.PeerApplyRequest{ObligationID: anchors.ObligationID, ContractRevisionID: anchors.FinalContractRevisionID, WorkspaceRevision: initialHandover.WorkspaceRevision + 1, EvidenceRefs: []string{"v3-old-writer-apply"}}, "v3-old-writer-apply")
	_ = oldApply
	oldResolveErr := k.TXPeerResolve(ctx, initial.binding, anchors.ObligationID, successor.turn.ArtifactID, "v3-old-writer-resolve")
	oldWriterRejected := errors.Is(oldApplyErr, core.StaleEpoch) && errors.Is(oldResolveErr, core.StaleEpoch)
	if err = writeJSON(filepath.Join(cfg.Evidence, "old-writer-rejection.json"), map[string]any{"workspace_apply": fmt.Sprint(oldApplyErr), "obligation_resolve": fmt.Sprint(oldResolveErr), "all_stale_epoch": oldWriterRejected}); err != nil {
		writeResult()
		return result, err
	}
	if !oldWriterRejected || successor.turn.ArtifactID == "" || successor.turn.CheckpointID == "" || !turnHas(successor.turn, "obligation_resolve") {
		result["successor_business_completion"], err = "FAILED", errors.New("successor did not complete V3 artifact/resolve or old writer fence")
		writeResult()
		return result, err
	}
	result["successor_business_completion"] = "PASSED"
	finalAnchors, err := k.PeerRecoveryAnchors(ctx, anchors.CompanyID)
	if err != nil {
		writeResult()
		return result, err
	}
	artifact, err := k.PeerArtifactAt(ctx, k.LocalScope(anchors.CompanyID), successor.turn.ArtifactID)
	if err != nil {
		writeResult()
		return result, err
	}
	workspace, err := k.PeerWorkspaceAt(ctx, k.LocalScope(anchors.CompanyID), anchors.FrontendTaskID)
	if err != nil {
		writeResult()
		return result, err
	}
	contract, err := k.PeerContractAt(ctx, k.LocalScope(anchors.CompanyID), anchors.FinalContractRevisionID)
	if err != nil {
		writeResult()
		return result, err
	}
	if finalAnchors.ObligationState != "fulfilled" || finalAnchors.MessageState != "resolved" || contract.State != "accepted" || workspace.Revision <= initialWorkspace.Revision {
		result["status"], err = "FAILED", errors.New("V3 final authoritative state is incomplete")
		writeResult()
		return result, err
	}
	integration, err := k.TXFreezePeerIntegration(ctx, k.LocalScope(anchors.CompanyID), kernel.PeerIntegrationInput{Mission: anchors.MissionID, BackendArtifactID: anchors.BackendArtifactID, FrontendArtifactID: artifact.ID, ContractRevisionID: anchors.FinalContractRevisionID, BaseRevision: "api-v1", VerifierRevision: fixture.PeerPaginationBehaviorVerifierRevisionV2}, "v3-positive-integration")
	if err != nil {
		writeResult()
		return result, err
	}
	positive, err := k.VerifyPeerIntegration(ctx, k.LocalScope(anchors.CompanyID), integration.ID)
	if err != nil {
		writeResult()
		return result, err
	}
	if !positive.Passed || positive.PaginationRuntimeBehavior != "passed" || positive.ResponseShapeCompatibility != "passed" || positive.LifecycleBinding != "passed" || positive.CollaborationCausality != "passed" {
		result["status"], err = "FAILED", errors.New("V3 positive integration dimensions did not all pass")
		writeResult()
		return result, err
	}
	if err = writeJSON(filepath.Join(cfg.Evidence, "integration-verifier.json"), positive); err != nil {
		writeResult()
		return result, err
	}
	negative, err := runPeerNegativeControl(ctx, k)
	if err != nil {
		writeResult()
		return result, err
	}
	if negative.Passed {
		result["status"], err = "FAILED", errors.New("collaboration-suppressed negative control unexpectedly passed")
		writeResult()
		return result, err
	}
	if err = writeJSON(filepath.Join(cfg.Evidence, "negative-control.json"), negative); err != nil {
		writeResult()
		return result, err
	}
	budgetState := budget.ToolBudget()
	result["status"], result["new_subject_real_peer_collaboration"] = "PASSED", "PASSED"
	result["eligible_for_new_independent_high_review"], result["high_started"] = true, 0
	result["current_contract_revision_id"], result["current_contract_revision"] = contract.ID, contract.Revision
	result["message_id"], result["obligation_id"] = finalAnchors.MessageID, finalAnchors.ObligationID
	result["frontend_artifact_id"], result["frontend_artifact_digest"] = artifact.ID, artifact.Digest
	result["frontend_workspace_revision"], result["frontend_workspace_digest"] = workspace.Revision, workspace.Digest
	result["tool_calls_used"], result["tool_calls_remaining"] = budgetState.Used, budgetState.Remaining
	result["pagination_runtime_behavior"], result["response_shape_compatibility"] = positive.PaginationRuntimeBehavior, positive.ResponseShapeCompatibility
	result["collaboration_lifecycle"], result["causal_positive"], result["negative_control"] = positive.LifecycleBinding, positive.CollaborationCausality, "FAIL_AS_EXPECTED"
	writeResult()
	return result, nil
}

func requireV3Records(cfg R03APaginationV3Config) error {
	if _, err := os.Stat(filepath.Join(cfg.Evidence, "preflight.json")); err != nil {
		return errors.New("pagination V3 preflight missing")
	}
	if _, err := os.Stat(filepath.Join(cfg.Evidence, "continuation-freshness.json")); err != nil {
		return errors.New("pagination V3 freshness missing")
	}
	return nil
}

func paginationV3BusinessBinding(fingerprint, employee string, cfg R03APaginationV3Config) codex.BusinessAuthorizationBinding {
	return codex.BusinessAuthorizationBinding{ExecutionFingerprint: fingerprint, EmployeeID: employee, ProblemKey: cfg.ProblemKey, Purpose: cfg.RunPurpose, Model: cfg.Model, Profile: cfg.Model + "/medium", Effort: "medium", Limits: codex.AllowanceLimits{Medium: 3, High: 0, Concurrency: 1, ToolCallLimit: 48}, TransportPolicyRevision: cfg.TransportPolicy.Revision, TransportPolicy: cfg.TransportPolicy.Snapshot()}
}

func paginationV3BackendBusinessBinding(fingerprint string, cfg R03APaginationV3Config) codex.BusinessAuthorizationBinding {
	return codex.BusinessAuthorizationBinding{ExecutionFingerprint: fingerprint, EmployeeID: "emp-backend", ProblemKey: cfg.ProblemKey, Purpose: cfg.RunPurpose, Model: cfg.Model, Profile: cfg.Model + "/medium", Effort: "medium", Limits: codex.AllowanceLimits{Medium: 1, High: 0, Concurrency: 1, ToolCallLimit: 48}, TransportPolicyRevision: cfg.TransportPolicy.Revision, TransportPolicy: cfg.TransportPolicy.Snapshot()}
}

func v3StageConfig(cfg R03APaginationV3Config, qualification R03AT2QualificationReport, bindingPath string) R03AT2Config {
	stage := cfg.Config
	stage.CapabilityDigest = qualification.CapabilityDigest
	stage.ExpectedNativeVersion = qualification.CodexVersion
	return R03AT2Config{Config: stage, ProblemKey: cfg.ProblemKey, RunPurpose: cfg.RunPurpose, AuthorizationBindingPath: bindingPath, ExecutionFingerprint: qualification.ExecutionFingerprint}
}

func executionFingerprintFromManifest(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var record struct {
		ExecutionFingerprint string `json:"execution_fingerprint"`
	}
	if err := json.Unmarshal(raw, &record); err != nil || record.ExecutionFingerprint == "" {
		return "", errors.New("execution manifest fingerprint missing")
	}
	return record.ExecutionFingerprint, nil
}

func requireV3BackendL2Qualification(cfg R03AT2Config) (R03AT2QualificationReport, error) {
	var report R03AT2QualificationReport
	var manifest currentBinaryL2ExecutionManifest
	if err := readT11JSON(cfg.ExecutionManifestPath, &manifest); err != nil {
		return report, fmt.Errorf("V3 Backend gate blocked: read current Backend L2 manifest: %w", err)
	}
	if manifest.SchemaVersion != r03aCurrentBinaryL2ManifestSchema || manifest.Qualification != r03aCurrentBinaryL2Qualification || manifest.ExecutionFingerprint != cfg.ExecutionFingerprint || !validCurrentBinaryDigest(manifest.ExecutionFingerprint) || manifest.L1Fingerprint == "" || manifest.ControlledRuntimeManifestDigest == "" || manifest.ToolManifestDigest == "" {
		return report, errors.New("V3 Backend gate blocked: current Backend L2 manifest is stale or mismatched")
	}
	if err := manifest.CanonicalManifest.Validate(); err != nil {
		return report, fmt.Errorf("V3 Backend gate blocked: invalid current Backend L2 canonical manifest: %w", err)
	}
	expectedFingerprint := manifest.CanonicalManifest.Base.Combination.CurrentFingerprintV7(manifest.CanonicalManifest)
	if manifest.CanonicalManifestDigest != expectedFingerprint.CanonicalManifestDigest {
		return report, errors.New("V3 Backend gate blocked: current Backend L2 canonical manifest digest mismatch")
	}
	bindingFingerprint, err := manifest.Binding.Fingerprint()
	if err != nil || bindingFingerprint != manifest.ExecutionFingerprint {
		return report, errors.New("V3 Backend gate blocked: current Backend L2 authorization binding mismatch")
	}
	var l1 currentBinaryL1Evidence
	if cfg.CurrentL1EvidencePath == "" || readT11JSON(filepath.Join(cfg.CurrentL1EvidencePath, "result.json"), &l1) != nil || l1.Status != "passed" || l1.CurrentBinaryWindowsL1 != "QUALIFIED" || l1.ExecutionFingerprint != manifest.L1Fingerprint || l1.ControlledRuntimeManifestDigest != manifest.ControlledRuntimeManifestDigest || l1.CodexBinarySHA256 != manifest.CanonicalManifest.Base.Combination.BinarySHA256 || l1.CodeModeHostSHA256 != manifest.CanonicalManifest.Base.Combination.CodeModeHostSHA256 || l1.Model != "gpt-5.6-luna" || l1.Effort != "medium" || l1.RuntimeProfile != "windows-native" || l1.ProviderTransportPolicy != "native_default" || l1.DynamicToolCount != 0 || l1.MediumStarted != 1 || l1.ProviderEgress != 1 || l1.UnresolvedTransportState {
		return report, errors.New("V3 Backend gate blocked: current Windows L1 evidence is stale or mismatched")
	}
	currentSurface, tools, _, err := buildT21ToolSurface()
	if err != nil {
		return report, fmt.Errorf("V3 Backend gate blocked: build formal PeerBackendTools surface: %w", err)
	}
	payload, err := json.Marshal(t21NormalizedThreadPayload(tools, r03aT21DeveloperInstruction))
	if err != nil {
		return report, err
	}
	currentSurface.ThreadStartPayloadDigest = digest(payload)
	currentSurface.ThreadStartPayloadBytes = len(payload)
	if !sameV3BackendProviderSurface(currentSurface, manifest.ToolSurface) || manifest.ToolSurface.AggregateManifestDigest != manifest.ToolManifestDigest || !reflect.DeepEqual(manifest.CanonicalManifest.ToolSurface, manifest.ToolSurface) {
		return report, fmt.Errorf("V3 Backend gate blocked: provider-visible Backend tool surface drifted: %s", strings.Join(v3BackendProviderSurfaceDiff(currentSurface, manifest.ToolSurface), ","))
	}
	var live currentBinaryL2LiveQualification
	if err := readT11JSON(cfg.QualificationPath, &live); err != nil {
		return report, fmt.Errorf("V3 Backend gate blocked: read Backend L2 live qualification: %w", err)
	}
	if live.Qualification != r03aCurrentBinaryL2Qualification || live.Status != "passed" || live.ExecutionFingerprint != manifest.ExecutionFingerprint || live.CurrentBinaryRevised11ToolL2 != "QUALIFIED" || !live.EligibleForRevisedBackendRun {
		return report, errors.New("V3 Backend gate blocked: Backend L2 live qualification is not eligible")
	}
	binaryDigest, err := sha256File(cfg.Binary)
	if err != nil || binaryDigest != manifest.CanonicalManifest.Base.Combination.BinarySHA256 {
		return report, errors.New("V3 Backend gate blocked: controlled Codex binary drifted")
	}
	helperDigest, err := sha256File(cfg.CodeModeHost)
	if err != nil || helperDigest != manifest.CanonicalManifest.Base.Combination.CodeModeHostSHA256 {
		return report, errors.New("V3 Backend gate blocked: controlled code-mode host drifted")
	}
	selectedConfigDigest, err := sha256File(cfg.SelectedConfigPath)
	if err != nil || selectedConfigDigest != l1.SelectedConfigRawSHA256 {
		return report, errors.New("V3 Backend gate blocked: selected config drifted from current L1")
	}
	authRaw, err := os.ReadFile(cfg.AuthFile)
	if err != nil {
		return report, fmt.Errorf("V3 Backend gate blocked: read current auth: %w", err)
	}
	authMaterial, err := codex.ParseAuthMaterial(authRaw, manifest.CanonicalManifest.Base.Auth.AuthSourceClass)
	if err != nil || !reflect.DeepEqual(authMaterial.Manifest(), manifest.CanonicalManifest.Base.Auth) {
		return report, errors.New("V3 Backend gate blocked: current auth identity or credential revision drifted")
	}
	if cfg.ExpectedNativeVersion != "" && cfg.ExpectedNativeVersion != manifest.CanonicalManifest.Base.Combination.CodexVersion {
		return report, errors.New("V3 Backend gate blocked: expected Codex version drifted")
	}
	return R03AT2QualificationReport{Qualification: manifest.Qualification, ExecutionFingerprint: manifest.ExecutionFingerprint, CodexVersion: manifest.CanonicalManifest.Base.Combination.CodexVersion, Model: manifest.CanonicalManifest.Base.Combination.Model, Effort: manifest.CanonicalManifest.Base.Combination.Effort, CapabilityDigest: manifest.CanonicalManifest.Base.Combination.CapabilityDigest, ToolManifestDigest: manifest.ToolSurface.AggregateManifestDigest, ToolCount: manifest.ToolSurface.ToolCount, AuthSourceClass: authMaterial.SourceClass, AuthIdentityFingerprint: authMaterial.IdentityFingerprint, AuthCredentialRevision: authMaterial.CredentialRevisionFingerprint, BehavioralContractQualification: "passed", RevalidatedAt: time.Now().UTC()}, nil
}

func sameV3BackendProviderSurface(current, expected codex.ToolSurfaceManifest) bool {
	current.CheckpointPolicyRevision = expected.CheckpointPolicyRevision
	current.ArtifactEligibilityPolicyRevision = expected.ArtifactEligibilityPolicyRevision
	current.ContractSupersessionPolicyRevision = expected.ContractSupersessionPolicyRevision
	current.AcceptanceCheckerRevision = expected.AcceptanceCheckerRevision
	return reflect.DeepEqual(current, expected)
}

func v3BackendProviderSurfaceDiff(current, expected codex.ToolSurfaceManifest) []string {
	diff := []string{}
	if current.SurfaceID != expected.SurfaceID {
		diff = append(diff, "surface_id")
	}
	if current.ToolCount != expected.ToolCount {
		diff = append(diff, "tool_count")
	}
	if current.AggregateManifestDigest != expected.AggregateManifestDigest {
		diff = append(diff, "aggregate_manifest_digest")
	}
	if current.AggregateSchemaDigest != expected.AggregateSchemaDigest {
		diff = append(diff, "aggregate_schema_digest")
	}
	if current.AggregateSchemaBytes != expected.AggregateSchemaBytes {
		diff = append(diff, "aggregate_schema_bytes")
	}
	if current.ThreadStartPayloadDigest != expected.ThreadStartPayloadDigest {
		diff = append(diff, "thread_start_payload_digest")
	}
	if current.ThreadStartPayloadBytes != expected.ThreadStartPayloadBytes {
		diff = append(diff, "thread_start_payload_bytes")
	}
	if current.BusinessWritePolicy != expected.BusinessWritePolicy {
		diff = append(diff, "business_write_policy")
	}
	if len(current.Tools) != len(expected.Tools) {
		diff = append(diff, "tools.length")
	} else {
		for i := range current.Tools {
			if current.Tools[i].Name != expected.Tools[i].Name {
				diff = append(diff, fmt.Sprintf("tools[%d].name", i+1))
			}
			if current.Tools[i].SchemaDigest != expected.Tools[i].SchemaDigest {
				diff = append(diff, fmt.Sprintf("tools[%d].schema_digest", i+1))
			}
			if current.Tools[i].SchemaBytes != expected.Tools[i].SchemaBytes {
				diff = append(diff, fmt.Sprintf("tools[%d].schema_bytes", i+1))
			}
			if current.Tools[i].DescriptionDigest != expected.Tools[i].DescriptionDigest {
				diff = append(diff, fmt.Sprintf("tools[%d].description_digest", i+1))
			}
			if current.Tools[i].BindingIdentity != expected.Tools[i].BindingIdentity {
				diff = append(diff, fmt.Sprintf("tools[%d].binding_identity", i+1))
			}
			if current.Tools[i].AuthorizationClass != expected.Tools[i].AuthorizationClass {
				diff = append(diff, fmt.Sprintf("tools[%d].authorization_class", i+1))
			}
			if current.Tools[i].RegistrationOrdinal != expected.Tools[i].RegistrationOrdinal {
				diff = append(diff, fmt.Sprintf("tools[%d].registration_ordinal", i+1))
			}
		}
	}
	if len(diff) == 0 && !reflect.DeepEqual(current, expected) {
		diff = append(diff, "unclassified")
	}
	return diff
}

func backendV3Outcome(ctx context.Context, k *kernel.Kernel, scope kernel.Scope, fx kernel.PeerFixture, session r03aSession) (kernel.PeerContractRevision, kernel.PeerMessage, string, kernel.Artifact, kernel.Workspace, error) {
	var emptyContract kernel.PeerContractRevision
	var emptyMessage kernel.PeerMessage
	var emptyArtifact kernel.Artifact
	var emptyWorkspace kernel.Workspace
	if session.turn.ContractRevisionID == "" || session.turn.MessageID == "" || session.turn.ObligationID == "" || session.turn.ArtifactID == "" || session.turn.CheckpointID == "" || !session.turn.StopConfirmed {
		return emptyContract, emptyMessage, "", emptyArtifact, emptyWorkspace, errors.New("Backend V3 session omitted required receipts")
	}
	contract, err := k.PeerContractAt(ctx, scope, session.turn.ContractRevisionID)
	if err != nil {
		return emptyContract, emptyMessage, "", emptyArtifact, emptyWorkspace, err
	}
	parsed, err := fixture.ParsePaginationBehaviorContract(contract.Schema)
	if err != nil || !reflect.DeepEqual(parsed, fixture.MustPeerPaginationContract()) {
		return emptyContract, emptyMessage, "", emptyArtifact, emptyWorkspace, errors.New("Backend did not accept the exact public pagination contract@3")
	}
	message, _, obligationState, err := k.PeerMessageAt(ctx, scope, session.turn.MessageID)
	if err != nil {
		return emptyContract, emptyMessage, "", emptyArtifact, emptyWorkspace, err
	}
	if message.DeliveryState != "persisted" || obligationState != "pending" || message.ContractRevisionID != contract.ID {
		return emptyContract, emptyMessage, "", emptyArtifact, emptyWorkspace, errors.New("Backend Message or pending Obligation is not current")
	}
	artifact, err := k.PeerArtifactAt(ctx, scope, session.turn.ArtifactID)
	if err != nil {
		return emptyContract, emptyMessage, "", emptyArtifact, emptyWorkspace, err
	}
	workspace, err := k.PeerWorkspaceAt(ctx, scope, fx.Backend.ID)
	if err != nil {
		return emptyContract, emptyMessage, "", emptyArtifact, emptyWorkspace, err
	}
	planner, err := k.PeerPlannerPathCount(ctx, scope)
	if err != nil || planner != 0 || artifact.State != "ready" || artifact.Verdict != "candidate" {
		return emptyContract, emptyMessage, "", emptyArtifact, emptyWorkspace, errors.New("Backend V3 artifact or Planner path failed")
	}
	if !turnHas(session.turn, "collab_send") || !turnHas(session.turn, "workspace_check") || !turnHas(session.turn, "work_checkpoint") || !turnHas(session.turn, "artifact_submit") || firstToolOrdinal(session.turn, "collab_send") > firstToolOrdinal(session.turn, "artifact_submit") {
		return emptyContract, emptyMessage, "", emptyArtifact, emptyWorkspace, errors.New("Backend V3 tool order did not preserve direct send before artifact")
	}
	return contract, message, session.turn.ObligationID, artifact, workspace, nil
}

func v3FrontendBoundarySatisfied(ctx context.Context, k *kernel.Kernel, b kernel.Binding, initialRevision int64, turn R03ATurn) bool {
	if !turnHas(turn, "collab_apply") || !turnHas(turn, "workspace_check") || !turnHas(turn, "work_checkpoint") || turnHas(turn, "artifact_submit") || turnHas(turn, "obligation_resolve") {
		return false
	}
	h, err := k.PeerHandover(ctx, b)
	if err != nil {
		return false
	}
	return h.WorkspaceRevision > initialRevision && h.MessageState == "applied" && h.ObligationState == "applied" && hasCheckpointKind(h.Checkpoints, kernel.CheckpointProgress)
}

func hasCheckpointKind(checkpoints []kernel.Checkpoint, wanted string) bool {
	for _, checkpoint := range checkpoints {
		if checkpoint.Kind == wanted {
			return true
		}
	}
	return false
}

func firstToolOrdinal(turn R03ATurn, wanted string) int {
	for i, event := range turn.ToolEvents {
		if event == wanted {
			return i
		}
	}
	return len(turn.ToolEvents) + 1
}

func stringValue(record map[string]any, key string) string {
	value, _ := record[key].(string)
	return value
}

func r03aPaginationV3BusinessPassed(result map[string]any) bool {
	value, ok := result["medium_started"].(int)
	return ok && value == 3
}

func r03aPaginationV3BackendPrompt() string {
	return `You are the Backend Employee for a disposable Polis peer-collaboration task. Use only the registered Polis peer tools and persisted receipts. Do not use shell, external tools, Planner, delegation or natural-language workarounds. The public contract revision is r03a-pagination-contract@3. Propose and accept the exact public contract document below; this document specifies business semantics only, not implementation code. Then implement a real candidate that consumes cursor and limit, returns a structured items/id/name/next_cursor response, requests the exact returned next cursor for another page, and stops when next_cursor is null. Run workspace_check and use its public result. Persist a qualified checkpoint before artifact_submit. Use collab_send to send the accepted current contract directly to emp-frontend while your task is still working, then submit the artifact. Do not create contract churn.

Public contract document:
` + fixture.PeerPaginationContractV3
}

func r03aPaginationV3FrontendInitialPrompt() string {
	return `You are the initial Frontend Employee for a disposable Polis peer-collaboration task. Use only the registered Polis peer tools. Discover the pending Message and its current ContractRevision yourself with work_current/collab_inbox/contract_read; the controller will not provide the business answer. Do not use shell, external tools, Planner, delegation or natural-language workarounds. Acknowledge the current Message, make a real workspace change implementing the public contract you observed, run workspace_check, and use collab_apply with the current obligation, current contract revision, the new workspace revision and persisted receipt IDs in evidence_refs[]. Then persist a progress checkpoint using receipt IDs. Do not submit an artifact or resolve the obligation; the driver will interrupt once the deterministic handover boundary is reached.`
}

func r03aPaginationV3FrontendSuccessorPrompt() string {
	return `You are the successor Frontend Employee with the same EmployeeId after a neutral handover. Use only the registered Polis peer tools. Restore the persisted current Message, Obligation, ContractRevision and workspace state yourself. Complete the public pagination contract you observed: consume the structured response fields, follow the current next-page cursor exactly, and terminate on null next_cursor. Run workspace_check, persist a qualified checkpoint, submit the candidate and resolve the original obligation with the submitted artifact. Do not use shell, external tools, Planner, delegation, a new contract, or a guessed answer.`
}
