// pattern: Imperative Shell

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"polis/internal/probe"
	"polis/internal/runner"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	preflight := flag.Bool("preflight", false, "run the offline real-Backend preflight")
	freshness := flag.Bool("continuation-freshness", false, "record a continuation freshness revalidation without creating allowance")
	flag.Parse()
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	binary := requiredEnv("POLIS_CODEX_BINARY", `C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\codex.exe`)
	helper := requiredEnv("POLIS_CODEX_CODE_MODE_HOST", `C:\Users\chyinan\AppData\Local\OpenAI\Codex\controlled-runtime\polis-r03a\bffc5354119c8421\codex-code-mode-host.exe`)
	if manifestPath := os.Getenv("POLIS_WINDOWS_RUNTIME_MANIFEST"); manifestPath != "" {
		artifact, loadErr := runner.LoadAndVerifyWindowsRuntimeArtifact(manifestPath)
		if loadErr != nil {
			return fmt.Errorf("controlled Windows runtime manifest rejected: %w", loadErr)
		}
		binary, helper = artifact.CodexBinaryStagedPath, artifact.CodeModeHostStagedPath
	}
	cfg := probe.R03AT2Config{
		Config: probe.Config{
			DSN:                                 os.Getenv("POLIS_DSN"),
			Binary:                              binary,
			CodeModeHost:                        helper,
			AuthFile:                            os.Getenv("POLIS_CODEX_AUTH_FILE"),
			SelectedConfigPath:                  os.Getenv("POLIS_SELECTED_CODEX_CONFIG"),
			ExecutionConfigPath:                 os.Getenv("POLIS_CURRENT_EXECUTION_CONFIG"),
			BlobDurabilityQualificationPath:     os.Getenv("POLIS_BLOB_DURABILITY_QUALIFICATION"),
			BehavioralContractQualificationPath: os.Getenv("POLIS_BEHAVIORAL_CONTRACT_QUALIFICATION"),
			Root:                                requiredEnv("POLIS_BACKEND_RUNTIME_ROOT", filepath.Join(cwd, ".runtime", "windows", "r0.3a-real-backend")),
			Evidence:                            requiredEnv("POLIS_BACKEND_EVIDENCE", filepath.Join(cwd, "evidence", "development", "r0.3a-real-backend-employee", "luna-1")),
			ExpectedNativeVersion:               "0.154.0-alpha.6.2",
			Model:                               "gpt-5.6-luna",
			MediumLimit:                         1,
			HighLimit:                           0,
			ToolCallLimit:                       envInt("POLIS_BUSINESS_TOOL_CALL_LIMIT"),
			QualificationPath:                   os.Getenv("POLIS_BUSINESS_QUALIFICATION_RECORD"),
			ExecutionManifestPath:               os.Getenv("POLIS_CURRENT_EXECUTION_MANIFEST"),
			CurrentL1EvidencePath:               os.Getenv("POLIS_CURRENT_L1_EVIDENCE"),
		},
		ProblemKey:               probe.R03AT2ProblemKey,
		T1Evidence:               filepath.Join(cwd, "evidence", "development", "r0.3a-t1"),
		OldEvidence:              filepath.Join(cwd, "evidence", "development", "r0.3a-real", "luna-1"),
		RunPurpose:               "real_backend_peer_collaboration",
		AuthorizationBindingPath: os.Getenv("POLIS_BUSINESS_AUTHORIZATION_BINDING"),
		ExecutionFingerprint:     os.Getenv("POLIS_BUSINESS_EXECUTION_FINGERPRINT"),
	}
	if *preflight {
		return probe.InspectR03AT2(cfg)
	}
	if *freshness {
		_, err := probe.RecordR03AT2ContinuationFreshness(cfg, os.Getenv("POLIS_BACKEND_ORCHESTRATION_REVISION"))
		return err
	}
	for name, value := range map[string]string{
		"POLIS_DSN":                            cfg.DSN,
		"POLIS_CODEX_AUTH_FILE":                cfg.AuthFile,
		"POLIS_SELECTED_CODEX_CONFIG":          cfg.SelectedConfigPath,
		"POLIS_CURRENT_EXECUTION_CONFIG":       cfg.ExecutionConfigPath,
		"POLIS_CURRENT_EXECUTION_MANIFEST":     cfg.ExecutionManifestPath,
		"POLIS_BLOB_DURABILITY_QUALIFICATION":  cfg.BlobDurabilityQualificationPath,
		"POLIS_BUSINESS_QUALIFICATION_RECORD":  cfg.QualificationPath,
		"POLIS_BUSINESS_AUTHORIZATION_BINDING": cfg.AuthorizationBindingPath,
		"POLIS_BUSINESS_EXECUTION_FINGERPRINT": cfg.ExecutionFingerprint,
		"POLIS_BUSINESS_TOOL_CALL_LIMIT":       fmt.Sprint(cfg.ToolCallLimit),
	} {
		if value == "" {
			return fmt.Errorf("%s is required", name)
		}
	}
	result, err := probe.RunR03AT2(cfg)
	fmt.Printf("r0.3a-real-backend=%s backend=%s fingerprint=%s contract=%s message=%s obligation=%s planner=%s candidate=%s transport=%s medium=%d high=%d\n", result.Status, result.BackendRealExecution, result.ExecutionFingerprint, result.ContractRevisionV2, result.PeerMessage, result.Obligation, result.PlannerRelay, result.BackendCandidate, result.TransportState, result.MediumStarted, result.HighStarted)
	return err
}

func requiredEnv(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func envInt(name string) int {
	value := os.Getenv(name)
	var parsed int
	if value != "" {
		_, _ = fmt.Sscanf(value, "%d", &parsed)
	}
	return parsed
}
