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
	result, err := probe.RunR03AT19(probe.R03AT19Config{
		WindowsBinary:       "/mnt/c/Users/chyinan/AppData/Local/OpenAI/Codex/bin/fd4c151a749f3ab4/codex.exe",
		WindowsCodeModeHost: "/mnt/c/Users/chyinan/AppData/Local/OpenAI/Codex/bin/fd4c151a749f3ab4/codex-code-mode-host.exe",
		WindowsRunEvidence:  filepath.Join(cwd, "evidence/development/r0.3a-t19"),
		T14CEvidence:        filepath.Join(cwd, "evidence/development/r0.3a-t14c/luna-1"),
		T16Evidence:         filepath.Join(cwd, "evidence/development/r0.3a-t16"),
		T17Evidence:         filepath.Join(cwd, "evidence/development/r0.3a-t17"),
		AuthFile:            "/mnt/c/Users/chyinan/.codex/auth.json",
		PolisHostHome:       filepath.Join(cwd, ".runtime/linux/r03a-t14c/home"),
		Evidence:            filepath.Join(cwd, "evidence/development/r0.3a-t19"),
	})
	if result != nil {
		raw, marshalErr := json.Marshal(result)
		if marshalErr == nil {
			fmt.Println(string(raw))
		}
	}
	return err
}
