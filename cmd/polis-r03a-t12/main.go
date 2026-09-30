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
	preflight := flag.Bool("preflight", false, "run T12 current-auth 0.151.0 baseline preflight")
	repair := flag.Bool("repair-derived-evidence", false, "recompute derived summaries from the sealed T12 protocol log")
	flag.Parse()
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cfg := probe.R03AT12Config{
		Config: probe.Config{
			Binary:      filepath.Join(cwd, ".tools/codex-linux/package/vendor/x86_64-unknown-linux-musl/bin/codex"),
			AuthFile:    os.Getenv("POLIS_CODEX_AUTH_FILE"),
			Root:        filepath.Join(cwd, ".runtime/linux/r03a-t12"),
			Evidence:    filepath.Join(cwd, "evidence/development/r0.3a-t12/luna-1"),
			GoRoot:      filepath.Join(cwd, ".tools/go"),
			ProxyURL:    "",
			Model:       "gpt-5.6-luna",
			MediumLimit: 1,
			HighLimit:   0,
		},
		Version:         "0.151.0",
		ProblemKey:      probe.R03AProblemKey,
		AuthSourceClass: "mounted_codex_auth_file",
		T9Evidence:      filepath.Join(cwd, "evidence/development/r0.3a-t9/luna-1"),
	}
	if *preflight {
		return probe.InspectR03AT12(cfg)
	}
	if *repair {
		return probe.RepairR03AT12Evidence(cfg)
	}
	result, err := probe.RunR03AT12(cfg)
	fmt.Printf("r03a-t12=%s case=%s transport=%s turn=%s sentinel=%s tools=%d medium=%d high=%d reconnects=%d first_output=%t first_output_ms=%d usage_updates=%d stop_confirmed=%t\n", result.Status, result.Case, result.Transport, result.TurnState, result.SentinelMatch, result.DynamicToolCount, result.MediumStarted, result.HighStarted, result.ReconnectCount, result.FirstValidOutput, result.TimeToFirstOutputMS, result.NativeUsageUpdates, result.StopConfirmed)
	return err
}
