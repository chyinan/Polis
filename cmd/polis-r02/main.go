// pattern: Imperative Shell
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"polis/internal/core"
	"polis/internal/probe"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() error {
	inspect := flag.Bool("inspect", false, "native no-turn inspection")
	flag.Parse()
	cwd, e := os.Getwd()
	if e != nil {
		return e
	}
	cfg := probe.Config{DSN: os.Getenv("POLIS_DSN"), AuthFile: os.Getenv("POLIS_CODEX_AUTH_FILE"), Binary: filepath.Join(cwd, ".tools/codex-linux/package/vendor/x86_64-unknown-linux-musl/bin/codex"), Root: filepath.Join(cwd, ".runtime/linux/r02-luna"), Evidence: filepath.Join(cwd, "evidence/development/r0.2/luna-1"), GoRoot: filepath.Join(cwd, ".tools/go"), SchemaDigest: os.Getenv("POLIS_SCHEMA_DIGEST"), ProxyURL: os.Getenv("POLIS_NATIVE_PROXY"), Model: "gpt-5.6-luna", MediumLimit: 2, HighLimit: 1}
	if *inspect {
		return probe.Inspect(cfg)
	}
	if !core.ValidID("r02-luna-1") {
		return fmt.Errorf("invalid fixed experiment label")
	}
	if cfg.AuthFile == "" {
		return fmt.Errorf("POLIS_CODEX_AUTH_FILE is required for the authorized real probe")
	}
	result, e := probe.RunR02(cfg)
	fmt.Printf("r02=%s single=%s handover=%s behavior=%s medium=%d high=%d\n", result.Status, result.SingleWorker, result.Handover, result.Behavior, result.Usage.Medium, result.Usage.High)
	return e
}
