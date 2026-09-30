// pattern: Imperative Shell
package probe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"polis/internal/codex"
	"polis/internal/core"
	"polis/internal/fixture"
	"polis/internal/kernel"
)

func validateR03APaginationV3FrontendSingleConfig(cfg R03APaginationV3Config) error {
	if cfg.ProblemKey != R03AProblemKey || cfg.RunPurpose != R03APaginationV3Purpose {
		return errors.New("R0.3A single-session Frontend ProblemKey or purpose mismatch")
	}
	if cfg.Model != "gpt-5.6-luna" || cfg.MediumLimit != 1 || cfg.HighLimit != 0 || cfg.ToolCallLimit != 48 {
		return errors.New("R0.3A single-session Frontend requires one Medium, zero High and tool_call_limit=48")
	}
	missing := make([]string, 0)
	for name, value := range map[string]string{"evidence": cfg.Evidence, "root": cfg.Root, "dsn": cfg.DSN, "binary": cfg.Binary, "code_mode_host": cfg.CodeModeHost, "auth_file": cfg.AuthFile, "selected_config_path": cfg.SelectedConfigPath, "acceptance_qualification": cfg.AcceptanceQualificationPath, "backend_execution_manifest": cfg.BackendExecutionManifestPath, "backend_l2_live": cfg.BackendL2LivePath, "frontend_execution_manifest": cfg.FrontendExecutionManifestPath, "frontend_l2_live": cfg.FrontendL2LivePath, "frontend_binding": cfg.FrontendBindingPath, "allowance": cfg.AllowancePath, "frontend_result": cfg.FrontendResultPath, "blob_durability": cfg.BlobDurabilityQualificationPath, "checker_feedback": cfg.CheckerFeedbackQualificationPath, "current_l1": cfg.CurrentL1EvidencePath, "baseline_manifest": cfg.FrontendBaselineManifestPath, "baseline_hash": cfg.FrontendBaselineManifestSHA256, "cas_manifest": cfg.FrontendCASManifestPath, "execution_envelope": cfg.ExecutionEnvelopeFingerprint, "database_binding_fingerprint": cfg.RuntimeDatabaseBindingFingerprint, "database_strategy": cfg.DatabaseAccessStrategyRevision, "consumption_qualification": cfg.FrontendConsumptionQualificationPath, "consumption_contract": cfg.FrontendConsumptionContractRevision, "frontend_binding_contract": cfg.FrontendBindingContractRevision, "behavior_verifier": cfg.FrontendBehaviorVerifierRevision} {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("R0.3A single-session Frontend configuration is incomplete: missing=%s", strings.Join(missing, ","))
	}
	if cfg.Evidence == "" || cfg.Root == "" || cfg.DSN == "" || cfg.Binary == "" || cfg.CodeModeHost == "" || cfg.AuthFile == "" || cfg.SelectedConfigPath == "" || cfg.AcceptanceQualificationPath == "" || cfg.BackendExecutionManifestPath == "" || cfg.BackendL2LivePath == "" || cfg.FrontendExecutionManifestPath == "" || cfg.FrontendL2LivePath == "" || cfg.FrontendBindingPath == "" || cfg.AllowancePath == "" || cfg.FrontendResultPath == "" || cfg.BlobDurabilityQualificationPath == "" || cfg.CheckerFeedbackQualificationPath == "" || cfg.CurrentL1EvidencePath == "" || cfg.FrontendBaselineManifestPath == "" || cfg.FrontendBaselineManifestSHA256 == "" || cfg.FrontendCASManifestPath == "" || cfg.ExecutionEnvelopeFingerprint == "" || cfg.RuntimeDatabaseBindingFingerprint == "" || cfg.DatabaseAccessStrategyRevision == "" || cfg.FrontendConsumptionQualificationPath == "" || cfg.FrontendConsumptionContractRevision == "" || cfg.FrontendBindingContractRevision == "" || cfg.FrontendBehaviorVerifierRevision == "" {
		return errors.New("R0.3A single-session Frontend configuration is incomplete")
	}
	if err := validateR03ATransportPolicy(cfg.TransportPolicy); err != nil {
		return err
	}
	if err := cfg.RuntimeCASBinding.ValidateSyntax(); err != nil {
		return fmt.Errorf("R0.3A single-session Frontend runtime CAS binding is invalid: %w", err)
	}
	if err := requireFrontendConsumptionStack(cfg); err != nil {
		return err
	}
	if len(cfg.FrontendRequiredCAS) == 0 {
		return errors.New("R0.3A single-session Frontend required CAS blob set is empty")
	}
	return nil
}

func requireFrontendConsumptionStack(cfg R03APaginationV3Config) error {
	raw, err := os.ReadFile(cfg.FrontendConsumptionQualificationPath)
	if err != nil {
		return fmt.Errorf("Frontend consumption ABI qualification unavailable: %w", err)
	}
	var record struct {
		Status                         string `json:"status"`
		ContractRevision               string `json:"contract_revision"`
		BindingRevision                string `json:"binding_revision"`
		BehaviorVerifierRevision       string `json:"behavior_verifier_revision"`
		FrontendPublicConsumptionABI   string `json:"frontend_public_consumption_abi"`
		FrontendBehaviorObservability  string `json:"frontend_behavior_observability"`
		FrontendCheckerPublicCoherence string `json:"frontend_checker_public_coherence"`
	}
	if err := json.Unmarshal(raw, &record); err != nil {
		return fmt.Errorf("Frontend consumption ABI qualification is invalid: %w", err)
	}
	if record.Status != "PASSED" || record.ContractRevision != fixture.FrontendConsumptionContractRevision || record.BindingRevision != fixture.FrontendBindingContractRevision || record.BehaviorVerifierRevision != fixture.FrontendPaginationBehaviorVerifierRevision || record.FrontendPublicConsumptionABI != "PASSED" || record.FrontendBehaviorObservability != "PASSED" || record.FrontendCheckerPublicCoherence != "PASSED" {
		return errors.New("Frontend consumption ABI qualification is stale or incomplete")
	}
	if cfg.FrontendConsumptionContractRevision != record.ContractRevision || cfg.FrontendBindingContractRevision != record.BindingRevision || cfg.FrontendBehaviorVerifierRevision != record.BehaviorVerifierRevision {
		return errors.New("Frontend consumption ABI revisions are not bound to the qualified stack")
	}
	return nil
}

func requireV3FrontendReusableQualification(cfg R03APaginationV3Config) (R03AT2QualificationReport, error) {
	var manifest currentBinaryL2ExecutionManifest
	if err := readT11JSON(cfg.FrontendExecutionManifestPath, &manifest); err != nil {
		return R03AT2QualificationReport{}, fmt.Errorf("Frontend L2 reusable gate: read manifest: %w", err)
	}
	if manifest.SchemaVersion != r03aFrontendL2ManifestSchema || manifest.Qualification != r03aFrontendL2Qualification || !validCurrentBinaryDigest(manifest.ExecutionFingerprint) || manifest.ToolSurface.ToolCount != 12 {
		return R03AT2QualificationReport{}, errors.New("Frontend L2 reusable gate: manifest is stale")
	}
	if err := manifest.CanonicalManifest.Validate(); err != nil {
		return R03AT2QualificationReport{}, fmt.Errorf("Frontend L2 reusable gate: invalid canonical manifest: %w", err)
	}
	if expected := manifest.CanonicalManifest.Base.Combination.CurrentFingerprintV7(manifest.CanonicalManifest); expected.CanonicalManifestDigest != manifest.CanonicalManifestDigest {
		return R03AT2QualificationReport{}, errors.New("Frontend L2 reusable gate: canonical digest mismatch")
	}
	currentSurface, tools, _, err := buildR03AFrontendToolSurface()
	if err != nil {
		return R03AT2QualificationReport{}, err
	}
	payload, err := json.Marshal(t21NormalizedThreadPayload(tools, r03aT21DeveloperInstruction))
	if err != nil {
		return R03AT2QualificationReport{}, err
	}
	currentSurface.ThreadStartPayloadDigest = digest(payload)
	currentSurface.ThreadStartPayloadBytes = len(payload)
	expectedSurface := manifest.ToolSurface
	currentSurface.CheckpointPolicyRevision = expectedSurface.CheckpointPolicyRevision
	currentSurface.ArtifactEligibilityPolicyRevision = expectedSurface.ArtifactEligibilityPolicyRevision
	currentSurface.ContractSupersessionPolicyRevision = expectedSurface.ContractSupersessionPolicyRevision
	currentSurface.AcceptanceCheckerRevision = expectedSurface.AcceptanceCheckerRevision
	if !reflect.DeepEqual(currentSurface, expectedSurface) {
		return R03AT2QualificationReport{}, errors.New("Frontend L2 reusable gate: provider-visible registration surface drifted")
	}
	var l1 currentBinaryL1Evidence
	if err := readT11JSON(filepath.Join(cfg.CurrentL1EvidencePath, "result.json"), &l1); err != nil {
		return R03AT2QualificationReport{}, fmt.Errorf("Frontend L2 reusable gate: read L1: %w", err)
	}
	if l1.Status != "passed" || l1.CurrentBinaryWindowsL1 != "QUALIFIED" || l1.ExecutionFingerprint != manifest.L1Fingerprint || l1.ControlledRuntimeManifestDigest != manifest.ControlledRuntimeManifestDigest || l1.CodexBinarySHA256 != manifest.CanonicalManifest.Base.Combination.BinarySHA256 || l1.CodeModeHostSHA256 != manifest.CanonicalManifest.Base.Combination.CodeModeHostSHA256 || l1.Model != "gpt-5.6-luna" || l1.Effort != "medium" || l1.RuntimeProfile != "windows-native" || l1.ProviderTransportPolicy != "native_default" || l1.DynamicToolCount != 0 || l1.MediumStarted != 1 || l1.ProviderEgress != 1 || l1.UnresolvedTransportState {
		return R03AT2QualificationReport{}, errors.New("Frontend L2 reusable gate: current L1 is stale")
	}
	var live struct {
		Qualification                         string `json:"qualification"`
		Status                                string `json:"status"`
		ExecutionFingerprint                  string `json:"execution_fingerprint"`
		CurrentFrontendRevisedL2              string `json:"current_frontend_revised_l2"`
		EligibleForRevisedFrontendInitialLive bool   `json:"eligible_for_revised_frontend_initial_live"`
		RegisteredToolCount                   int    `json:"registered_tool_count"`
		ToolManifestDigest                    string `json:"tool_manifest_digest"`
		AggregateSchemaDigest                 string `json:"aggregate_schema_digest"`
		AggregateSchemaBytes                  int    `json:"aggregate_schema_bytes"`
	}
	if err := readT11JSON(cfg.FrontendL2LivePath, &live); err != nil {
		return R03AT2QualificationReport{}, fmt.Errorf("Frontend L2 reusable gate: read live L2: %w", err)
	}
	if live.Qualification != r03aFrontendL2Qualification || live.Status != "passed" || live.ExecutionFingerprint != manifest.ExecutionFingerprint || live.CurrentFrontendRevisedL2 != "QUALIFIED" || !live.EligibleForRevisedFrontendInitialLive || live.RegisteredToolCount != manifest.ToolSurface.ToolCount || live.ToolManifestDigest != manifest.ToolSurface.AggregateManifestDigest || live.AggregateSchemaDigest != manifest.ToolSurface.AggregateSchemaDigest || live.AggregateSchemaBytes != manifest.ToolSurface.AggregateSchemaBytes {
		return R03AT2QualificationReport{}, errors.New("Frontend L2 reusable gate: existing live qualification is not reusable")
	}
	var feedback struct {
		Qualification       string            `json:"qualification"`
		Status              string            `json:"status"`
		RegistrationChanged bool              `json:"provider_visible_registration_changed"`
		PolicyRevisions     map[string]string `json:"policy_revisions"`
	}
	if err := readT11JSON(cfg.CheckerFeedbackQualificationPath, &feedback); err != nil {
		return R03AT2QualificationReport{}, fmt.Errorf("Frontend checker qualification: %w", err)
	}
	if feedback.Qualification != "R0.3A-FRONTEND-CHECKER-FEEDBACK-L2" || feedback.Status != "PASSED" || feedback.RegistrationChanged || feedback.PolicyRevisions["acceptance_checker_revision"] != core.PeerAcceptanceCheckerRevision {
		return R03AT2QualificationReport{}, errors.New("Frontend checker qualification is stale")
	}
	authRaw, err := os.ReadFile(cfg.AuthFile)
	if err != nil {
		return R03AT2QualificationReport{}, err
	}
	auth, err := codex.ParseAuthMaterial(authRaw, manifest.CanonicalManifest.Base.Auth.AuthSourceClass)
	if err != nil || !reflect.DeepEqual(auth.Manifest(), manifest.CanonicalManifest.Base.Auth) {
		return R03AT2QualificationReport{}, errors.New("Frontend L2 reusable gate: auth drifted")
	}
	return R03AT2QualificationReport{Qualification: manifest.Qualification, ExecutionFingerprint: manifest.ExecutionFingerprint, CodexVersion: manifest.CanonicalManifest.Base.Combination.CodexVersion, Model: manifest.CanonicalManifest.Base.Combination.Model, Effort: manifest.CanonicalManifest.Base.Combination.Effort, CapabilityDigest: manifest.CanonicalManifest.Base.Combination.CapabilityDigest, ToolManifestDigest: manifest.ToolSurface.AggregateManifestDigest, ToolCount: manifest.ToolSurface.ToolCount, AuthSourceClass: auth.SourceClass, AuthIdentityFingerprint: auth.IdentityFingerprint, AuthCredentialRevision: auth.CredentialRevisionFingerprint, BehavioralContractQualification: "passed", RevalidatedAt: time.Now().UTC()}, nil
}

func requireR03APaginationV3FrontendSingleQualification(cfg R03APaginationV3Config) (r03aPaginationV3Qualification, error) {
	acceptance, err := loadR03APaginationV3Acceptance(cfg.AcceptanceQualificationPath)
	if err != nil {
		return r03aPaginationV3Qualification{}, err
	}
	backendFingerprint, err := executionFingerprintFromManifest(cfg.BackendExecutionManifestPath)
	if err != nil {
		return r03aPaginationV3Qualification{}, err
	}
	backendCfg := R03AT2Config{Config: cfg.Config, ProblemKey: cfg.ProblemKey, RunPurpose: cfg.RunPurpose, ExecutionFingerprint: backendFingerprint}
	backendCfg.ExecutionManifestPath = cfg.BackendExecutionManifestPath
	backendCfg.QualificationPath = cfg.BackendL2LivePath
	backend, err := requireV3BackendL2Qualification(backendCfg)
	if err != nil {
		return r03aPaginationV3Qualification{}, err
	}
	frontend, err := requireV3FrontendReusableQualification(cfg)
	if err != nil {
		return r03aPaginationV3Qualification{}, err
	}
	blob, err := RequireR03AT2BlobDurabilityQualification(cfg.BlobDurabilityQualificationPath)
	if err != nil || blob.Status != "PASSED" || blob.MediumConsumed != 0 || blob.HighConsumed != 0 || blob.ProviderEgress != 0 {
		if err != nil {
			return r03aPaginationV3Qualification{}, err
		}
		return r03aPaginationV3Qualification{}, errors.New("blob qualification is not zero-egress passed evidence")
	}
	return r03aPaginationV3Qualification{Acceptance: acceptance, Backend: backend, Frontend: frontend, Blob: blob}, nil
}

func paginationV3FrontendSingleBinding(fingerprint string, cfg R03APaginationV3Config) codex.BusinessAuthorizationBinding {
	return codex.BusinessAuthorizationBinding{ExecutionFingerprint: fingerprint, EmployeeID: "emp-frontend", ProblemKey: cfg.ProblemKey, Purpose: cfg.RunPurpose, Model: cfg.Model, Profile: cfg.Model + "/medium", Effort: "medium", Limits: codex.AllowanceLimits{Medium: 1, High: 0, Concurrency: 1, ToolCallLimit: 48}, TransportPolicyRevision: cfg.TransportPolicy.Revision, TransportPolicy: cfg.TransportPolicy.Snapshot(), DatabaseBindingFingerprint: cfg.RuntimeDatabaseBindingFingerprint, DatabaseAccessStrategyRevision: cfg.DatabaseAccessStrategyRevision, FrontendConsumptionContractRevision: cfg.FrontendConsumptionContractRevision, FrontendBindingContractRevision: cfg.FrontendBindingContractRevision, FrontendBehaviorVerifierRevision: cfg.FrontendBehaviorVerifierRevision}
}

func InspectR03APaginationV3FrontendSingle(cfg R03APaginationV3Config) error {
	if err := validateR03APaginationV3FrontendSingleConfig(cfg); err != nil {
		return err
	}
	if entries, err := os.ReadDir(cfg.Evidence); err == nil && len(entries) != 0 {
		return errors.New("single-session Frontend evidence path is not fresh")
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	if _, err := os.Stat(cfg.AllowancePath); err == nil {
		return errors.New("single-session Frontend allowance already exists")
	}
	qualification, err := requireR03APaginationV3FrontendSingleQualification(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.Evidence, 0700); err != nil {
		return err
	}
	return writeJSON(filepath.Join(cfg.Evidence, "preflight.json"), map[string]any{"qualification": "R0.3A-PAGINATION-V3-FRONTEND-SINGLE-SESSION", "status": "preflight_passed", "passed": true, "parent_problem_key": cfg.ProblemKey, "purpose": cfg.RunPurpose, "subject_revision": R03APaginationV3SubjectRevision, "contract_revision": fixture.PeerPaginationContractRevision, "behavior_verifier": fixture.PeerPaginationBehaviorVerifierRevisionV2, "frontend_l2_status": "QUALIFIED_REUSABLE", "execution_fingerprint": qualification.Frontend.ExecutionFingerprint, "tool_manifest_digest": qualification.Frontend.ToolManifestDigest, "tool_count": qualification.Frontend.ToolCount, "medium": 0, "high": 0, "provider_egress": 0, "historical_evidence_modified": false})
}

func RecordR03APaginationV3FrontendSingleFreshness(cfg R03APaginationV3Config) error {
	if err := validateR03APaginationV3FrontendSingleConfig(cfg); err != nil {
		return err
	}
	preflightRaw, err := os.ReadFile(filepath.Join(cfg.Evidence, "preflight.json"))
	if err != nil {
		return err
	}
	var preflight map[string]any
	if json.Unmarshal(preflightRaw, &preflight) != nil || preflight["passed"] != true {
		return errors.New("single-session Frontend preflight invalid")
	}
	_, err = requireR03APaginationV3FrontendSingleQualification(cfg)
	if err != nil {
		return err
	}
	binding := paginationV3FrontendSingleBinding(cfg.ExecutionEnvelopeFingerprint, cfg)
	context := codex.BusinessExecutionContext{ExecutionFingerprint: binding.ExecutionFingerprint, EmployeeID: binding.EmployeeID, ProblemKey: binding.ProblemKey, Purpose: binding.Purpose, Model: binding.Model, Profile: binding.Profile, Effort: binding.Effort, Limits: binding.Limits, TransportPolicyRevision: binding.TransportPolicyRevision, TransportPolicy: binding.TransportPolicy, DatabaseBindingFingerprint: cfg.RuntimeDatabaseBindingFingerprint, DatabaseAccessStrategyRevision: cfg.DatabaseAccessStrategyRevision, FrontendConsumptionContractRevision: cfg.FrontendConsumptionContractRevision, FrontendBindingContractRevision: cfg.FrontendBindingContractRevision, FrontendBehaviorVerifierRevision: cfg.FrontendBehaviorVerifierRevision}
	if decision := codex.AuthorizeBusinessExecution(binding, context); !decision.Allowed {
		return fmt.Errorf("single-session Frontend authorization rejected: %s", decision.ReasonCode)
	}
	if err := writeJSON(cfg.FrontendBindingPath, binding); err != nil {
		return err
	}
	return writeJSON(filepath.Join(cfg.Evidence, "continuation-freshness.json"), map[string]any{"record_type": "r0.3a-pagination-v3-frontend-single-freshness-v1", "parent_problem_key": cfg.ProblemKey, "purpose": cfg.RunPurpose, "execution_fingerprint": binding.ExecutionFingerprint, "transport_policy": cfg.TransportPolicy.Snapshot(), "employee_id": "emp-frontend", "contract_revision": fixture.PeerPaginationContractRevision, "behavior_verifier": fixture.PeerPaginationBehaviorVerifierRevisionV2, "tool_call_limit": 48, "medium_limit": 1, "high_limit": 0, "provider_surface_changed": false, "frontend_l2_status": "QUALIFIED_REUSABLE", "medium": 0, "high": 0, "provider_egress": 0, "historical_evidence_modified": false})
}

func RunR03APaginationV3FrontendSingle(cfg R03APaginationV3Config) (result map[string]any, err error) {
	result = map[string]any{"qualification": "R0.3A-PAGINATION-V3-FRONTEND-SINGLE-SESSION", "status": "NOT_STARTED", "parent_problem_key": cfg.ProblemKey, "employee_id": "emp-frontend", "high_started": 0, "provider_egress": 0, "successor": "NOT_NEEDED", "historical_evidence_modified": false}
	writeResult := func() {
		refreshFrontendTerminalStatus(result)
		_ = writeJSON(cfg.FrontendResultPath, result)
	}
	if err = validateR03APaginationV3FrontendSingleConfig(cfg); err != nil {
		writeResult()
		return result, err
	}
	if err = requireV3Records(cfg); err != nil {
		writeResult()
		return result, err
	}
	qualification, err := requireR03APaginationV3FrontendSingleQualification(cfg)
	if err != nil {
		writeResult()
		return result, err
	}
	if _, err := os.Stat(cfg.AllowancePath); err == nil {
		writeResult()
		return result, errors.New("single-session Frontend terminal allowance already exists")
	}
	preflightCtx, preflightCancel := context.WithTimeout(context.Background(), 2*time.Minute)
	activationPreflight, preflightErr := FrontendActivationPreflight(preflightCtx, cfg.DSN, cfg.FrontendBaselineManifestPath, cfg.FrontendBaselineManifestSHA256, cfg.RuntimeCASBinding, cfg.FrontendRequiredCAS)
	preflightCancel()
	if err := writeJSON(filepath.Join(cfg.Evidence, "frontend-activation-preflight.json"), activationPreflight); err != nil {
		writeResult()
		return result, err
	}
	if preflightErr != nil {
		writeResult()
		return result, preflightErr
	}
	runtimePreflightCtx, runtimePreflightCancel := context.WithTimeout(context.Background(), 2*time.Minute)
	runtimePreflight, runtimePreflightErr := FrontendRuntimeActivationPreflight(runtimePreflightCtx, cfg)
	runtimePreflightCancel()
	if err := writeJSON(filepath.Join(cfg.Evidence, "frontend-runtime-preflight.json"), runtimePreflight); err != nil {
		writeResult()
		return result, err
	}
	if runtimePreflightErr != nil {
		writeResult()
		return result, runtimePreflightErr
	}
	if err := ValidateExecutionEnvelopeFingerprint(cfg.ExecutionEnvelopeFingerprint, runtimePreflight.ExecutionEnvelopeFingerprint); err != nil {
		writeResult()
		return result, err
	}
	result["execution_envelope_fingerprint"] = runtimePreflight.ExecutionEnvelopeFingerprint
	binding, err := LoadBusinessAuthorization(cfg.FrontendBindingPath, codex.BusinessExecutionContext{ExecutionFingerprint: cfg.ExecutionEnvelopeFingerprint, EmployeeID: "emp-frontend", ProblemKey: cfg.ProblemKey, Purpose: cfg.RunPurpose, Model: cfg.Model, Profile: cfg.Model + "/medium", Effort: "medium", Limits: codex.AllowanceLimits{Medium: 1, High: 0, Concurrency: 1, ToolCallLimit: 48}, TransportPolicyRevision: cfg.TransportPolicy.Revision, TransportPolicy: cfg.TransportPolicy.Snapshot(), DatabaseBindingFingerprint: cfg.RuntimeDatabaseBindingFingerprint, DatabaseAccessStrategyRevision: cfg.DatabaseAccessStrategyRevision, FrontendConsumptionContractRevision: cfg.FrontendConsumptionContractRevision, FrontendBindingContractRevision: cfg.FrontendBindingContractRevision, FrontendBehaviorVerifierRevision: cfg.FrontendBehaviorVerifierRevision})
	if err != nil {
		writeResult()
		return result, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	startingState, probeErr := kernel.ProbeFrontendStartingState(ctx, cfg.DSN)
	startingReasons := kernel.ValidateFrontendStartingState(startingState)
	if err := writeJSON(filepath.Join(cfg.Evidence, "starting-state-probe.json"), map[string]any{"status": map[bool]string{true: "READY", false: "MISMATCH"}[probeErr == nil && len(startingReasons) == 0], "read_only": true, "state": startingState, "reasons": startingReasons, "error": fmt.Sprint(probeErr), "mutation": false}); err != nil {
		writeResult()
		return result, err
	}
	if probeErr != nil || len(startingReasons) != 0 {
		writeResult()
		if probeErr != nil {
			return result, fmt.Errorf("FrontendStartingStateProbe failed: %w", probeErr)
		}
		return result, fmt.Errorf("FrontendStartingStateProbe rejected state: %s", strings.Join(startingReasons, ","))
	}
	budget, err := codex.NewBusinessBudget(cfg.AllowancePath, 1, 0, 48)
	if err != nil {
		writeResult()
		return result, err
	}
	result["allowance_created"], result["allowance_binding"], result["tool_call_limit"] = true, binding, 48
	if err = writeJSON(filepath.Join(cfg.Evidence, "allowance-binding.json"), map[string]any{"binding": binding, "execution_envelope_fingerprint": runtimePreflight.ExecutionEnvelopeFingerprint, "transport_policy": cfg.TransportPolicy.Snapshot(), "allowance_path": cfg.AllowancePath, "historical_evidence_modified": false}); err != nil {
		writeResult()
		return result, err
	}
	k, err := kernel.OpenWithRuntimeCASBinding(ctx, cfg.DSN, cfg.RuntimeCASBinding)
	if err != nil {
		writeResult()
		return result, err
	}
	defer k.Close()
	anchors, err := k.PeerRecoveryAnchors(ctx, "")
	if err != nil {
		writeResult()
		return result, err
	}
	scope := k.LocalScope(anchors.CompanyID)
	plannerRelay, err := k.PeerPlannerPathCount(ctx, scope)
	if err != nil {
		writeResult()
		return result, err
	}
	workspace, err := k.PeerWorkspaceAt(ctx, scope, anchors.FrontendTaskID)
	if err != nil {
		writeResult()
		return result, err
	}
	if plannerRelay != 0 || anchors.MessageState != "persisted" || anchors.ObligationState != "pending" || anchors.ObligationOwner != "emp-frontend" || anchors.MessageContractRevisionID != anchors.FinalContractRevisionID || anchors.BackendSessionState != "stopped" {
		writeResult()
		return result, errors.New("post-probe authoritative anchor changed before Worker creation")
	}
	if err = k.VerifyHistoricalWorkerFence(ctx, anchors.CompanyID, anchors.BackendSessionID, "emp-backend", anchors.BackendSessionIncarnation, anchors.BackendSessionEpoch); err != nil {
		writeResult()
		return result, err
	}
	worker, err := k.TXNewWorkerWithToolBudget(ctx, scope, anchors.FrontendTaskID, cfg.Model+"/medium", 48)
	if err != nil {
		writeResult()
		return result, err
	}
	if err = budget.Reserve("medium"); err != nil {
		writeResult()
		return result, err
	}
	result["medium_started"] = 1
	stageCfg := v3StageConfig(cfg, qualification.Frontend, cfg.FrontendBindingPath)
	session, sessionErr := runPeerSession(ctx, R03AConfig{Config: stageCfg.Config, ProblemKey: cfg.ProblemKey}, k, worker, "peer_frontend", r03aPaginationV3FrontendCompletionPrompt(), false, true, 1, "pagination V3 Frontend single-session employee")
	result["frontend_session"], result["provider_egress"] = session.turn, boolInt(session.turn.ProviderEgress)
	result["transport_outcome_classification"] = session.turn.OutcomeClassification
	if sessionErr != nil {
		if frontendTransportIsInconclusive(session.turn.OutcomeClassification) {
			result["frontend_real_execution"], result["real_peer_collaboration_v3"] = "INCONCLUSIVE", "INCONCLUSIVE"
			result["failure_class"] = "PROVIDER_TRANSPORT_INCONCLUSIVE"
		} else {
			result["frontend_real_execution"], result["real_peer_collaboration_v3"] = "FAILED", "FAILED"
		}
		if session.turn.TurnCompleted {
			result["frontend_provider_turn"] = "COMPLETED"
		} else if session.turn.TurnStarted || session.turn.ProviderEgress {
			result["frontend_provider_turn"] = "INCONCLUSIVE"
		}
		err = sessionErr
		writeResult()
		return result, err
	}
	result["frontend_provider_turn"] = "COMPLETED"
	finalAnchors, err := k.PeerRecoveryAnchors(ctx, anchors.CompanyID)
	if err != nil {
		finalizeCompletedFrontendFailure(result, session.turn, err)
		writeResult()
		return result, err
	}
	artifact, err := k.PeerArtifactAt(ctx, scope, session.turn.ArtifactID)
	if err != nil {
		finalizeCompletedFrontendFailure(result, session.turn, err)
		writeResult()
		return result, err
	}
	finalWorkspace, err := k.PeerWorkspaceAt(ctx, scope, anchors.FrontendTaskID)
	if err != nil {
		finalizeCompletedFrontendFailure(result, session.turn, err)
		writeResult()
		return result, err
	}
	contract, err := k.PeerContractAt(ctx, scope, anchors.FinalContractRevisionID)
	if err != nil {
		finalizeCompletedFrontendFailure(result, session.turn, err)
		writeResult()
		return result, err
	}
	if !turnHas(session.turn, "work_current") || !turnHas(session.turn, "collab_inbox") || !turnHas(session.turn, "collab_ack") || !turnHas(session.turn, "contract_read") || !turnHas(session.turn, "workspace_replace") || !turnHas(session.turn, "workspace_check") || !turnHas(session.turn, "work_checkpoint") || !turnHas(session.turn, "artifact_submit") || !turnHas(session.turn, "obligation_resolve") || session.turn.ArtifactID == "" || session.turn.CheckpointID == "" || finalAnchors.MessageState != "resolved" || finalAnchors.ObligationState != "fulfilled" || artifact.State != "ready" || artifact.Verdict != "candidate" || contract.State != "accepted" || finalWorkspace.Revision <= workspace.Revision {
		result["frontend_real_execution"], result["real_peer_collaboration_v3"], err = "FAILED", "FAILED", errors.New("single-session Frontend did not complete authoritative lifecycle")
		writeResult()
		return result, err
	}
	integration, err := k.TXFreezePeerIntegration(ctx, scope, kernel.PeerIntegrationInput{Mission: anchors.MissionID, BackendArtifactID: anchors.BackendArtifactID, FrontendArtifactID: artifact.ID, ContractRevisionID: anchors.FinalContractRevisionID, BaseRevision: "api-v1", VerifierRevision: fixture.PeerPaginationBehaviorVerifierRevisionV2}, "v3-single-positive-integration")
	if err != nil {
		writeResult()
		return result, err
	}
	positive, err := k.VerifyPeerIntegration(ctx, scope, integration.ID)
	if err != nil {
		writeResult()
		return result, err
	}
	if !positive.Passed || positive.PaginationRuntimeBehavior != "passed" || positive.ResponseShapeCompatibility != "passed" || positive.LifecycleBinding != "passed" || positive.CollaborationCausality != "passed" {
		result["frontend_real_execution"], result["real_peer_collaboration_v3"], err = "FAILED", "FAILED", errors.New("single-session Frontend integration dimensions failed")
		writeResult()
		return result, err
	}
	negative, err := runPeerNegativeControl(ctx, k)
	if err != nil {
		writeResult()
		return result, err
	}
	if negative.Passed {
		result["frontend_real_execution"], result["real_peer_collaboration_v3"], err = "FAILED", "FAILED", errors.New("single-session negative control unexpectedly passed")
		writeResult()
		return result, err
	}
	budgetState := budget.ToolBudget()
	result["status"], result["frontend_real_execution"], result["real_peer_collaboration_v3"] = "PASSED", "PASSED", "PASSED"
	result["peer_message_discovered"], result["obligation_observed"], result["contract_consumed"] = "PASSED", "PASSED", "PASSED"
	result["workspace_implementation"], result["frontend_semantic_acceptance"], result["pagination_integration"] = "PASSED", "PASSED", "PASSED"
	result["qualified_checkpoint"], result["frontend_artifact"], result["obligation_fulfilled"] = "PASSED", "PASSED", "PASSED"
	result["planner_relay"], result["successor_required"] = 0, "NO"
	result["message_id"], result["obligation_id"], result["contract_revision_id"] = finalAnchors.MessageID, finalAnchors.ObligationID, finalAnchors.FinalContractRevisionID
	result["workspace_revision"], result["frontend_artifact_id"], result["frontend_artifact_digest"] = finalWorkspace.Revision, artifact.ID, artifact.Digest
	result["tool_calls_used"], result["tool_calls_remaining"] = budgetState.Used, budgetState.Remaining
	result["integration_verifier"], result["negative_control"] = positive, "FAIL_AS_EXPECTED"
	if err = writeJSON(filepath.Join(cfg.Evidence, "integration-verifier.json"), positive); err != nil {
		return result, err
	}
	if err = writeJSON(filepath.Join(cfg.Evidence, "negative-control.json"), negative); err != nil {
		return result, err
	}
	writeResult()
	return result, nil
}

func finalizeCompletedFrontendFailure(result map[string]any, turn R03ATurn, finalizerErr error) {
	result["frontend_provider_turn"] = "COMPLETED"
	result["frontend_real_execution"] = "FAILED"
	result["real_peer_collaboration_v3"] = "FAILED"
	result["failure_class"] = "BUSINESS_FINALIZATION_FAILED"
	result["finalizer_error"] = finalizerErr.Error()
	result["frontend_artifact"] = "NOT_CREATED"
	result["artifact_state"] = "NOT_CREATED"
	result["qualified_checkpoint"] = "NOT_CONFIRMED"
	result["obligation_fulfilled"] = "NO"
	result["successor_required"] = "NO"
	result["planner_relay"] = 0
	result["tool_calls_used"] = turn.ToolCallsUsed
	result["tool_calls_remaining"] = turn.ToolCallsRemaining
	refreshFrontendTerminalStatus(result)
}

func refreshFrontendTerminalStatus(result map[string]any) {
	result["status"] = DeriveFrontendTerminalStatus(FrontendTerminalExecution{
		AllowanceConsumed:        frontendResultBool(result, "allowance_created"),
		SessionCreated:           frontendResultSessionCreated(result),
		ProviderEgress:           frontendResultInt(result, "provider_egress") > 0,
		BusinessExecutionEntered: frontendResultBool(result, "allowance_created"),
		BusinessVerdict:          frontendResultString(result, "frontend_real_execution"),
		TransportOutcome:         frontendResultString(result, "transport_outcome_classification"),
	})
}

func frontendResultBool(result map[string]any, key string) bool {
	value, ok := result[key].(bool)
	return ok && value
}

func frontendResultInt(result map[string]any, key string) int {
	value, ok := result[key].(int)
	if ok {
		return value
	}
	value64, ok := result[key].(int64)
	if ok {
		return int(value64)
	}
	return 0
}

func frontendResultString(result map[string]any, key string) string {
	value, _ := result[key].(string)
	return value
}

func frontendResultSessionCreated(result map[string]any) bool {
	switch session := result["frontend_session"].(type) {
	case R03ATurn:
		return session.SessionID != "" || session.TurnStarted || session.ProviderEgress
	case *R03ATurn:
		return session != nil && (session.SessionID != "" || session.TurnStarted || session.ProviderEgress)
	default:
		return result["frontend_session"] != nil
	}
}

func r03aPaginationV3FrontendCompletionPrompt() string {
	return `You are the Frontend Employee in a recovered disposable Polis peer-collaboration task. Use only the registered Polis peer tools and complete the entire task in this single session. Discover the current Message, pending Obligation, current ContractRevision, Frontend task and workspace yourself through the formal work/context/collaboration tools; the controller will not provide the business answer. Do not use shell, external tools, Planner, delegation or natural-language workarounds. Observe and acknowledge the peer Message, read and consume the public r03a-pagination-contract@3 and the public r03a-frontend-consumption-contract@1 with r03a-frontend-binding@1. Implement the declared step ABI: consume response items.id and items.name, consume next_cursor, emit the exact next request cursor and requested limit when next_cursor is non-null, and emit stop when next_cursor is null. Use the public r03a-frontend-pagination-behavior@1 feedback. Run workspace_check and use its structured public feedback. Apply the current obligation with obligation_id, contract_revision_id, workspace_revision and receipt IDs in evidence_refs[]. Persist a qualified checkpoint, submit the checked candidate artifact, and resolve the original obligation. Do not stop at a progress checkpoint, create a new contract, or guess a hidden answer.`
}
