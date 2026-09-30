// pattern: Imperative Shell
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"polis/internal/kernel"
)

func main() {
	dsn := flag.String("dsn", "", "dedicated read-only Frontend starting-state DSN")
	output := flag.String("output", "", "JSON report path")
	flag.Parse()
	if *dsn == "" || *output == "" {
		fail("dsn and output are required")
	}
	state, err := kernel.ProbeFrontendStartingState(context.Background(), *dsn)
	reasons := kernel.ValidateFrontendStartingState(state)
	report := map[string]any{"status": "FRONTEND_STARTING_STATE_READY", "read_only": true, "state": state, "reasons": reasons, "mutation": false}
	if err != nil {
		report["status"] = "FRONTEND_STARTING_STATE_UNAVAILABLE"
		report["error"] = err.Error()
	}
	if len(reasons) != 0 {
		report["status"] = "FRONTEND_STARTING_STATE_MISMATCH"
	}
	raw, marshalErr := json.MarshalIndent(report, "", "  ")
	if marshalErr != nil {
		fail(marshalErr.Error())
	}
	if writeErr := os.WriteFile(*output, append(raw, '\n'), 0o600); writeErr != nil {
		fail(writeErr.Error())
	}
	if err != nil || len(reasons) != 0 {
		fail(report["status"].(string))
	}
}

func fail(message string) { fmt.Fprintln(os.Stderr, message); os.Exit(1) }
