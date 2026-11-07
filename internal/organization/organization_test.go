// pattern: Functional Core
package organization

import "testing"

func TestValidateCompanyDraftRequiresRuntimeRoster(t *testing.T) {
	draft := CompanyDraft{
		ID:            "dogfood-company",
		Name:          "Dogfood Company",
		WorkspaceRoot: "C:/workspace/dogfood",
		Roster: []EmployeeDraft{
			{ID: "emp-planning", DisplayName: "Planner", Role: "planning", ModelProfile: "deterministic/fake"},
			{ID: "emp-backend", DisplayName: "Backend", Role: "backend", ModelProfile: "deterministic/fake"},
			{ID: "emp-frontend", DisplayName: "Frontend", Role: "frontend", ModelProfile: "deterministic/fake"},
			{ID: "emp-review", DisplayName: "Reviewer", Role: "review", ModelProfile: "deterministic/fake"},
		},
	}

	if err := ValidateCompanyDraft(draft); err != nil {
		t.Fatalf("ValidateCompanyDraft: %v", err)
	}
}

func TestValidateCompanyDraftRejectsDuplicateOrUnknownRosterMembers(t *testing.T) {
	draft := CompanyDraft{
		ID:            "dogfood-company",
		Name:          "Dogfood Company",
		WorkspaceRoot: "C:/workspace/dogfood",
		Roster: []EmployeeDraft{
			{ID: "emp-planning", DisplayName: "Planner", Role: "planning", ModelProfile: "deterministic/fake"},
			{ID: "emp-backend", DisplayName: "Backend", Role: "backend", ModelProfile: "deterministic/fake"},
			{ID: "emp-frontend", DisplayName: "Frontend", Role: "frontend", ModelProfile: "deterministic/fake"},
			{ID: "emp-frontend", DisplayName: "Reviewer", Role: "review", ModelProfile: "deterministic/fake"},
		},
	}

	if err := ValidateCompanyDraft(draft); err == nil {
		t.Fatal("ValidateCompanyDraft unexpectedly accepted duplicate roster member")
	}
}
