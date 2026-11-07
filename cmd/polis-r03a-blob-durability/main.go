// pattern: Imperative Shell

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"polis/internal/kernel"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	root := flag.String("root", ".runtime/blob-durability-smoke", "dedicated smoke root")
	evidence := flag.String("evidence", "evidence/development/r0.3a-blob-durability/smoke.json", "smoke evidence path")
	flag.Parse()
	if _, err := os.Stat(*evidence); err == nil {
		return fmt.Errorf("blob durability smoke evidence already exists; refusing overwrite")
	}
	report, err := kernel.RunBlobDurabilitySmoke(*root)
	if err != nil {
		report.Error = err.Error()
	}
	raw, marshalErr := json.MarshalIndent(report, "", "  ")
	if marshalErr != nil {
		return marshalErr
	}
	if mkdirErr := os.MkdirAll(filepath.Dir(*evidence), 0700); mkdirErr != nil {
		return mkdirErr
	}
	if writeErr := os.WriteFile(*evidence, append(raw, '\n'), 0600); writeErr != nil {
		return writeErr
	}
	fmt.Println(string(raw))
	return err
}
