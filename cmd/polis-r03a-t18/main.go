// pattern: Imperative Shell
package main

import (
	"encoding/json"
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
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	result, err := probe.RunR03AT18(probe.R03AT18Config{
		T17Evidence:    filepath.Join(cwd, "evidence/development/r0.3a-t17"),
		T14CEvidence:   filepath.Join(cwd, "evidence/development/r0.3a-t14c/luna-1"),
		T16Evidence:    filepath.Join(cwd, "evidence/development/r0.3a-t16"),
		AuthFile:       "/mnt/c/Users/chyinan/.codex/auth.json",
		PolisHostHome:  filepath.Join(cwd, ".runtime/linux/r03a-t14c/home"),
		PolisGuestHome: "/home/codex",
		Evidence:       filepath.Join(cwd, "evidence/development/r0.3a-t18"),
	})
	if result != nil {
		raw, marshalErr := json.Marshal(result)
		if marshalErr == nil {
			fmt.Println(string(raw))
		}
	}
	return err
}
