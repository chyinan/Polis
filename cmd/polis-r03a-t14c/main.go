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
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	preflight := flag.Bool("preflight", false, "run T14C corrected WebSocket-policy-only preflight")
	flag.Parse()
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cfg := probe.R03AT14CConfig{
		Config: probe.Config{
			Binary:      filepath.Join(cwd, ".tools/codex-linux-0.153.4/package/vendor/x86_64-unknown-linux-musl/bin/codex"),
			AuthFile:    os.Getenv("POLIS_CODEX_AUTH_FILE"),
			Root:        filepath.Join(cwd, ".runtime/linux/r03a-t14c"),
			Evidence:    filepath.Join(cwd, "evidence/development/r0.3a-t14c/luna-1"),
			GoRoot:      filepath.Join(cwd, ".tools/go"),
			ProxyURL:    "",
			Model:       "gpt-5.6-luna",
			MediumLimit: 1,
			HighLimit:   0,
		},
		Version:         "0.153.4",
		ProblemKey:      probe.R03AProblemKey,
		AuthSourceClass: "mounted_codex_auth_file",
		T13Evidence:     filepath.Join(cwd, "evidence/development/r0.3a-t13/luna-1"),
		T14AEvidence:    filepath.Join(cwd, "evidence/development/r0.3a-t14a"),
		T10Evidence:     filepath.Join(cwd, "evidence/development/r0.3a-t10"),
	}
	if *preflight {
		return probe.InspectR03AT14C(cfg)
	}
	result, err := probe.RunR03AT14C(cfg)
	fmt.Printf("r03a-t14c=%s case=%s transport=%s configured=%s actual=%s turn=%s sentinel=%s tools=%d medium=%d high=%d egress=%d reconnects=%d first_output=%t first_output_ms=%d usage_updates=%d stop_confirmed=%t\n", result.Status, result.Case, result.Transport, result.ConfiguredTransport, result.ActualTransport, result.TurnState, result.SentinelMatch, result.DynamicToolCount, result.MediumStarted, result.HighStarted, result.ProviderEgress, result.ReconnectCount, result.FirstValidOutput, result.TimeToFirstOutputMS, result.NativeUsageUpdates, result.StopConfirmed)
	return err
}
