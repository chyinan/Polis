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
	oldEvidence := flag.String("old-evidence", "evidence/development/r0.2/luna-1", "immutable R0.2 evidence directory")
	allowance := flag.String("allowance", "", "new independent High-only allowance path")
	flag.Parse()
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	evidence := os.Getenv("POLIS_R02H2_EVIDENCE")
	if evidence == "" {
		evidence = filepath.Join(cwd, "evidence/development/r0.2h2")
	}
	cfg := probe.Config{DSN: os.Getenv("POLIS_DSN"), AuthFile: os.Getenv("POLIS_CODEX_AUTH_FILE"), Binary: filepath.Join(cwd, ".tools/codex-linux/package/vendor/x86_64-unknown-linux-musl/bin/codex"), Root: filepath.Join(cwd, ".runtime/linux/r02h2"), Evidence: evidence, GoRoot: filepath.Join(cwd, ".tools/go"), SchemaDigest: os.Getenv("POLIS_SCHEMA_DIGEST"), ProxyURL: os.Getenv("POLIS_NATIVE_PROXY"), Model: "gpt-5.6-luna", HighLimit: 1}
	result, err := probe.RunR02H2(cfg, *oldEvidence, *allowance)
	fmt.Printf("r02h2=%s preflight=%s reviewer=%s hidden=%s isolation=%s high=%d\n", result.Status, result.Preflight, result.ReviewerVerdict, result.HiddenVerifier, result.ReviewIsolation, result.Usage.High)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
