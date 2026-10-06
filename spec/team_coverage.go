// pattern: Functional Core
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

// FixedTeamCoverageAssignment is one semantic task-type contract from the
// reviewed fixed-team matrix. TaskType is intentionally separate from the
// lower-level persisted TaskKind discriminator.
type FixedTeamCoverageAssignment struct {
	TaskType                    string   `json:"task_type"`
	Owner                       string   `json:"owner"`
	EligibleIndependentCheckers []string `json:"eligible_independent_checkers"`
	AcceptancePath              string   `json:"acceptance_path"`
	Qualification               string   `json:"qualification"`
}

// FixedTeamCoverageDraft is the exact semantic role matrix compiled from the
// embedded approved design draft. Its flags and qualification values are
// retained as authored; compilation does not enable execution or qualify a
// role.
type FixedTeamCoverageDraft struct {
	DocumentVersion                   string                        `json:"document_version"`
	DesignKind                        string                        `json:"design_kind"`
	ExecutionEnabled                  bool                          `json:"execution_enabled"`
	RoleChangesAtRuntime              bool                          `json:"role_changes_at_runtime"`
	TemplateRequiresHumanConfirmation bool                          `json:"template_requires_human_confirmation"`
	EmployeeIDs                       []string                      `json:"employee_ids"`
	Coverage                          []FixedTeamCoverageAssignment `json:"coverage"`
	MissingPath                       string                        `json:"missing_path"`
	CheckerPolicy                     string                        `json:"checker_policy"`
	TrustedBaselineMutableByWorkers   bool                          `json:"trusted_baseline_mutable_by_workers"`
	GuaranteesSemanticIndependence    bool                          `json:"guarantees_semantic_independence"`
}

// FixedTeamCoverageRoleRevision binds the owner confirmation to a stable
// content-addressed team-matrix revision. It is not a per-employee ordinal
// RoleRevision and the assignment vocabulary remains independent of persisted
// TaskKind values.
type FixedTeamCoverageRoleRevision struct {
	RevisionSHA256 string                 `json:"revision_sha256"`
	OwnerDecision  string                 `json:"owner_decision"`
	Qualification  string                 `json:"qualification"`
	Contract       FixedTeamCoverageDraft `json:"contract"`
	compiled       bool
}

// FixedTeamCoverageAdmission is the deterministic result of resolving a
// semantic task type against a fixed-team draft. Uncovered, unqualified, or
// globally disabled work always requires a human path.
type FixedTeamCoverageAdmission struct {
	TaskType                    string   `json:"task_type"`
	TaskKind                    string   `json:"task_kind"`
	Owner                       string   `json:"owner"`
	EligibleIndependentCheckers []string `json:"eligible_independent_checkers"`
	AcceptancePath              string   `json:"acceptance_path"`
	Qualification               string   `json:"qualification"`
	Covered                     bool     `json:"covered"`
	OwnerMatches                bool     `json:"owner_matches"`
	RequiresHuman               bool     `json:"requires_human"`
}

// FixedTeamCoverageTaskKind is the owner-selected semantic-to-runtime
// vocabulary bridge. It is descriptive only: execution remains disabled until
// a separately qualified Role/TaskRevision policy enables an admission path.
func FixedTeamCoverageTaskKind(taskType string) string {
	switch taskType {
	case "planning", "design_change":
		return "bootstrap_plan"
	case "frontend":
		return "peer_frontend"
	case "backend":
		return "compat"
	case "environment_plan", "delivery_assembly":
		return "compute"
	case "regression_test_overlay":
		return "review"
	default:
		return ""
	}
}

const FixedTeamCoverageOwnerDecision = "installation_owner_confirmed_fixed_team_mapping"

// FixedTeamCoverageSHA256 identifies the exact reviewed team coverage draft.
func FixedTeamCoverageSHA256() string {
	digest := sha256.Sum256(fixedTeamCoverage)
	return hex.EncodeToString(digest[:])
}

// FixedTeamCoverageDraftSnapshot returns an independent copy of the exact
// reviewed bytes for durable storage alongside an owner acknowledgment.
func FixedTeamCoverageDraftSnapshot() []byte {
	return append([]byte(nil), fixedTeamCoverage...)
}

// CompileFixedTeamCoverageDraft validates and compiles the exact embedded
// matrix without translating its semantic task types into runtime TaskKinds.
func CompileFixedTeamCoverageDraft() (FixedTeamCoverageDraft, error) {
	if err := ValidateFixedTeamCoverageDraft(); err != nil {
		return FixedTeamCoverageDraft{}, err
	}
	var draft FixedTeamCoverageDraft
	decoder := json.NewDecoder(bytes.NewReader(fixedTeamCoverage))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&draft); err != nil {
		return FixedTeamCoverageDraft{}, fmt.Errorf("compile fixed team coverage draft: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		if err == nil {
			return FixedTeamCoverageDraft{}, fmt.Errorf("fixed team coverage draft must contain one JSON value")
		}
		return FixedTeamCoverageDraft{}, fmt.Errorf("decode trailing fixed team coverage data: %w", err)
	}
	return draft, nil
}

// CompileFixedTeamCoverageRoleRevision verifies a stored owner-confirmation
// snapshot and compiles it to the exact current semantic contract. A draft
// acknowledgment is not qualification or execution permission.
func CompileFixedTeamCoverageRoleRevision(snapshot []byte, revisionSHA256, ownerDecision, qualification string) (FixedTeamCoverageRoleRevision, error) {
	digest := sha256.Sum256(snapshot)
	if len(snapshot) == 0 || hex.EncodeToString(digest[:]) != revisionSHA256 ||
		revisionSHA256 != FixedTeamCoverageSHA256() || !bytes.Equal(snapshot, fixedTeamCoverage) {
		return FixedTeamCoverageRoleRevision{}, fmt.Errorf("stored fixed team role revision does not match the current reviewed matrix")
	}
	if ownerDecision != FixedTeamCoverageOwnerDecision || qualification != "unverified" {
		return FixedTeamCoverageRoleRevision{}, fmt.Errorf("stored fixed team role revision has an invalid owner decision or qualification")
	}
	contract, err := CompileFixedTeamCoverageDraft()
	if err != nil {
		return FixedTeamCoverageRoleRevision{}, err
	}
	return FixedTeamCoverageRoleRevision{
		RevisionSHA256: revisionSHA256,
		OwnerDecision:  ownerDecision,
		Qualification:  qualification,
		Contract:       contract,
		compiled:       true,
	}, nil
}

// AdmissionFor resolves only an owner-confirmed revision's semantic task_type vocabulary.
// Unknown types, owner mismatches, disabled execution, and unqualified roles
// remain on the human-required path.
func (revision FixedTeamCoverageRoleRevision) AdmissionFor(taskType, owner string) FixedTeamCoverageAdmission {
	result := FixedTeamCoverageAdmission{TaskType: taskType, RequiresHuman: true}
	if !revision.compiled || revision.RevisionSHA256 != FixedTeamCoverageSHA256() || revision.OwnerDecision != FixedTeamCoverageOwnerDecision {
		return result
	}
	for _, assignment := range revision.Contract.Coverage {
		if assignment.TaskType != taskType {
			continue
		}
		result.TaskKind = FixedTeamCoverageTaskKind(taskType)
		result.Owner = assignment.Owner
		result.EligibleIndependentCheckers = append([]string(nil), assignment.EligibleIndependentCheckers...)
		result.AcceptancePath = assignment.AcceptancePath
		result.Qualification = assignment.Qualification
		result.Covered = true
		result.OwnerMatches = owner == assignment.Owner
		result.RequiresHuman = !result.OwnerMatches || revision.Qualification != "qualified" ||
			!revision.Contract.ExecutionEnabled || assignment.Qualification != "qualified"
		return result
	}
	return result
}

// EligibleIndependentChecker checks both the reviewed checker set and the
// matrix rule that the checker did not author the exact candidate.
func (assignment FixedTeamCoverageAssignment) EligibleIndependentChecker(checkerID, candidateAuthorID string) bool {
	if checkerID == "" || candidateAuthorID == "" || checkerID == candidateAuthorID {
		return false
	}
	for _, eligible := range assignment.EligibleIndependentCheckers {
		if checkerID == eligible {
			return true
		}
	}
	return false
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
