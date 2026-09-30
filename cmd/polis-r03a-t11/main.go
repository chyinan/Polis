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
	preflight := flag.Bool("preflight", false, "run T11 version-only single-factor preflight")
	flag.Parse()
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cfg := probe.R03AT11Config{
		Config: probe.Config{
			Binary:      filepath.Join(cwd, ".tools/codex-linux-0.153.4/package/vendor/x86_64-unknown-linux-musl/bin/codex"),
			AuthFile:    os.Getenv("POLIS_CODEX_AUTH_FILE"),
			Root:        filepath.Join(cwd, ".runtime/linux/r03a-t11"),
			Evidence:    filepath.Join(cwd, "evidence/development/r0.3a-t11/luna-1"),
			GoRoot:      filepath.Join(cwd, ".tools/go"),
			ProxyURL:    "",
			Model:       "gpt-5.6-luna",
			MediumLimit: 1,
			HighLimit:   0,
		},
		Version:          "0.153.4",
		HistoricalBinary: filepath.Join(cwd, ".tools/codex-linux/package/vendor/x86_64-unknown-linux-musl/bin/codex"),
		ProblemKey:       probe.R03AProblemKey,
		T9Evidence:       filepath.Join(cwd, "evidence/development/r0.3a-t9/luna-1"),
		T10Evidence:      filepath.Join(cwd, "evidence/development/r0.3a-t10"),
	}
	if *preflight {
		return probe.InspectR03AT11(cfg)
	}
	result, err := probe.RunR03AT11(cfg)
	fmt.Printf("r03a-t11=%s transport=%s turn=%s sentinel=%s tools=%d medium=%d high=%d reconnects=%d first_output=%t first_output_ms=%d stop_confirmed=%t\n", result.Status, result.Transport, result.TurnState, result.SentinelMatch, result.DynamicToolCount, result.MediumStarted, result.HighStarted, result.ReconnectCount, result.FirstValidOutput, result.FirstValidOutputDeltaMS, result.StopConfirmed)
	return err
}
