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
	real := flag.Bool("real", false, "consume the single explicitly authorized R0.1 allowance")
	inspect := flag.Bool("inspect", false, "native protocol only; no inference")
	label := flag.String("run-label", "real", "explicit experiment label; a new real label requires new owner authorization")
	medium := flag.Int("medium-turns", 3, "approved medium turn cap, 1..3")
	high := flag.Int("high-turns", 3, "approved high turn cap, 1..3")
	flag.Parse()
	if !core.ValidID(*label) || len(*label) > 40 || *medium < 1 || *medium > 3 || *high < 1 || *high > 3 {
		return fmt.Errorf("invalid experiment label or turn limits")
	}
	cwd, e := os.Getwd()
	if e != nil {
		return e
	}
	cfg := probe.Config{DSN: os.Getenv("POLIS_DSN"), AuthFile: os.Getenv("POLIS_CODEX_AUTH_FILE"), Binary: filepath.Join(cwd, ".tools/codex-linux/package/vendor/x86_64-unknown-linux-musl/bin/codex"), Root: filepath.Join(cwd, ".runtime/linux/r01-probe"), Evidence: filepath.Join(cwd, "evidence/development/r0.1/real"), GoRoot: filepath.Join(cwd, ".tools/go"), SchemaDigest: os.Getenv("POLIS_SCHEMA_DIGEST")}
	if *inspect {
		cfg.Evidence = filepath.Join(cwd, "evidence/development/r0.1", *label)
		cfg.ProxyURL = os.Getenv("POLIS_NATIVE_PROXY")
		return probe.Inspect(cfg)
	}
	if !*real {
		return fmt.Errorf("choose --inspect or the authorized --real probe")
	}
	if cfg.AuthFile == "" {
		return fmt.Errorf("POLIS_CODEX_AUTH_FILE: native authorized auth.json path required; no credential extraction or copying")
	}
	cfg.ProxyURL = os.Getenv("POLIS_NATIVE_PROXY")
	cfg.Evidence = filepath.Join(cwd, "evidence/development/r0.1", *label)
	if *label != "real" {
		cfg.Root = filepath.Join(cfg.Root, *label)
	}
	cfg.MediumLimit = *medium
	cfg.HighLimit = *high
	result, e := probe.Run(cfg)
	fmt.Printf("probe=%s single=%s handover=%s behavior=%s turns=%d\n", result.Status, result.Single, result.Handover, result.Behavior, result.Medium+result.High)
	return e
}
