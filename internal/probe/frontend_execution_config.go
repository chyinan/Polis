// pattern: Functional Core
package probe

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"polis/internal/codex"
	"polis/internal/kernel"
	"strings"
)

type FrontendExecutionConfig struct {
	PathEncoding                      string                   `json:"path_encoding"`
	DSN                               string                   `json:"dsn"`
	Binary                            string                   `json:"binary"`
	CodeModeHost                      string                   `json:"code_mode_host"`
	AuthFile                          string                   `json:"auth_file"`
	SelectedConfigPath                string                   `json:"selected_config_path"`
	ExecutionConfigPath               string                   `json:"execution_config_path"`
	ExecutionManifestPath             string                   `json:"execution_manifest_path"`
	QualificationPath                 string                   `json:"qualification_path"`
	BlobDurabilityQualificationPath   string                   `json:"blob_durability_qualification_path"`
	BehavioralContractPath            string                   `json:"behavioral_contract_path"`
	CheckerFeedbackQualificationPath  string                   `json:"checker_feedback_qualification_path"`
	PostgresDumpPath                  string                   `json:"postgres_dump_path"`
	PostgresSnapshotDSN               string                   `json:"postgres_snapshot_dsn"`
	RuntimeRoot                       string                   `json:"runtime_root"`
	EvidenceRoot                      string                   `json:"evidence_root"`
	RecoveryPackageRoot               string                   `json:"recovery_package_root"`
	SourceCASRoot                     string                   `json:"source_cas_root"`
	RuntimeArtifactManifestPath       string                   `json:"runtime_artifact_manifest_path"`
	RuntimeCASBinding                 kernel.RuntimeCASBinding `json:"runtime_cas_binding"`
	RuntimeDatabaseBindingPath        string                   `json:"runtime_database_binding_path"`
	DatabaseAccessViewPath            string                   `json:"database_access_view_path"`
	RuntimeDatabaseBindingFingerprint string                   `json:"runtime_database_binding_fingerprint"`
	DatabaseAccessStrategyRevision    string                   `json:"database_access_strategy_revision"`
	AuthorizationBindingPath          string                   `json:"authorization_binding_path"`
	CurrentL1EvidencePath             string                   `json:"current_l1_evidence_path"`
	ExecutionFingerprint              string                   `json:"execution_fingerprint"`
	CurrentL1Fingerprint              string                   `json:"current_l1_fingerprint"`
	HandoverBoundaryEvidencePath      string                   `json:"handover_boundary_evidence_path"`
	HandoverBoundarySnapshotPath      string                   `json:"handover_boundary_snapshot_path"`
	ProblemKey                        string                   `json:"problem_key"`
	Purpose                           string                   `json:"purpose"`
	EmployeeID                        string                   `json:"employee_id"`
	Model                             string                   `json:"model"`
	Effort                            string                   `json:"effort"`
	ToolCallLimit                     int                      `json:"tool_call_limit"`
	MediumLimit                       int                      `json:"medium_limit"`
	HighLimit                         int                      `json:"high_limit"`
	Concurrency                       int                      `json:"concurrency"`
	Retry                             bool                     `json:"retry"`
	Reset                             bool                     `json:"reset"`
	TransportPolicy                   codex.TransportPolicy    `json:"transport_policy"`
}

type FrontendExecutionConfigProbeReport struct {
	Status               string                        `json:"status"`
	Missing              []string                      `json:"missing,omitempty"`
	Invalid              []string                      `json:"invalid,omitempty"`
	SecretSourcesPresent map[string]bool               `json:"secret_sources_present"`
	ConfigDigest         string                        `json:"config_digest"`
	PathEncoding         string                        `json:"path_encoding"`
	EmployeeID           string                        `json:"employee_id,omitempty"`
	ProblemKey           string                        `json:"problem_key,omitempty"`
	Purpose              string                        `json:"purpose,omitempty"`
	ExecutionFingerprint string                        `json:"execution_fingerprint,omitempty"`
	ToolCallLimit        int                           `json:"tool_call_limit,omitempty"`
	MediumLimit          int                           `json:"medium_limit,omitempty"`
	HighLimit            int                           `json:"high_limit,omitempty"`
	Concurrency          int                           `json:"concurrency,omitempty"`
	TransportPolicy      codex.TransportPolicySnapshot `json:"transport_policy,omitempty"`
	PathDiagnostics      []ExecutionPathDiagnostic     `json:"path_diagnostics,omitempty"`
}

func LoadFrontendExecutionConfig(path string) (FrontendExecutionConfig, FrontendExecutionConfigProbeReport, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return FrontendExecutionConfig{}, FrontendExecutionConfigProbeReport{Status: "FRONTEND_EXECUTION_CONFIG_INVALID", Invalid: []string{"config_file_unreadable"}}, err
	}
	var cfg FrontendExecutionConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return cfg, FrontendExecutionConfigProbeReport{Status: "FRONTEND_EXECUTION_CONFIG_INVALID", Invalid: []string{"malformed_json"}}, err
	}
	report := ValidateFrontendExecutionConfig(cfg, true)
	if report.ConfigDigest == "" {
		h := sha256.Sum256(raw)
		report.ConfigDigest = hex.EncodeToString(h[:])
	}
	if report.Status != "FRONTEND_EXECUTION_CONFIG_READY" {
		return cfg, report, errors.New("FRONTEND_EXECUTION_CONFIG_INVALID")
	}
	return cfg, report, nil
}

func ValidateFrontendExecutionConfig(cfg FrontendExecutionConfig, checkPaths bool) FrontendExecutionConfigProbeReport {
	report := FrontendExecutionConfigProbeReport{Status: "FRONTEND_EXECUTION_CONFIG_READY", SecretSourcesPresent: map[string]bool{"dsn": cfg.DSN != "", "postgres_snapshot_dsn": cfg.PostgresSnapshotDSN != "", "auth_file": cfg.AuthFile != ""}, PathEncoding: "wsl_absolute"}
	if cfg.PathEncoding != "wsl" && cfg.PathEncoding != "windows-native" {
		report.Invalid = append(report.Invalid, "path_encoding")
	}
	required := map[string]string{"dsn": cfg.DSN, "binary": cfg.Binary, "code_mode_host": cfg.CodeModeHost, "auth_file": cfg.AuthFile, "selected_config_path": cfg.SelectedConfigPath, "execution_config_path": cfg.ExecutionConfigPath, "execution_manifest_path": cfg.ExecutionManifestPath, "qualification_path": cfg.QualificationPath, "blob_durability_qualification_path": cfg.BlobDurabilityQualificationPath, "behavioral_contract_path": cfg.BehavioralContractPath, "checker_feedback_qualification_path": cfg.CheckerFeedbackQualificationPath, "postgres_dump_path": cfg.PostgresDumpPath, "postgres_snapshot_dsn": cfg.PostgresSnapshotDSN, "runtime_root": cfg.RuntimeRoot, "evidence_root": cfg.EvidenceRoot, "recovery_package_root": cfg.RecoveryPackageRoot, "source_cas_root": cfg.SourceCASRoot, "runtime_artifact_manifest_path": cfg.RuntimeArtifactManifestPath, "authorization_binding_path": cfg.AuthorizationBindingPath, "current_l1_evidence_path": cfg.CurrentL1EvidencePath, "execution_fingerprint": cfg.ExecutionFingerprint, "current_l1_fingerprint": cfg.CurrentL1Fingerprint, "handover_boundary_evidence_path": cfg.HandoverBoundaryEvidencePath, "handover_boundary_snapshot_path": cfg.HandoverBoundarySnapshotPath, "problem_key": cfg.ProblemKey, "purpose": cfg.Purpose, "employee_id": cfg.EmployeeID, "model": cfg.Model, "effort": cfg.Effort}
	for name, value := range required {
		if strings.TrimSpace(value) == "" {
			report.Missing = append(report.Missing, name)
		}
	}
	if cfg.ProblemKey != R03AProblemKey {
		report.Invalid = append(report.Invalid, "problem_key")
	}
	if cfg.Purpose != r03aFrontendHandoverPurpose && cfg.Purpose != R03APaginationV3Purpose {
		report.Invalid = append(report.Invalid, "purpose")
	}
	if cfg.Purpose == R03APaginationV3Purpose {
		if err := cfg.RuntimeCASBinding.ValidateSyntax(); err != nil {
			report.Invalid = append(report.Invalid, "runtime_cas_binding")
		}
		if err := cfg.TransportPolicy.Validate(); err != nil || cfg.TransportPolicy.Revision != codex.TransportPolicyRevision {
			report.Invalid = append(report.Invalid, "transport_policy")
		}
		for name, value := range map[string]string{"runtime_database_binding_path": cfg.RuntimeDatabaseBindingPath, "database_access_view_path": cfg.DatabaseAccessViewPath, "runtime_database_binding_fingerprint": cfg.RuntimeDatabaseBindingFingerprint, "database_access_strategy_revision": cfg.DatabaseAccessStrategyRevision} {
			if strings.TrimSpace(value) == "" {
				report.Missing = append(report.Missing, name)
			}
		}
	}
	if cfg.EmployeeID != "emp-frontend" {
		report.Invalid = append(report.Invalid, "employee_id")
	}
	if cfg.Model != "gpt-5.6-luna" || cfg.Effort != "medium" {
		report.Invalid = append(report.Invalid, "model_effort")
	}
	if cfg.ToolCallLimit != 48 || cfg.MediumLimit != 1 || cfg.HighLimit != 0 || cfg.Concurrency != 1 || cfg.Retry || cfg.Reset {
		report.Invalid = append(report.Invalid, "allowance")
	}
	pathFields := map[string]string{"binary": cfg.Binary, "code_mode_host": cfg.CodeModeHost, "auth_file": cfg.AuthFile, "selected_config_path": cfg.SelectedConfigPath, "execution_config_path": cfg.ExecutionConfigPath, "execution_manifest_path": cfg.ExecutionManifestPath, "qualification_path": cfg.QualificationPath, "blob_durability_qualification_path": cfg.BlobDurabilityQualificationPath, "behavioral_contract_path": cfg.BehavioralContractPath, "checker_feedback_qualification_path": cfg.CheckerFeedbackQualificationPath, "postgres_dump_path": cfg.PostgresDumpPath, "runtime_root": cfg.RuntimeRoot, "evidence_root": cfg.EvidenceRoot, "recovery_package_root": cfg.RecoveryPackageRoot, "source_cas_root": cfg.SourceCASRoot, "runtime_artifact_manifest_path": cfg.RuntimeArtifactManifestPath, "current_l1_evidence_path": cfg.CurrentL1EvidencePath, "handover_boundary_evidence_path": cfg.HandoverBoundaryEvidencePath, "handover_boundary_snapshot_path": cfg.HandoverBoundarySnapshotPath}
	for name, path := range pathFields {
		validEncoding := isWSLAbsolute(path)
		if cfg.PathEncoding == "windows-native" {
			validEncoding = isWindowsAbsolute(path)
		}
		if !validEncoding {
			report.Invalid = append(report.Invalid, name+"_path_encoding")
		} else if checkPaths && name != "runtime_root" && name != "evidence_root" && name != "handover_boundary_evidence_path" && name != "handover_boundary_snapshot_path" {
			if _, err := os.Stat(path); err != nil {
				report.Invalid = append(report.Invalid, name+"_path_unavailable")
			}
		}
	}
	if cfg.Purpose == R03APaginationV3Purpose {
		for name, path := range map[string]string{"runtime_database_binding_path": cfg.RuntimeDatabaseBindingPath, "database_access_view_path": cfg.DatabaseAccessViewPath} {
			validEncoding := isWSLAbsolute(path)
			if cfg.PathEncoding == "windows-native" {
				validEncoding = isWindowsAbsolute(path)
			}
			if !validEncoding {
				report.Invalid = append(report.Invalid, name+"_path_encoding")
			} else if checkPaths {
				if _, err := os.Stat(path); err != nil {
					report.Invalid = append(report.Invalid, name+"_path_unavailable")
				}
			}
		}
	}
	futureAuthorizationOutput := cfg.Purpose == R03APaginationV3Purpose
	authorizationPath := AuthorizationBindingPathRef(cfg.AuthorizationBindingPath, cfg.PathEncoding, futureAuthorizationOutput)
	pathDiagnostic, pathErr := ValidateExecutionPathRef(authorizationPath, checkPaths)
	report.PathDiagnostics = append(report.PathDiagnostics, pathDiagnostic)
	if pathErr != nil {
		report.Invalid = append(report.Invalid, "authorization_binding_path")
	}
	if cfg.PathEncoding == "windows-native" {
		report.PathEncoding = "windows-native_absolute"
	}
	if len(report.Missing) > 0 || len(report.Invalid) > 0 {
		report.Status = "FRONTEND_EXECUTION_CONFIG_INVALID"
	}
	report.EmployeeID, report.ProblemKey, report.Purpose, report.ExecutionFingerprint = cfg.EmployeeID, cfg.ProblemKey, cfg.Purpose, cfg.ExecutionFingerprint
	report.ToolCallLimit, report.MediumLimit, report.HighLimit, report.Concurrency = cfg.ToolCallLimit, cfg.MediumLimit, cfg.HighLimit, cfg.Concurrency
	report.TransportPolicy = cfg.TransportPolicy.Snapshot()
	return report
}

func (c FrontendExecutionConfig) R03A(initialOnly bool) R03AFrontendConfig {
	medium := c.MediumLimit
	if initialOnly {
		medium = 1
	}
	return R03AFrontendConfig{Config: Config{DSN: c.DSN, Binary: c.Binary, CodeModeHost: c.CodeModeHost, AuthFile: c.AuthFile, SelectedConfigPath: c.SelectedConfigPath, ExecutionConfigPath: c.ExecutionConfigPath, ExecutionManifestPath: c.ExecutionManifestPath, QualificationPath: c.QualificationPath, BlobDurabilityQualificationPath: c.BlobDurabilityQualificationPath, BehavioralContractQualificationPath: c.BehavioralContractPath, CheckerFeedbackQualificationPath: c.CheckerFeedbackQualificationPath, PostgresDumpPath: c.PostgresDumpPath, PostgresSnapshotDSN: c.PostgresSnapshotDSN, Root: c.RuntimeRoot, Evidence: c.EvidenceRoot, Model: c.Model, MediumLimit: medium, HighLimit: c.HighLimit, ToolCallLimit: c.ToolCallLimit, RuntimeCASBinding: c.RuntimeCASBinding, TransportPolicy: c.TransportPolicy}, ProblemKey: c.ProblemKey, RunPurpose: c.Purpose, AuthorizationBindingPath: c.AuthorizationBindingPath, CurrentL1EvidencePath: c.CurrentL1EvidencePath, CurrentL1Fingerprint: c.CurrentL1Fingerprint, ExecutionFingerprint: c.ExecutionFingerprint, HandoverBoundaryEvidencePath: c.HandoverBoundaryEvidencePath, HandoverBoundarySnapshotPath: c.HandoverBoundarySnapshotPath, InitialOnly: initialOnly}
}

func isWSLAbsolute(path string) bool {
	return strings.HasPrefix(path, "/mnt/") || strings.HasPrefix(path, "/home/") || strings.HasPrefix(path, "/run/") || strings.HasPrefix(path, "/tmp/")
}

func isWindowsAbsolute(path string) bool {
	if len(path) >= 3 && ((path[0] >= 'A' && path[0] <= 'Z') || (path[0] >= 'a' && path[0] <= 'z')) && path[1] == ':' && (path[2] == '\\' || path[2] == '/') {
		return true
	}
	return strings.HasPrefix(path, `\\`)
}

func WriteFrontendExecutionConfig(path string, cfg FrontendExecutionConfig) error {
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o600)
}
