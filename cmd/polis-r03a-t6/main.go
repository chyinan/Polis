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
	preflight := flag.Bool("preflight", false, "run local T5/history preflight")
	flag.Parse()
	cwd, e := os.Getwd()
	if e != nil {
		return e
	}
	cfg := probe.R03AT6Config{Config: probe.Config{
		DSN: os.Getenv("POLIS_DSN"),
		AuthFile: os.Getenv("POLIS_CODEX_AUTH_FILE"),
		Binary: filepath.Join(cwd, ".tools/codex-linux/package/vendor/x86_64-unknown-linux-musl/bin/codex"),
		Root: filepath.Join(cwd, ".runtime/linux/r03a-t6"),
		Evidence: filepath.Join(cwd, "evidence/development/r0.3a-t6/luna-1"),
		GoRoot: filepath.Join(cwd, ".tools/go"),
		ProxyURL: os.Getenv("POLIS_NATIVE_PROXY"),
		Model: "gpt-5.6-luna",
		MediumLimit: 1,
		HighLimit: 0,
	}, ProblemKey: probe.R03AProblemKey, T5Evidence: filepath.Join(cwd, "evidence/development/r0.3a-t5"), OldEvidence: filepath.Join(cwd, "evidence/development/r0.3a-t2/luna-1")}
	if *preflight {
		return probe.InspectR03AT6(cfg)
	}
	if cfg.AuthFile == "" {
		return fmt.Errorf("POLIS_CODEX_AUTH_FILE is required")
	}
	result, e := probe.RunR03AT6(cfg)
	fmt.Printf("r03a-t6=%s transport=%s turn=%s sentinel=%s tools=%d medium=%d high=%d reconnects=%d\n", result.Status, result.Transport, result.TurnState, result.SentinelMatch, result.DynamicToolCount, result.MediumStarted, result.HighStarted, result.ReconnectCount)
	return e
}
