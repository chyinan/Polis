package spec

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
)

//go:embed design-v0.4.5/product/TEAM_COVERAGE.json
var fixedTeamCoverage []byte

// FixedTeamCoverageSHA256 identifies the exact reviewed team coverage draft.
func FixedTeamCoverageSHA256() string {
	digest := sha256.Sum256(fixedTeamCoverage)
	return hex.EncodeToString(digest[:])
}

// validateFixedTeamCoverageFieldNames rejects keys that Go's struct decoder
// would otherwise match case-insensitively even though the frontend reads the
// embedded JSON with case-sensitive property access.
func validateFixedTeamCoverageFieldNames(data []byte) error {
	var document map[string]json.RawMessage
	if err := json.Unmarshal(data, &document); err != nil {
		return fmt.Errorf("decode fixed team coverage field names: %w", err)
	}
	if err := requireExactJSONKeys(document, []string{
		"document_version", "design_kind", "execution_enabled", "role_changes_at_runtime",
		"template_requires_human_confirmation", "employee_ids", "coverage", "missing_path",
		"checker_policy", "trusted_baseline_mutable_by_workers", "guarantees_semantic_independence",
	}); err != nil {
		return fmt.Errorf("fixed team coverage top-level fields: %w", err)
	}
	var coverage []json.RawMessage
	if err := json.Unmarshal(document["coverage"], &coverage); err != nil {
		return fmt.Errorf("decode fixed team coverage assignments: %w", err)
	}
	for i, assignment := range coverage {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(assignment, &fields); err != nil {
			return fmt.Errorf("decode fixed team coverage assignment %d fields: %w", i+1, err)
		}
		if err := requireExactJSONKeys(fields, []string{
			"task_type", "owner", "eligible_independent_checkers", "acceptance_path", "qualification",
		}); err != nil {
			return fmt.Errorf("fixed team coverage assignment %d fields: %w", i+1, err)
		}
	}
	return nil
}

func requireExactJSONKeys(object map[string]json.RawMessage, expected []string) error {
	if len(object) != len(expected) {
		return fmt.Errorf("expected exactly %d fields, got %d", len(expected), len(object))
	}
	for _, key := range expected {
		if _, ok := object[key]; !ok {
			return fmt.Errorf("missing or non-canonical field %q", key)
		}
	}
	return nil
}

// ValidateFixedTeamCoverageDraft verifies that the embedded design file still
// describes the reviewed, non-executable draft. The digest alone identifies
// bytes; this semantic guard prevents a source edit from making an owner
// acknowledgment look like qualification or runtime execution permission.
func ValidateFixedTeamCoverageDraft() error {
	if err := validateFixedTeamCoverageFieldNames(fixedTeamCoverage); err != nil {
		return err
	}
	var draft struct {
		DocumentVersion                   string   `json:"document_version"`
		DesignKind                        string   `json:"design_kind"`
		ExecutionEnabled                  *bool    `json:"execution_enabled"`
		RoleChangesAtRuntime              *bool    `json:"role_changes_at_runtime"`
		TemplateRequiresHumanConfirmation *bool    `json:"template_requires_human_confirmation"`
		EmployeeIDs                       []string `json:"employee_ids"`
		Coverage                          []struct {
			TaskType                    string   `json:"task_type"`
			Owner                       string   `json:"owner"`
			EligibleIndependentCheckers []string `json:"eligible_independent_checkers"`
			AcceptancePath              string   `json:"acceptance_path"`
			Qualification               string   `json:"qualification"`
		} `json:"coverage"`
		MissingPath                     string `json:"missing_path"`
		CheckerPolicy                   string `json:"checker_policy"`
		TrustedBaselineMutableByWorkers *bool  `json:"trusted_baseline_mutable_by_workers"`
		GuaranteesSemanticIndependence  *bool  `json:"guarantees_semantic_independence"`
	}
	decoder := json.NewDecoder(bytes.NewReader(fixedTeamCoverage))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&draft); err != nil {
		return fmt.Errorf("decode fixed team coverage draft: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		if err == nil {
			return fmt.Errorf("fixed team coverage draft must contain one JSON value")
		}
		return fmt.Errorf("decode trailing fixed team coverage data: %w", err)
	}
	if draft.DocumentVersion != "0.4.5" || draft.DesignKind != "fixed_team_coverage_draft" ||
		draft.ExecutionEnabled == nil || *draft.ExecutionEnabled ||
		draft.RoleChangesAtRuntime == nil || *draft.RoleChangesAtRuntime ||
		draft.TemplateRequiresHumanConfirmation == nil || !*draft.TemplateRequiresHumanConfirmation ||
		draft.MissingPath != "show_requires_human_before_admission" ||
		draft.CheckerPolicy != "not author of exact candidate; test-overlay review does not permit production self-acceptance" ||
		draft.TrustedBaselineMutableByWorkers == nil || *draft.TrustedBaselineMutableByWorkers ||
		draft.GuaranteesSemanticIndependence == nil || *draft.GuaranteesSemanticIndependence {
		return fmt.Errorf("fixed team coverage draft has unsafe or unreviewed top-level contract fields")
	}

	expectedEmployees := []string{"emp-planning", "emp-backend", "emp-frontend", "emp-review"}
	if !sameStrings(draft.EmployeeIDs, expectedEmployees) {
		return fmt.Errorf("fixed team coverage draft employee roster differs from the reviewed roster")
	}
	type expectedAssignment struct {
		taskType   string
		owner      string
		checkers   []string
		acceptance string
	}
	expectedCoverage := []expectedAssignment{
		{taskType: "planning", owner: "emp-planning", checkers: []string{"emp-review"}, acceptance: "owner_protected_goal_contract"},
		{taskType: "frontend", owner: "emp-frontend", checkers: []string{"emp-review"}, acceptance: "trusted_baseline_plus_independent_review"},
		{taskType: "backend", owner: "emp-backend", checkers: []string{"emp-review"}, acceptance: "trusted_baseline_plus_independent_review"},
		{taskType: "environment_plan", owner: "emp-backend", checkers: []string{"emp-planning"}, acceptance: "current_environment_policy"},
		{taskType: "regression_test_overlay", owner: "emp-review", checkers: []string{"emp-backend", "emp-frontend"}, acceptance: "separate_test_quality_review_not_final_production_acceptance"},
		{taskType: "design_change", owner: "emp-planning", checkers: []string{"emp-review"}, acceptance: "risk_policy_owner_if_outside_scope"},
		{taskType: "delivery_assembly", owner: "emp-planning", checkers: []string{"emp-review"}, acceptance: "immutable_manifest_and_deterministic_checks"},
	}
	if len(draft.Coverage) != len(expectedCoverage) {
		return fmt.Errorf("fixed team coverage draft must contain exactly %d assignments", len(expectedCoverage))
	}
	for i, expected := range expectedCoverage {
		actual := draft.Coverage[i]
		if actual.TaskType != expected.taskType || actual.Owner != expected.owner ||
			!sameStrings(actual.EligibleIndependentCheckers, expected.checkers) ||
			actual.AcceptancePath != expected.acceptance || actual.Qualification != "unverified" {
			return fmt.Errorf("fixed team coverage assignment %d differs from the reviewed unverified draft", i+1)
		}
	}
	return nil
}

func sameStrings(actual, expected []string) bool {
	if len(actual) != len(expected) {
		return false
	}
	for i := range expected {
		if actual[i] != expected[i] {
			return false
		}
	}
	return true
}
