// pattern: Functional Core
package organization

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"polis/internal/core"
)

var errFixedRoleReconciliation = errors.New("invalid fixed-role reconciliation snapshot")

type FixedTeamRoleMismatch struct {
	EmployeeID   string `json:"employeeId"`
	CurrentRole  string `json:"currentRole"`
	ExpectedRole string `json:"expectedRole"`
}

type FixedTeamRoleReconciliationPreview struct {
	SnapshotSHA256 string                  `json:"snapshotSha256"`
	Mismatches     []FixedTeamRoleMismatch `json:"mismatches"`
	Eligible       bool                    `json:"eligible"`
	ReasonCode     string                  `json:"reasonCode"`
}

type fixedRoleReconciliationAssignment struct {
	EmployeeID   string `json:"employeeId"`
	CurrentRole  string `json:"currentRole"`
	ExpectedRole string `json:"expectedRole"`
}

type fixedRoleReconciliationSnapshot struct {
	SchemaVersion string                              `json:"schemaVersion"`
	CompanyID     string                              `json:"companyId"`
	Assignments   []fixedRoleReconciliationAssignment `json:"assignments"`
}

// BuildFixedTeamRoleReconciliationPreview describes only a one-time repair to
// the repository's canonical fixed roles. The digest binds every current and
// expected role; eligibility is false while any work is active or the company
// is not active. It never changes roles or qualifies an employee.
func BuildFixedTeamRoleReconciliationPreview(companyID, companyState string, roster []EmployeeDraft, hasActiveWork bool) (FixedTeamRoleReconciliationPreview, error) {
	if !core.ValidID(companyID) || len(roster) != len(DefaultRoster()) {
		return FixedTeamRoleReconciliationPreview{}, errFixedRoleReconciliation
	}
	current := make(map[string]string, len(roster))
	for _, employee := range roster {
		if !knownEmployee(employee.ID) {
			return FixedTeamRoleReconciliationPreview{}, errFixedRoleReconciliation
		}
		if _, duplicate := current[employee.ID]; duplicate {
			return FixedTeamRoleReconciliationPreview{}, errFixedRoleReconciliation
		}
		current[employee.ID] = employee.Role
	}
	expected := DefaultRoster()
	assignments := make([]fixedRoleReconciliationAssignment, 0, len(expected))
	mismatches := make([]FixedTeamRoleMismatch, 0, len(expected))
	for _, employee := range expected {
		currentRole, exists := current[employee.ID]
		if !exists {
			return FixedTeamRoleReconciliationPreview{}, errFixedRoleReconciliation
		}
		assignments = append(assignments, fixedRoleReconciliationAssignment{EmployeeID: employee.ID, CurrentRole: currentRole, ExpectedRole: employee.Role})
		if currentRole != employee.Role {
			mismatches = append(mismatches, FixedTeamRoleMismatch{EmployeeID: employee.ID, CurrentRole: currentRole, ExpectedRole: employee.Role})
		}
	}
	canonical, err := json.Marshal(fixedRoleReconciliationSnapshot{SchemaVersion: "fixed-team-role-reconciliation@1", CompanyID: companyID, Assignments: assignments})
	if err != nil {
		return FixedTeamRoleReconciliationPreview{}, errFixedRoleReconciliation
	}
	digest := sha256.Sum256(canonical)
	preview := FixedTeamRoleReconciliationPreview{
		SnapshotSHA256: hex.EncodeToString(digest[:]), Mismatches: mismatches,
		Eligible: len(mismatches) > 0 && companyState == "active" && !hasActiveWork,
	}
	switch {
	case len(mismatches) == 0:
		preview.ReasonCode = "no_role_drift"
	case companyState != "active":
		preview.ReasonCode = "company_not_active"
	case hasActiveWork:
		preview.ReasonCode = "active_work_present"
	default:
		preview.ReasonCode = "owner_reconciliation_required"
	}
	return preview, nil
}
