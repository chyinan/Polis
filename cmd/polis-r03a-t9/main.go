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
	preflight := flag.Bool("preflight", false, "run no-proxy differential preflight")
	flag.Parse()
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	base := probe.R03AT6Config{Config: probe.Config{Binary: filepath.Join(cwd, ".tools/codex-linux/package/vendor/x86_64-unknown-linux-musl/bin/codex"), AuthFile: os.Getenv("POLIS_CODEX_AUTH_FILE"), Root: filepath.Join(cwd, ".runtime/linux/r03a-t9"), Evidence: filepath.Join(cwd, "evidence/development/r0.3a-t9/luna-1"), GoRoot: filepath.Join(cwd, ".tools/go"), ProxyURL: "", Model: "gpt-5.6-luna", MediumLimit: 1, HighLimit: 0}, ProblemKey: probe.R03AProblemKey, T5Evidence: filepath.Join(cwd, "evidence/development/r0.3a-t5"), OldEvidence: filepath.Join(cwd, "evidence/development/r0.3a-t2/luna-1")}
	cfg := probe.R03AT9Config{R03AT6Config: base, BaselineT6: filepath.Join(cwd, "evidence/development/r0.3a-t6/luna-1"), T7State: filepath.Join(cwd, "evidence/development/r0.3a-t7/qualification-state.json"), T8Manifest: filepath.Join(cwd, "evidence/development/r0.3a-t8/execution-diff.json")}
	if *preflight {
		return probe.InspectR03AT9(cfg)
	}
	if base.AuthFile == "" {
		return fmt.Errorf("POLIS_CODEX_AUTH_FILE is required")
	}
	result, err := probe.RunR03AT6(base)
	fmt.Printf("r03a-t9=%s transport=%s turn=%s sentinel=%s tools=%d medium=%d high=%d reconnects=%d\n", result.Status, result.Transport, result.TurnState, result.SentinelMatch, result.DynamicToolCount, result.MediumStarted, result.HighStarted, result.ReconnectCount)
	return err
}
