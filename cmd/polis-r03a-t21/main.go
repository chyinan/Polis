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
	mode := flag.String("mode", "preflight", "preflight or finalize")
	flag.Parse()
	cwd, err := os.Getwd()
	if err != nil {
		fail(err)
	}
	cfg := probe.R03AT21Config{
		WindowsBinary:       "/mnt/c/Users/chyinan/AppData/Local/OpenAI/Codex/bin/fd4c151a749f3ab4/codex.exe",
		WindowsCodeModeHost: "/mnt/c/Users/chyinan/AppData/Local/OpenAI/Codex/bin/fd4c151a749f3ab4/codex-code-mode-host.exe",
		T5Evidence:          filepath.Join(cwd, "evidence/development/r0.3a-t5"),
		T20Evidence:         filepath.Join(cwd, "evidence/development/r0.3a-t20"),
		AuthFile:            "/mnt/c/Users/chyinan/.codex/auth.json",
		SelectedLinuxConfig: filepath.Join(cwd, ".runtime/linux/r03a-t14c/home/config.toml"),
		Evidence:            filepath.Join(cwd, "evidence/development/r0.3a-t21"),
	}
	var result any
	switch *mode {
	case "preflight":
		result, err = probe.RunR03AT21Preflight(cfg)
	case "finalize":
		result, err = probe.FinalizeR03AT21(cfg)
	default:
		fail(fmt.Errorf("unsupported mode %q", *mode))
	}
	if result != nil {
		raw, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			fail(marshalErr)
		}
		fmt.Println(string(raw))
	}
	if err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
