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
	result, err := probe.RunR03AT17(probe.R03AT17Config{
		WindowsHome:       "/mnt/c/Users/chyinan/.codex",
		WindowsExecutable: "/mnt/c/Users/chyinan/AppData/Local/OpenAI/Codex/bin/fd4c151a749f3ab4/codex.exe",
		PolisHostHome:     filepath.Join(cwd, ".runtime/linux/r03a-t14c/home"),
		PolisGuestHome:    "/home/codex",
		T14CEvidence:      filepath.Join(cwd, "evidence/development/r0.3a-t14c/luna-1"),
		Evidence:          filepath.Join(cwd, "evidence/development/r0.3a-t17"),
	})
	if result != nil {
		raw, marshalErr := json.Marshal(result)
		if marshalErr == nil {
			fmt.Println(string(raw))
		}
	}
	return err
}
