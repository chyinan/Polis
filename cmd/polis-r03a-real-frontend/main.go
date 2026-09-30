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
	initialOnly := flag.Bool("initial-only", false, "run only the Frontend initial handover boundary")
	executionConfigPath := flag.String("execution-config", "", "explicit shared Windows-to-WSL Frontend execution config")
	flag.Parse()
	if *executionConfigPath == "" {
		fmt.Fprintln(os.Stderr, "FRONTEND_EXECUTION_CONFIG_INVALID: -execution-config is required")
		os.Exit(1)
	}
	executionConfig, report, err := probe.LoadFrontendExecutionConfig(*executionConfigPath)
	if err != nil {
		raw, _ := json.Marshal(report)
		fmt.Fprintln(os.Stderr, string(raw))
		os.Exit(1)
	}
	result, err := probe.RunR03AFrontendHandover(executionConfig.R03A(*initialOnly))
	if result != nil {
		fmt.Printf("frontend-handover status=%v initial=%v successor=%v real_handover=%v collaboration=%v\n", result["status"], result["frontend_initial"], result["frontend_successor"], result["real_frontend_handover"], result["real_peer_collaboration"])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
