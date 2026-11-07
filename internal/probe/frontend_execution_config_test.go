// pattern: Functional Core
package probe

import (
	"encoding/json"
	"polis/internal/codex"
	"polis/internal/kernel"
	"strings"
	"testing"
)

func validFrontendExecutionConfigForTest() FrontendExecutionConfig {
	return FrontendExecutionConfig{
		PathEncoding: "wsl", ExecutionConfigPath: "/mnt/d/Polis/frontend-config.json", RecoveryPackageRoot: "/mnt/d/Polis/recovery", SourceCASRoot: "/mnt/d/Polis/cas", RuntimeArtifactManifestPath: "/mnt/c/Codex/runtime.json", CurrentL1Fingerprint: "l1-fingerprint",
		DSN: "host=/tmp/polis-test dbname=restore user=polis_runtime", Binary: "/mnt/c/Codex/codex.exe", CodeModeHost: "/mnt/c/Codex/helper.exe", AuthFile: "/mnt/c/Codex/auth.json", SelectedConfigPath: "/mnt/d/Polis/config.toml", ExecutionManifestPath: "/mnt/d/Polis/execution.json", QualificationPath: "/mnt/d/Polis/qualification.json", BlobDurabilityQualificationPath: "/mnt/d/Polis/blob.json", BehavioralContractPath: "/mnt/d/Polis/behavior.json", CheckerFeedbackQualificationPath: "/mnt/d/Polis/checker.json", PostgresDumpPath: "/mnt/d/Polis/pg_dump", PostgresSnapshotDSN: "host=/tmp/polis-test dbname=restore user=chyinan", RuntimeRoot: "/home/chyinan/.local/state/polis-restore-v9", EvidenceRoot: "/mnt/d/Polis/evidence", RuntimeDatabaseBindingPath: "/mnt/d/Polis/runtime-database-binding.json", DatabaseAccessViewPath: "/mnt/d/Polis/database-access-view.json", RuntimeDatabaseBindingFingerprint: "database-binding-fingerprint", DatabaseAccessStrategyRevision: "r03a-windows-localhost-forwarding@1", AuthorizationBindingPath: "/mnt/d/Polis/frontend-binding.json", CurrentL1EvidencePath: "/mnt/d/Polis/l1", ExecutionFingerprint: "frontend-fingerprint", HandoverBoundaryEvidencePath: "/mnt/d/Polis/handover", HandoverBoundarySnapshotPath: "/home/chyinan/.local/state/polis-restore-v9/handover.dump", ProblemKey: R03AProblemKey, Purpose: r03aFrontendHandoverPurpose, EmployeeID: "emp-frontend", Model: "gpt-5.6-luna", Effort: "medium", ToolCallLimit: 48, MediumLimit: 1, HighLimit: 0, Concurrency: 1,
	}
}

func TestValidateFrontendExecutionConfigRequiresExplicitTypedBoundary(t *testing.T) {
	report := ValidateFrontendExecutionConfig(validFrontendExecutionConfigForTest(), false)
	if report.Status != "FRONTEND_EXECUTION_CONFIG_READY" || len(report.Missing) != 0 || len(report.Invalid) != 0 {
		t.Fatalf("report=%+v", report)
	}
	cfg := validFrontendExecutionConfigForTest()
	cfg.ProblemKey = "typo"
	if report := ValidateFrontendExecutionConfig(cfg, false); report.Status != "FRONTEND_EXECUTION_CONFIG_INVALID" {
		t.Fatalf("invalid problem key accepted: %+v", report)
	}
	cfg = validFrontendExecutionConfigForTest()
	cfg.DSN = ""
	if report := ValidateFrontendExecutionConfig(cfg, false); report.Status != "FRONTEND_EXECUTION_CONFIG_INVALID" {
		t.Fatalf("missing DSN accepted: %+v", report)
	}
	cfg = validFrontendExecutionConfigForTest()
	cfg.RuntimeRoot = `D:\wrong\windows\path`
	if report := ValidateFrontendExecutionConfig(cfg, false); report.Status != "FRONTEND_EXECUTION_CONFIG_INVALID" {
		t.Fatalf("Windows path accepted by WSL boundary: %+v", report)
	}
}

func TestValidateFrontendExecutionConfigRequiresExplicitPathEncoding(t *testing.T) {
	cfg := validFrontendExecutionConfigForTest()
	cfg.PathEncoding = ""
	if report := ValidateFrontendExecutionConfig(cfg, false); report.Status != "FRONTEND_EXECUTION_CONFIG_INVALID" {
		t.Fatalf("missing path encoding accepted: %+v", report)
	}
}

func TestValidateFrontendExecutionConfigRequiresTypedCASBindingForPaginationV3(t *testing.T) {
	cfg := validFrontendExecutionConfigForTest()
	cfg.Purpose = R03APaginationV3Purpose
	cfg.TransportPolicy = codex.DefaultTransportPolicy()
	cfg.RuntimeCASBinding = kernel.RuntimeCASBinding{
		CanonicalRoot:               "/home/test/cas",
		LayoutRevision:              kernel.CASLayoutRevision,
		CompanyNamespace:            "company-1",
		RequiredBlobInventoryDigest: strings.Repeat("a", 64),
		RequiredBlobCount:           1,
	}
	if report := ValidateFrontendExecutionConfig(cfg, false); report.Status != "FRONTEND_EXECUTION_CONFIG_READY" {
		t.Fatalf("typed pagination CAS binding rejected: %+v", report)
	}
	cfg.RuntimeCASBinding = kernel.RuntimeCASBinding{}
	if report := ValidateFrontendExecutionConfig(cfg, false); report.Status != "FRONTEND_EXECUTION_CONFIG_INVALID" || !containsConfigString(report.Invalid, "runtime_cas_binding") {
		t.Fatalf("missing pagination CAS binding accepted: %+v", report)
	}
}

func containsConfigString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestFrontendExecutionConfigProbeDoesNotExposeSecrets(t *testing.T) {
	cfg := validFrontendExecutionConfigForTest()
	report := ValidateFrontendExecutionConfig(cfg, false)
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), cfg.DSN) || strings.Contains(string(raw), cfg.PostgresSnapshotDSN) {
		t.Fatal("raw DSN leaked into probe report")
	}
	if !report.SecretSourcesPresent["dsn"] || !report.SecretSourcesPresent["auth_file"] {
		t.Fatalf("secret source presence not recorded: %+v", report.SecretSourcesPresent)
	}
}

func TestValidateFrontendExecutionConfigAcceptsWindowsPathsWithSpaces(t *testing.T) {
	cfg := validFrontendExecutionConfigForTest()
	cfg.PathEncoding = "windows-native"
	for name, value := range map[string]string{
		"Binary":                           `C:\Codex Runtime\codex.exe`,
		"CodeModeHost":                     `C:\Codex Runtime\codex-code-mode-host.exe`,
		"AuthFile":                         `C:\Users\tester\.codex\auth.json`,
		"SelectedConfigPath":               `D:\Programs\Polis\config.toml`,
		"ExecutionConfigPath":              `D:\Programs\Polis\frontend config.json`,
		"ExecutionManifestPath":            `D:\Programs\Polis\execution.json`,
		"QualificationPath":                `D:\Programs\Polis\qualification.json`,
		"BlobDurabilityQualificationPath":  `D:\Programs\Polis\blob.json`,
		"BehavioralContractPath":           `D:\Programs\Polis\behavior.json`,
		"CheckerFeedbackQualificationPath": `D:\Programs\Polis\checker.json`,
		"PostgresDumpPath":                 `D:\PostgreSQL 18\bin\pg_dump.exe`,
		"RuntimeRoot":                      `D:\Polis recovery\runtime`,
		"EvidenceRoot":                     `D:\Programs\Polis\evidence`,
		"RecoveryPackageRoot":              `D:\Polis recovery\package`,
		"SourceCASRoot":                    `D:\Polis recovery\cas`,
		"RuntimeArtifactManifestPath":      `C:\Codex Runtime\runtime-manifest.json`,
		"AuthorizationBindingPath":         `D:\Programs\Polis\binding.json`,
		"CurrentL1EvidencePath":            `D:\Programs\Polis\l1`,
		"HandoverBoundaryEvidencePath":     `D:\Programs\Polis\handover`,
		"HandoverBoundarySnapshotPath":     `D:\Polis recovery\handover.dump`,
	} {
		switch name {
		case "Binary":
			cfg.Binary = value
		case "CodeModeHost":
			cfg.CodeModeHost = value
		case "AuthFile":
			cfg.AuthFile = value
		case "SelectedConfigPath":
			cfg.SelectedConfigPath = value
		case "ExecutionConfigPath":
			cfg.ExecutionConfigPath = value
		case "ExecutionManifestPath":
			cfg.ExecutionManifestPath = value
		case "QualificationPath":
			cfg.QualificationPath = value
		case "BlobDurabilityQualificationPath":
			cfg.BlobDurabilityQualificationPath = value
		case "BehavioralContractPath":
			cfg.BehavioralContractPath = value
		case "CheckerFeedbackQualificationPath":
			cfg.CheckerFeedbackQualificationPath = value
		case "PostgresDumpPath":
			cfg.PostgresDumpPath = value
		case "RuntimeRoot":
			cfg.RuntimeRoot = value
		case "EvidenceRoot":
			cfg.EvidenceRoot = value
		case "RecoveryPackageRoot":
			cfg.RecoveryPackageRoot = value
		case "SourceCASRoot":
			cfg.SourceCASRoot = value
		case "RuntimeArtifactManifestPath":
			cfg.RuntimeArtifactManifestPath = value
		case "AuthorizationBindingPath":
			cfg.AuthorizationBindingPath = value
		case "CurrentL1EvidencePath":
			cfg.CurrentL1EvidencePath = value
		case "HandoverBoundaryEvidencePath":
			cfg.HandoverBoundaryEvidencePath = value
		case "HandoverBoundarySnapshotPath":
			cfg.HandoverBoundarySnapshotPath = value
		}
	}
	if report := ValidateFrontendExecutionConfig(cfg, false); report.Status != "FRONTEND_EXECUTION_CONFIG_READY" {
		t.Fatalf("Windows path boundary rejected valid spaces: %+v", report)
	}
}
