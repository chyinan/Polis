// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
	"polis/internal/organization"
)

type FixedTeamRoleReconciliationInput struct {
	ExpectedSnapshotSHA256 string
	Rationale              string
	RequestID              string
}

// lockCompanyFixedRoleStateTX serializes operations that can begin Company
// work, change fixed roles, or archive a Company with an owner-approved
// reconciliation. Use an advisory lock rather than locking the Company row
// before subsidiary budget/task rows; many existing writers append their event
// (and lock the Company row) only after those rows are held.
func lockCompanyFixedRoleStateTX(ctx context.Context, tx pgx.Tx, companyID string) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, capabilitySourceAdvisoryLockKey(companyID, "fixed-team-role-state")); err != nil {
		return err
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM companies WHERE id=$1)`, companyID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return core.OutOfScope
	}
	return nil
}

// TXReconcileFixedTeamRoleAssignments performs a one-time owner-reviewed
// correction of legacy role-name drift to the already-approved canonical
// employee-ID/role pairs. It requires an exact read-preview digest and a fully
// idle Company; it does not confirm coverage or qualify execution.
func (k *Kernel) TXReconcileFixedTeamRoleAssignments(ctx context.Context, companyID string, input FixedTeamRoleReconciliationInput) (Receipt, error) {
	if k == nil {
		return Receipt{}, core.StaleEpoch
	}
	if !core.ValidID(companyID) || !validEnvironmentSHA256(input.ExpectedSnapshotSHA256) || !core.ValidID(input.RequestID) ||
		strings.TrimSpace(input.Rationale) == "" || len(input.Rationale) > 512 {
		return Receipt{}, core.Malformed
	}
	input.Rationale = strings.TrimSpace(input.Rationale)
	return k.TXWrite(ctx, k.LocalScope(companyID), nil, input.RequestID, "company.team_roles.reconcile", input, func(tx pgx.Tx) (Receipt, error) {
		if err := lockCompanyFixedRoleStateTX(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		var companyState string
		if err := tx.QueryRow(ctx, `SELECT state FROM companies WHERE id=$1`, companyID).Scan(&companyState); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		rows, err := tx.Query(ctx, `SELECT id,display_name,role_name,model_profile FROM employees WHERE company_id=$1 AND enabled ORDER BY id FOR UPDATE`, companyID)
		if err != nil {
			return Receipt{}, err
		}
		roster := make([]organization.EmployeeDraft, 0, 4)
		for rows.Next() {
			var employee organization.EmployeeDraft
			if err := rows.Scan(&employee.ID, &employee.DisplayName, &employee.Role, &employee.ModelProfile); err != nil {
				rows.Close()
				return Receipt{}, err
			}
			roster = append(roster, employee)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return Receipt{}, err
		}
		preview, err := fixedTeamRoleReconciliationPreviewTX(ctx, tx, companyID, companyState, roster)
		if err != nil {
			return Receipt{}, err
		}
		if preview.SnapshotSHA256 != input.ExpectedSnapshotSHA256 {
			return Receipt{}, core.ConflictError{Reason: "fixed-role reconciliation snapshot changed; reread and review the exact mismatch list", CurrentState: "snapshot_changed"}
		}
		if !preview.Eligible {
			return Receipt{}, core.ConflictError{Reason: "fixed-role reconciliation requires an active idle Company and a role mismatch", CurrentState: preview.ReasonCode}
		}
		expected := organization.DefaultRoster()
		expectedRoles := make(map[string]string, len(expected))
		for _, employee := range expected {
			expectedRoles[employee.ID] = employee.Role
		}
		for _, employee := range roster {
			role := expectedRoles[employee.ID]
			if role == employee.Role {
				continue
			}
			commandTag, err := tx.Exec(ctx, `UPDATE employees SET role_name=$3 WHERE company_id=$1 AND id=$2 AND enabled AND role_name=$4`, companyID, employee.ID, role, employee.Role)
			if err != nil {
				return Receipt{}, err
			}
			if commandTag.RowsAffected() != 1 {
				return Receipt{}, core.Conflict
			}
		}
		if err := organization.ValidateFixedTeamRoleAssignments(func() []organization.EmployeeDraft {
			for index := range roster {
				if role, exists := expectedRoles[roster[index].ID]; exists {
					roster[index].Role = role
				}
			}
			return roster
		}()); err != nil {
			return Receipt{}, core.Integrity
		}
		after, err := organization.BuildFixedTeamRoleReconciliationPreview(companyID, companyState, roster, false)
		if err != nil || after.ReasonCode != "no_role_drift" {
			return Receipt{}, core.Integrity
		}
		beforeJSON, err := json.Marshal(preview.Mismatches)
		if err != nil {
			return Receipt{}, err
		}
		payload := struct {
			RequestID            string          `json:"requestId"`
			BeforeSnapshotSHA256 string          `json:"beforeSnapshotSha256"`
			AfterSnapshotSHA256  string          `json:"afterSnapshotSha256"`
			Mismatches           json.RawMessage `json:"mismatches"`
			Rationale            string          `json:"rationale"`
		}{input.RequestID, preview.SnapshotSHA256, after.SnapshotSHA256, beforeJSON, input.Rationale}
		if err := appendEvent(ctx, tx, k.LocalScope(companyID), "company.team_roles.reconciled", payload); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: companyID, Status: "reconciled"}, nil
	})
}
