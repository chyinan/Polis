// pattern: Imperative Shell
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"polis/internal/probe"
)

func main() {
	configPath := flag.String("config", "", "explicit Frontend WSL execution config")
	output := flag.String("output", "", "probe report path")
	flag.Parse()
	if *configPath == "" || *output == "" {
		fail("config and output are required")
	}
	_, report, err := probe.LoadFrontendExecutionConfig(*configPath)
	raw, marshalErr := json.MarshalIndent(report, "", "  ")
	if marshalErr != nil {
		fail(marshalErr.Error())
	}
	if writeErr := os.WriteFile(*output, append(raw, '\n'), 0o600); writeErr != nil {
		fail(writeErr.Error())
	}
	if err != nil {
		fail(err.Error())
	}
	if report.Status != "FRONTEND_EXECUTION_CONFIG_READY" {
		fail("FRONTEND_EXECUTION_CONFIG_INVALID")
	}
}

func fail(message string) { fmt.Fprintln(os.Stderr, message); os.Exit(1) }
