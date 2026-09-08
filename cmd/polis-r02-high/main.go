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
	old := flag.String("old-evidence", "", "existing R0.2 evidence directory with 2 Medium passes")
	flag.Parse()
	if *old == "" {
		return fmt.Errorf("old R0.2 evidence directory is required")
	}
	cwd, e := os.Getwd()
	if e != nil {
		return e
	}
	cfg := probe.Config{DSN: os.Getenv("POLIS_DSN"), AuthFile: os.Getenv("POLIS_CODEX_AUTH_FILE"), Binary: filepath.Join(cwd, ".tools/codex-linux/package/vendor/x86_64-unknown-linux-musl/bin/codex"), Root: filepath.Join(cwd, ".runtime/linux/r02-high-recovery"), Evidence: filepath.Join(cwd, "evidence/development/r0.2/luna-1-high-recovery"), GoRoot: filepath.Join(cwd, ".tools/go"), SchemaDigest: os.Getenv("POLIS_SCHEMA_DIGEST"), ProxyURL: os.Getenv("POLIS_NATIVE_PROXY"), Model: "gpt-5.6-luna", MediumLimit: 2, HighLimit: 1}
	if cfg.AuthFile == "" {
		return fmt.Errorf("POLIS_CODEX_AUTH_FILE is required")
	}
	result, e := probe.RunR02HighRecovery(cfg, *old)
	fmt.Printf("r02-high=%s handover=%s behavior=%s high=%d\n", result.Status, result.Handover, result.Behavior, result.Usage.High)
	return e
}
