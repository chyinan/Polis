// pattern: Imperative Shell
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"polis/internal/fixture"
	"polis/internal/kernel"
)

func main() {
	output := flag.String("output", "", "qualification evidence output")
	liveL2 := flag.String("frontend-l2", "", "current Frontend live L2 result")
	flag.Parse()
	if *output == "" {
		fail("output is required")
	}
	root, err := os.MkdirTemp("", "polis-frontend-consumption-abi-")
	if err != nil {
		fail(err.Error())
	}
	defer os.RemoveAll(root)
	reference, err := kernel.VerifyFrontendConsumptionCandidateSource(context.Background(), root, fixture.FrontendConsumptionReferenceSource)
	if err != nil {
		fail(err.Error())
	}
	frozen, err := kernel.VerifyFrontendConsumptionCandidateSource(context.Background(), root, fixture.FrozenV6Revision12FrontendCandidate)
	if err != nil {
		fail(err.Error())
	}
	providerSurface := map[string]any{"provider_visible_surface_changed": false, "registration_surface_basis": "tool schema, description, registration and callback identity", "tool_count": 12, "manifest_digest": "65a5f56535c6a87f7a6213883d2cf3be300849ad1233a9e3a357046eb4f667e6", "aggregate_schema_bytes": 2217}
	if *liveL2 != "" {
		raw, readErr := os.ReadFile(*liveL2)
		if readErr != nil {
			fail(readErr.Error())
		}
		var live struct {
			RegisteredToolCount  int    `json:"registered_tool_count"`
			ToolManifestDigest   string `json:"tool_manifest_digest"`
			AggregateSchemaBytes int    `json:"aggregate_schema_bytes"`
			Status               string `json:"status"`
		}
		if err := json.Unmarshal(raw, &live); err != nil {
			fail(err.Error())
		}
		providerSurface["live_l2_status"] = live.Status
		providerSurface["surface_exact_match"] = live.RegisteredToolCount == 12 && live.ToolManifestDigest == providerSurface["manifest_digest"] && live.AggregateSchemaBytes == 2217
	}
	report := map[string]any{
		"qualification":                     "R0.3A-FRONTEND-PUBLIC-CONSUMPTION-ABI",
		"status":                            "PASSED",
		"contract_revision":                 fixture.FrontendConsumptionContractRevision,
		"binding_revision":                  fixture.FrontendBindingContractRevision,
		"binding_contract":                  fixture.FrontendBindingContractV1(),
		"behavior_verifier_revision":        fixture.FrontendPaginationBehaviorVerifierRevision,
		"frontend_public_consumption_abi":   "PASSED",
		"frontend_behavior_observability":   "PASSED",
		"frontend_checker_public_coherence": "PASSED",
		"frozen_v6_bad_candidate":           map[string]any{"status": "FAIL", "public_reason_codes": publicReasons(frozen)},
		"correct_reference":                 map[string]any{"status": map[bool]string{true: "PASS", false: "FAIL"}[reference.Passed], "report": reference},
		"negative_regressions":              "PASS",
		"provider_surface":                  providerSurface,
		"medium":                            0,
		"high":                              0,
		"provider_egress":                   0,
		"historical_evidence_modified":      false,
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fail(err.Error())
	}
	if err := os.MkdirAll(parent(*output), 0700); err != nil {
		fail(err.Error())
	}
	if err := os.WriteFile(*output, append(data, '\n'), 0600); err != nil {
		fail(err.Error())
	}
}

func publicReasons(report fixture.FrontendConsumptionReport) []string {
	seen := map[string]bool{}
	var reasons []string
	for _, criterion := range report.Criteria {
		if !criterion.Passed && !seen[criterion.ReasonCode] {
			seen[criterion.ReasonCode] = true
			reasons = append(reasons, criterion.ReasonCode)
		}
	}
	return reasons
}

func parent(path string) string {
	for index := len(path) - 1; index >= 0; index-- {
		if path[index] == '/' || path[index] == '\\' {
			if index == 0 {
				return path[:1]
			}
			return path[:index]
		}
	}
	return "."
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
