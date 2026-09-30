// pattern: Imperative Shell
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"polis/internal/probe"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}

func run() error {
	preflight := flag.Bool("preflight", false, "run local T1/history preflight")
	flag.Parse()
	cwd, e := os.Getwd()
	if e != nil {
		return e
	}
	cfg := probe.R03AT2Config{Config: probe.Config{
		DSN:                   os.Getenv("POLIS_DSN"),
		AuthFile:              os.Getenv("POLIS_CODEX_AUTH_FILE"),
		Binary:                filepath.Join(cwd, ".tools/codex-linux/package/vendor/x86_64-unknown-linux-musl/bin/codex"),
		Root:                  filepath.Join(cwd, ".runtime/linux/r03a-t2"),
		Evidence:              filepath.Join(cwd, "evidence/development/r0.3a-t2/luna-1"),
		GoRoot:                filepath.Join(cwd, ".tools/go"),
		ProxyURL:              os.Getenv("POLIS_NATIVE_PROXY"),
		Model:                 "gpt-5.6-luna",
		MediumLimit:           1,
		HighLimit:             0,
		QualificationPath:     os.Getenv("POLIS_BUSINESS_QUALIFICATION_RECORD"),
		ExecutionManifestPath: os.Getenv("POLIS_CURRENT_EXECUTION_MANIFEST"),
	}, ProblemKey: probe.R03AT2ProblemKey, T1Evidence: filepath.Join(cwd, "evidence/development/r0.3a-t1"), OldEvidence: filepath.Join(cwd, "evidence/development/r0.3a-real/luna-1"), AuthorizationBindingPath: os.Getenv("POLIS_BUSINESS_AUTHORIZATION_BINDING"), ExecutionFingerprint: os.Getenv("POLIS_BUSINESS_EXECUTION_FINGERPRINT")}
	if *preflight {
		return probe.InspectR03AT2(cfg)
	}
	if cfg.AuthFile == "" {
		return fmt.Errorf("POLIS_CODEX_AUTH_FILE is required")
	}
	result, e := probe.RunR03AT2(cfg)
	fmt.Printf("r03a-t2=%s backend=%s contract=%s message=%s obligation=%s planner=%s candidate=%s transport=%s medium=%d high=%d\n", result.Status, result.BackendRealExecution, result.ContractRevisionV2, result.PeerMessage, result.Obligation, result.PlannerRelay, result.BackendCandidate, result.TransportState, result.MediumStarted, result.HighStarted)
	return e
}
