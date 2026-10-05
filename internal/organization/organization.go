// pattern: Functional Core
package organization

import (
	"fmt"
	"strings"

	"polis/internal/core"
)

const (
	EmployeePlanning = "emp-planning"
	EmployeeBackend  = "emp-backend"
	EmployeeFrontend = "emp-frontend"
	EmployeeReview   = "emp-review"
	MaxCompanyName   = 200
	MaxWorkspaceRoot = 1024
	MaxEmployeeLabel = 120
	MaxEmployeeRole  = 120
	MaxModelProfile  = 200
)

// EmployeeDraft is the human-editable portion of a fixed logical Employee.
// The ID remains runtime-owned and cannot be changed by a company draft.
type EmployeeDraft struct {
	ID           string `json:"id"`
	DisplayName  string `json:"displayName"`
	Role         string `json:"role"`
	ModelProfile string `json:"modelProfile"`
}

// CompanyDraft is the pure, user-facing organization configuration.
type CompanyDraft struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	WorkspaceRoot string          `json:"workspaceRoot"`
	Roster        []EmployeeDraft `json:"roster"`
}

// DefaultRoster returns the four logical employees required by the current
// product runtime. Callers receive a fresh slice and may edit presentation
// fields without mutating a shared template.
func DefaultRoster() []EmployeeDraft {
	return []EmployeeDraft{
		{ID: EmployeePlanning, DisplayName: "规划工程师", Role: "planning", ModelProfile: "deterministic/fake"},
		{ID: EmployeeBackend, DisplayName: "后端工程师", Role: "backend", ModelProfile: "deterministic/fake"},
		{ID: EmployeeFrontend, DisplayName: "前端工程师", Role: "frontend", ModelProfile: "deterministic/fake"},
		{ID: EmployeeReview, DisplayName: "独立验收员", Role: "review", ModelProfile: "deterministic/fake"},
	}
}

// ValidateCompanyDraft checks the stable runtime roster contract without I/O.
func ValidateCompanyDraft(draft CompanyDraft) error {
	if !core.ValidID(draft.ID) {
		return fmt.Errorf("invalid company id")
	}
	if value := strings.TrimSpace(draft.Name); value == "" || len(value) > MaxCompanyName {
		return fmt.Errorf("company name must be between 1 and %d characters", MaxCompanyName)
	}
	if value := strings.TrimSpace(draft.WorkspaceRoot); value == "" || len(value) > MaxWorkspaceRoot {
		return fmt.Errorf("workspace root must be between 1 and %d characters", MaxWorkspaceRoot)
	}
	if len(draft.Roster) != 4 {
		return fmt.Errorf("company roster must contain exactly four runtime employees")
	}
	seen := make(map[string]struct{}, len(draft.Roster))
	for _, employee := range draft.Roster {
		if _, exists := seen[employee.ID]; exists {
			return fmt.Errorf("company roster contains duplicate employee %q", employee.ID)
		}
		seen[employee.ID] = struct{}{}
		if !knownEmployee(employee.ID) {
			return fmt.Errorf("company roster contains unknown employee %q", employee.ID)
		}
		if value := strings.TrimSpace(employee.DisplayName); value == "" || len(value) > MaxEmployeeLabel {
			return fmt.Errorf("employee %q display name is invalid", employee.ID)
		}
		if value := strings.TrimSpace(employee.Role); value == "" || len(value) > MaxEmployeeRole {
			return fmt.Errorf("employee %q role is invalid", employee.ID)
		}
		if value := strings.TrimSpace(employee.ModelProfile); value == "" || len(value) > MaxModelProfile {
			return fmt.Errorf("employee %q model profile is invalid", employee.ID)
		}
	}
	for _, employeeID := range []string{EmployeePlanning, EmployeeBackend, EmployeeFrontend, EmployeeReview} {
		if _, exists := seen[employeeID]; !exists {
			return fmt.Errorf("company roster is missing employee %q", employeeID)
		}
	}
	return nil
}

// ValidateFixedTeamRoleAssignments enforces the role names for the product's
// fixed logical Employees. Display names and model profiles remain editable.
func ValidateFixedTeamRoleAssignments(roster []EmployeeDraft) error {
	expected := DefaultRoster()
	if len(roster) != len(expected) {
		return fmt.Errorf("fixed team roster must contain exactly %d employees", len(expected))
	}
	roles := make(map[string]string, len(roster))
	for _, employee := range roster {
		roles[employee.ID] = strings.TrimSpace(employee.Role)
	}
	for _, employee := range expected {
		if role, ok := roles[employee.ID]; !ok || role != employee.Role {
			return fmt.Errorf("employee %q must keep the fixed %q role", employee.ID, employee.Role)
		}
	}
	return nil
}

func knownEmployee(id string) bool {
	switch id {
	case EmployeePlanning, EmployeeBackend, EmployeeFrontend, EmployeeReview:
		return true
	default:
		return false
	}
}
