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
	preflight := flag.Bool("preflight", false, "run T13 current-auth version-only preflight")
	repair := flag.Bool("repair-derived-evidence", false, "recompute derived summaries from the sealed T13 protocol log")
	flag.Parse()
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cfg := probe.R03AT13Config{
		Config: probe.Config{
			Binary:      filepath.Join(cwd, ".tools/codex-linux-0.153.4/package/vendor/x86_64-unknown-linux-musl/bin/codex"),
			AuthFile:    os.Getenv("POLIS_CODEX_AUTH_FILE"),
			Root:        filepath.Join(cwd, ".runtime/linux/r03a-t13"),
			Evidence:    filepath.Join(cwd, "evidence/development/r0.3a-t13/luna-1"),
			GoRoot:      filepath.Join(cwd, ".tools/go"),
			ProxyURL:    "",
			Model:       "gpt-5.6-luna",
			MediumLimit: 1,
			HighLimit:   0,
		},
		Version:         "0.153.4",
		ProblemKey:      probe.R03AProblemKey,
		AuthSourceClass: "mounted_codex_auth_file",
		BaselineBinary:  filepath.Join(cwd, ".tools/codex-linux/package/vendor/x86_64-unknown-linux-musl/bin/codex"),
		T12Evidence:     filepath.Join(cwd, "evidence/development/r0.3a-t12/luna-1"),
		T10Evidence:     filepath.Join(cwd, "evidence/development/r0.3a-t10"),
	}
	if *preflight {
		return probe.InspectR03AT13(cfg)
	}
	if *repair {
		return probe.RepairR03AT13Evidence(cfg)
	}
	result, err := probe.RunR03AT13(cfg)
	fmt.Printf("r03a-t13=%s case=%s transport=%s turn=%s sentinel=%s tools=%d medium=%d high=%d reconnects=%d first_output=%t first_output_ms=%d usage_updates=%d stop_confirmed=%t\n", result.Status, result.Case, result.Transport, result.TurnState, result.SentinelMatch, result.DynamicToolCount, result.MediumStarted, result.HighStarted, result.ReconnectCount, result.FirstValidOutput, result.TimeToFirstOutputMS, result.NativeUsageUpdates, result.StopConfirmed)
	return err
}
