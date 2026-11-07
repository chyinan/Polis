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
	preflight := flag.Bool("preflight", false, "run no-inference native and local preflight")
	flag.Parse()
	cwd, e := os.Getwd()
	if e != nil {
		return e
	}
	cfg := probe.R03AConfig{Config: probe.Config{
		DSN:          os.Getenv("POLIS_DSN"),
		AuthFile:     os.Getenv("POLIS_CODEX_AUTH_FILE"),
		Binary:       filepath.Join(cwd, ".tools/codex-linux/package/vendor/x86_64-unknown-linux-musl/bin/codex"),
		Root:         filepath.Join(cwd, ".runtime/linux/r03a-real"),
		Evidence:     filepath.Join(cwd, "evidence/development/r0.3a-real/luna-1"),
		GoRoot:       filepath.Join(cwd, ".tools/go"),
		ProxyURL:     os.Getenv("POLIS_NATIVE_PROXY"),
		Model:        "gpt-5.6-luna",
		MediumLimit:  3,
		HighLimit:    1,
		QualificationPath: os.Getenv("POLIS_BUSINESS_QUALIFICATION_RECORD"),
		ExecutionManifestPath: os.Getenv("POLIS_CURRENT_EXECUTION_MANIFEST"),
	}, ProblemKey: probe.R03AProblemKey}
	if cfg.AuthFile == "" {
		return fmt.Errorf("POLIS_CODEX_AUTH_FILE is required")
	}
	if *preflight {
		return probe.InspectR03A(cfg)
	}
	result, e := probe.RunR03A(cfg)
	fmt.Printf("r03a=%s backend=%s message=%s frontend=%s handover=%s successor=%s verifier=%s negative=%s reviewer=%s hidden=%s isolation=%s medium=%d high=%d\n", result.Status, result.BackendRealExecution, result.PeerMessageDelivery, result.FrontendObservedV2, result.FrontendHandover, result.SuccessorBehavior, result.IntegrationVerifier, result.NegativeControl, result.ReviewerVerdict, result.HiddenVerifier, result.ReviewIsolation, result.MediumStarted, result.HighStarted)
	return e
}
