// pattern: Imperative Shell
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"polis/internal/probe"
	"strings"
)

type frozenResult struct {
	Status                string          `json:"status"`
	AllowanceCreated      bool            `json:"allowance_created"`
	ProviderEgress        int             `json:"provider_egress"`
	FrontendRealExecution string          `json:"frontend_real_execution"`
	TransportOutcome      string          `json:"transport_outcome_classification"`
	FrontendSession       json.RawMessage `json:"frontend_session"`
}

type regression struct {
	Name   string `json:"name"`
	Actual string `json:"actual"`
	Wanted string `json:"wanted"`
}

func main() {
	input := flag.String("input", "", "frozen result.json to adjudicate")
	output := flag.String("output", "", "independent adjudication evidence path")
	flag.Parse()
	if err := run(*input, *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(inputPath, outputPath string) error {
	if strings.TrimSpace(inputPath) == "" || strings.TrimSpace(outputPath) == "" {
		return errors.New("input and output are required")
	}
	raw, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read frozen result: %w", err)
	}
	var result frozenResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return fmt.Errorf("decode frozen result: %w", err)
	}
	sessionCreated := len(result.FrontendSession) != 0 && string(result.FrontendSession) != "null"
	actual := probe.DeriveFrontendTerminalStatus(probe.FrontendTerminalExecution{
		AllowanceConsumed: result.AllowanceCreated,
		SessionCreated:    sessionCreated,
		ProviderEgress:    result.ProviderEgress > 0,
		BusinessVerdict:   result.FrontendRealExecution,
		TransportOutcome:  result.TransportOutcome,
	})
	if result.Status != probe.FrontendTerminalNotStarted || actual != probe.FrontendTerminalFailed {
		return fmt.Errorf("frozen V7 evidence did not match expected regression: raw=%s derived=%s", result.Status, actual)
	}
	regressions := []regression{
		{Name: "pre-provider failure", Actual: probe.DeriveFrontendTerminalStatus(probe.FrontendTerminalExecution{}), Wanted: probe.FrontendTerminalNotStarted},
		{Name: "provider transport inconclusive", Actual: probe.DeriveFrontendTerminalStatus(probe.FrontendTerminalExecution{AllowanceConsumed: true, SessionCreated: true, ProviderEgress: true, TransportOutcome: "PROVIDER_STREAM_NONRECOVERABLE", BusinessVerdict: probe.FrontendTerminalFailed}), Wanted: probe.FrontendTerminalInconclusive},
		{Name: "completed business failure without artifact", Actual: probe.DeriveFrontendTerminalStatus(probe.FrontendTerminalExecution{AllowanceConsumed: true, SessionCreated: true, ProviderEgress: true, BusinessVerdict: probe.FrontendTerminalFailed}), Wanted: probe.FrontendTerminalFailed},
		{Name: "successful business", Actual: probe.DeriveFrontendTerminalStatus(probe.FrontendTerminalExecution{AllowanceConsumed: true, SessionCreated: true, ProviderEgress: true, BusinessVerdict: probe.FrontendTerminalPassed}), Wanted: probe.FrontendTerminalPassed},
	}
	for _, item := range regressions {
		if item.Actual != item.Wanted {
			return fmt.Errorf("terminal status regression failed: %s: got %s want %s", item.Name, item.Actual, item.Wanted)
		}
	}
	if _, err := os.Stat(outputPath); err == nil {
		return fmt.Errorf("refusing to overwrite adjudication evidence: %s", outputPath)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0700); err != nil {
		return err
	}
	evidence := map[string]any{
		"record_type":                  "r0.3a-terminal-result-status-adjudication-v1",
		"source_result_path":           inputPath,
		"frozen_v7_raw_status":         result.Status,
		"frozen_v7_adjudicated_status": actual,
		"provider_egress":              result.ProviderEgress,
		"session_present":              sessionCreated,
		"frontend_real_execution":      result.FrontendRealExecution,
		"artifact_absent_is_terminal":  true,
		"regressions":                  regressions,
		"historical_evidence_modified": false,
	}
	encoded, err := json.MarshalIndent(evidence, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(append(encoded, '\n')); err != nil {
		return err
	}
	return nil
}
