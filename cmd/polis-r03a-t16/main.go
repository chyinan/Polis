// pattern: Imperative Shell
package main

import (
	"encoding/json"
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
	flag.Parse()
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	result, err := probe.RunR03AT16(probe.R03AT16Config{
		Binary:       filepath.Join(cwd, ".tools/codex-linux-0.153.4/package/vendor/x86_64-unknown-linux-musl/bin/codex"),
		AuthFile:     os.Getenv("POLIS_CODEX_AUTH_FILE"),
		Root:         filepath.Join(cwd, ".runtime/linux/r0-3a-t16"),
		Evidence:     filepath.Join(cwd, "evidence/development/r0.3a-t16"),
		T13Evidence:  filepath.Join(cwd, "evidence/development/r0.3a-t13/luna-1"),
		T14CEvidence: filepath.Join(cwd, "evidence/development/r0.3a-t14c/luna-1"),
	})
	if result != nil {
		raw, marshalErr := json.Marshal(result)
		if marshalErr == nil {
			fmt.Println(string(raw))
		}
	}
	return err
}
