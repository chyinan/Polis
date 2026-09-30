// pattern: Imperative Shell
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"polis/internal/codex"
	"polis/internal/kernel"
	"polis/internal/probe"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	preflight := flag.Bool("preflight", false, "run offline V3 freshness preflight")
	freshness := flag.Bool("continuation-freshness", false, "record V3 freshness without allowance or PG")
	backendPreflight := flag.Bool("backend-preflight", false, "run Backend-only offline V3 freshness preflight")
	backendFreshness := flag.Bool("backend-freshness", false, "record Backend-only V3 freshness without allowance or PG")
	backendRestoreVerify := flag.Bool("backend-restore-verify", false, "verify restored Backend continuity without provider or business writes")
	recoveryReadiness := flag.Bool("recovery-readiness", false, "run recovery client/readiness gate without source database access")
	backend := flag.Bool("backend", false, "run one real Backend Medium")
	frontend := flag.Bool("frontend", false, "run Frontend initial and successor Medium turns")
	frontendSinglePreflight := flag.Bool("frontend-single-preflight", false, "run the single-session Frontend offline preflight")
	frontendSingleFreshness := flag.Bool("frontend-single-freshness", false, "record the single-session Frontend freshness binding")
	frontendActivationPreflight := flag.Bool("frontend-activation-preflight", false, "run the read-only Frontend activation preflight")
	frontendRuntimePreflight := flag.Bool("frontend-runtime-preflight", false, "run the no-provider Frontend runtime initialize preflight")
	frontendDatabasePreflight := flag.Bool("frontend-database-preflight", false, "run the Windows-side database identity preflight")
	frontendSingle := flag.Bool("frontend-single", false, "run one real single-session Frontend Medium")
	flag.Parse()
	transportPolicy := codex.DefaultTransportPolicy()
	transportPolicy.Revision = os.Getenv("POLIS_V3_TRANSPORT_POLICY_REVISION")
	selected := 0
	for _, value := range []*bool{preflight, freshness, backendPreflight, backendFreshness, backendRestoreVerify, recoveryReadiness, backend, frontend, frontendSinglePreflight, frontendSingleFreshness, frontendActivationPreflight, frontendRuntimePreflight, frontendDatabasePreflight, frontendSingle} {
		if *value {
			selected++
		}
	}
	if selected != 1 {
		return fmt.Errorf("select exactly one V3 phase")
	}
	mediumLimit := 3
	if *backendPreflight || *backendFreshness || *backendRestoreVerify || *recoveryReadiness || *backend || *frontendSinglePreflight || *frontendSingleFreshness || *frontendActivationPreflight || *frontendRuntimePreflight || *frontendSingle {
		mediumLimit = 1
	}
	cfg := probe.R03APaginationV3Config{
		Config: probe.Config{
			DSN:                               os.Getenv("POLIS_DSN"),
			Binary:                            os.Getenv("POLIS_CODEX_BINARY"),
			CodeModeHost:                      os.Getenv("POLIS_CODEX_CODE_MODE_HOST"),
			AuthFile:                          os.Getenv("POLIS_CODEX_AUTH_FILE"),
			SelectedConfigPath:                os.Getenv("POLIS_SELECTED_CODEX_CONFIG"),
			ExecutionConfigPath:               os.Getenv("POLIS_V3_BACKEND_EXECUTION_MANIFEST"),
			Root:                              os.Getenv("POLIS_V3_RUNTIME_ROOT"),
			Evidence:                          os.Getenv("POLIS_V3_EVIDENCE"),
			ExpectedNativeVersion:             "0.154.0-alpha.6.2",
			Model:                             "gpt-5.6-luna",
			MediumLimit:                       mediumLimit,
			HighLimit:                         0,
			ToolCallLimit:                     48,
			TransportPolicy:                   transportPolicy,
			PostgresDumpPath:                  os.Getenv("POLIS_V3_POSTGRES_DUMP"),
			RecoveryRoot:                      os.Getenv("POLIS_RECOVERY_ROOT_WSL"),
			BlobDurabilityQualificationPath:   os.Getenv("POLIS_BLOB_DURABILITY_QUALIFICATION"),
			CheckerFeedbackQualificationPath:  os.Getenv("POLIS_FRONTEND_CHECKER_FEEDBACK_QUALIFICATION"),
			CurrentL1EvidencePath:             os.Getenv("POLIS_CURRENT_L1_EVIDENCE"),
			RuntimeDatabaseBindingPath:        os.Getenv("POLIS_RUNTIME_DATABASE_BINDING"),
			DatabaseAccessViewPath:            os.Getenv("POLIS_WINDOWS_DATABASE_ACCESS_VIEW"),
			RuntimeDatabaseBindingFingerprint: os.Getenv("POLIS_RUNTIME_DATABASE_BINDING_FINGERPRINT"),
			DatabaseAccessStrategyRevision:    os.Getenv("POLIS_DATABASE_ACCESS_STRATEGY_REVISION"),
			FrontendConsumptionQualificationPath: os.Getenv("POLIS_FRONTEND_CONSUMPTION_QUALIFICATION"),
			FrontendConsumptionContractRevision: os.Getenv("POLIS_FRONTEND_CONSUMPTION_CONTRACT_REVISION"),
			FrontendBindingContractRevision:      os.Getenv("POLIS_FRONTEND_BINDING_CONTRACT_REVISION"),
			FrontendBehaviorVerifierRevision:     os.Getenv("POLIS_FRONTEND_BEHAVIOR_VERIFIER_REVISION"),
		},
		ProblemKey:                     probe.R03AProblemKey,
		RunPurpose:                     probe.R03APaginationV3Purpose,
		AcceptanceQualificationPath:    os.Getenv("POLIS_V3_ACCEPTANCE_QUALIFICATION"),
		BackendExecutionManifestPath:   os.Getenv("POLIS_V3_BACKEND_EXECUTION_MANIFEST"),
		BackendQualificationPath:       os.Getenv("POLIS_V3_BACKEND_QUALIFICATION"),
		BackendL2LivePath:              os.Getenv("POLIS_V3_BACKEND_L2_LIVE"),
		FrontendExecutionManifestPath:  os.Getenv("POLIS_V3_FRONTEND_EXECUTION_MANIFEST"),
		FrontendQualificationPath:      os.Getenv("POLIS_V3_FRONTEND_QUALIFICATION"),
		FrontendL2LivePath:             os.Getenv("POLIS_V3_FRONTEND_L2_LIVE"),
		AuthorizationBindingPath:       os.Getenv("POLIS_V3_BACKEND_BINDING"),
		FrontendBindingPath:            os.Getenv("POLIS_V3_FRONTEND_BINDING"),
		AllowancePath:                  os.Getenv("POLIS_V3_ALLOWANCE"),
		BackendResultPath:              os.Getenv("POLIS_V3_BACKEND_RESULT"),
		FrontendResultPath:             os.Getenv("POLIS_V3_FRONTEND_RESULT"),
		PostgresSnapshotPath:           os.Getenv("POLIS_V3_POSTGRES_SNAPSHOT"),
		PostgresSnapshotDSN:            os.Getenv("POLIS_V3_POSTGRES_SNAPSHOT_DSN"),
		RestoreProofPath:               os.Getenv("POLIS_V3_RESTORE_PROOF"),
		FrontendBaselineManifestPath:   os.Getenv("POLIS_V3_FRONTEND_BASELINE_MANIFEST"),
		FrontendBaselineManifestSHA256: os.Getenv("POLIS_V3_FRONTEND_BASELINE_MANIFEST_SHA256"),
		FrontendCASManifestPath:        os.Getenv("POLIS_V3_FRONTEND_CAS_MANIFEST"),
		RuntimeCASBinding: kernel.RuntimeCASBinding{
			CanonicalRoot: os.Getenv("POLIS_V3_FRONTEND_CAS_ROOT"), LayoutRevision: os.Getenv("POLIS_V3_FRONTEND_CAS_LAYOUT_REVISION"),
			CompanyNamespace: os.Getenv("POLIS_V3_FRONTEND_CAS_COMPANY_NAMESPACE"), RequiredBlobInventoryDigest: os.Getenv("POLIS_V3_FRONTEND_CAS_INVENTORY_DIGEST"),
		},
		ExecutionEnvelopeFingerprint: os.Getenv("POLIS_V3_FRONTEND_EXECUTION_ENVELOPE_FINGERPRINT"),
	}
	if path := cfg.FrontendCASManifestPath; path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read Frontend CAS manifest: %w", err)
		}
		var manifest struct {
			Entries []kernel.CASRequiredBlob `json:"entries"`
		}
		if err := json.Unmarshal(raw, &manifest); err != nil {
			return fmt.Errorf("decode Frontend CAS manifest: %w", err)
		}
		cfg.FrontendRequiredCAS = manifest.Entries
		cfg.RuntimeCASBinding.RequiredBlobCount = len(manifest.Entries)
	}
	if *preflight {
		return probe.InspectR03APaginationV3(cfg)
	}
	if *freshness {
		return probe.RecordR03APaginationV3Freshness(cfg)
	}
	if *backendPreflight {
		return probe.InspectR03APaginationV3BackendOnly(cfg)
	}
	if *backendFreshness {
		return probe.RecordR03APaginationV3BackendOnlyFreshness(cfg)
	}
	if *backendRestoreVerify {
		return probe.VerifyR03APaginationV3BackendRestore(cfg)
	}
	if *recoveryReadiness {
		return probe.RecordR03ARecoveryReadiness(context.Background(), cfg)
	}
	if *backend {
		_, err := probe.RunR03APaginationV3Backend(cfg)
		return err
	}
	if *frontendSinglePreflight {
		return probe.InspectR03APaginationV3FrontendSingle(cfg)
	}
	if *frontendSingleFreshness {
		return probe.RecordR03APaginationV3FrontendSingleFreshness(cfg)
	}
	if *frontendActivationPreflight {
		report, err := probe.FrontendActivationPreflight(context.Background(), cfg.DSN, cfg.FrontendBaselineManifestPath, cfg.FrontendBaselineManifestSHA256, cfg.RuntimeCASBinding, cfg.FrontendRequiredCAS)
		output := os.Getenv("POLIS_V3_FRONTEND_ACTIVATION_PREFLIGHT_OUTPUT")
		if output == "" {
			output = "frontend-activation-preflight.json"
		}
		raw, marshalErr := json.MarshalIndent(report, "", "  ")
		if marshalErr != nil {
			return marshalErr
		}
		if writeErr := os.WriteFile(output, append(raw, '\n'), 0o600); writeErr != nil {
			return writeErr
		}
		return err
	}
	if *frontendRuntimePreflight {
		report, err := probe.FrontendRuntimeActivationPreflight(context.Background(), cfg)
		output := os.Getenv("POLIS_V3_FRONTEND_RUNTIME_PREFLIGHT_OUTPUT")
		if output == "" {
			output = "frontend-runtime-preflight.json"
		}
		raw, marshalErr := json.MarshalIndent(report, "", "  ")
		if marshalErr != nil {
			return marshalErr
		}
		if writeErr := os.WriteFile(output, append(raw, '\n'), 0o600); writeErr != nil {
			return writeErr
		}
		return err
	}
	if *frontendDatabasePreflight {
		binding, err := probe.LoadRuntimeDatabaseBinding(cfg.RuntimeDatabaseBindingPath)
		if err != nil {
			return fmt.Errorf("load runtime database binding: %w", err)
		}
		view, err := probe.LoadDatabaseAccessView(cfg.DatabaseAccessViewPath, binding)
		if err != nil {
			return fmt.Errorf("load database access view: %w", err)
		}
		if cfg.RuntimeDatabaseBindingFingerprint == "" || cfg.RuntimeDatabaseBindingFingerprint != binding.Fingerprint || cfg.DatabaseAccessStrategyRevision == "" || cfg.DatabaseAccessStrategyRevision != view.StrategyRevision {
			return fmt.Errorf("database access binding environment does not match qualified database identity")
		}
		report, err := probe.WindowsDatabaseAccessPreflight(context.Background(), cfg.DSN, binding, view)
		output := os.Getenv("POLIS_V3_FRONTEND_DATABASE_PREFLIGHT_OUTPUT")
		if output == "" {
			output = "frontend-database-preflight.json"
		}
		raw, marshalErr := json.MarshalIndent(report, "", "  ")
		if marshalErr != nil {
			return marshalErr
		}
		if writeErr := os.WriteFile(output, append(raw, '\n'), 0o600); writeErr != nil {
			return writeErr
		}
		return err
	}
	if *frontendSingle {
		_, err := probe.RunR03APaginationV3FrontendSingle(cfg)
		return err
	}
	_, err := probe.RunR03APaginationV3Frontend(cfg)
	return err
}
