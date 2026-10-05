// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"polis/internal/core"
	"polis/internal/organization"
	"polis/spec"
)

// CompanyDetails is the authorized organization projection used by product
// settings and the company switcher. It contains no provider credentials.
type CompanyDetails struct {
	ID                             string
	Name                           string
	WorkspaceRoot                  string
	State                          string
	Provider                       string
	Model                          string
	Effort                         string
	Profile                        string
	Roster                         []organization.EmployeeDraft
	TeamCoverageConfirmed          bool
	TeamCoverageConfirmationSHA256 string
	TeamCoverageConfirmedAt        string
}

// TXCreateCompanyWithOrganization creates the fixed logical runtime roster in
// one transaction. It does not start a Mission or provider session.
func (k *Kernel) TXCreateCompanyWithOrganization(ctx context.Context, draft organization.CompanyDraft) (Scope, error) {
	return k.TXCreateCompanyWithOrganizationAndCoverageConfirmation(ctx, draft, "", "")
}

// TXCreateCompanyWithOrganizationAndCoverageConfirmation optionally records
// the installation owner's acknowledgment of the exact fixed-team draft in
// the same transaction as Company creation. It never qualifies a role.
func (k *Kernel) TXCreateCompanyWithOrganizationAndCoverageConfirmation(ctx context.Context, draft organization.CompanyDraft, requestID, confirmationSHA256 string) (Scope, error) {
	if err := organization.ValidateCompanyDraft(draft); err != nil {
		return Scope{}, core.Malformed
	}
	if confirmationSHA256 != "" && (confirmationSHA256 != spec.FixedTeamCoverageSHA256() || !core.ValidID(requestID)) {
		return Scope{}, core.Malformed
	}
	scope := Scope{company: draft.ID}
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return Scope{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `INSERT INTO companies(id,name,workspace_root,state) VALUES($1,$2,$3,'active')`, draft.ID, strings.TrimSpace(draft.Name), strings.TrimSpace(draft.WorkspaceRoot)); err != nil {
		if isUniqueViolation(err) {
			return Scope{}, core.Conflict
		}
		return Scope{}, err
	}
	if err = k.guard(ctx, tx, scope, nil); err != nil {
		return Scope{}, err
	}
	for _, employee := range draft.Roster {
		if _, err = tx.Exec(ctx, `INSERT INTO employees(company_id,id,display_name,role_name,model_profile,enabled)
VALUES($1,$2,$3,$4,$5,true)`, draft.ID, employee.ID, strings.TrimSpace(employee.DisplayName), strings.TrimSpace(employee.Role), strings.TrimSpace(employee.ModelProfile)); err != nil {
			return Scope{}, err
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO employee_schedules(company_id,employee_id,state)
SELECT company_id,id,'sleeping' FROM employees WHERE company_id=$1
ON CONFLICT(company_id,employee_id) DO NOTHING`, draft.ID); err != nil {
		return Scope{}, err
	}
	if err = appendEvent(ctx, tx, scope, "company.created", struct {
		Name          string `json:"name"`
		WorkspaceRoot string `json:"workspace_root"`
	}{Name: strings.TrimSpace(draft.Name), WorkspaceRoot: strings.TrimSpace(draft.WorkspaceRoot)}); err != nil {
		return Scope{}, err
	}
	if confirmationSHA256 != "" {
		if err = appendEvent(ctx, tx, scope, "company.team_coverage.confirmed", struct {
			RequestID      string `json:"request_id"`
			TemplateSHA256 string `json:"template_sha256"`
			Decision       string `json:"decision"`
			Qualification  string `json:"qualification"`
		}{RequestID: requestID, TemplateSHA256: confirmationSHA256, Decision: "installation_owner_confirmed_fixed_team_mapping", Qualification: "unverified"}); err != nil {
			return Scope{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return Scope{}, err
	}
	return scope, nil
}

// CompanyDetails reads one company and its fixed roster without taking
// controller ownership or changing runtime state.
func (k *Kernel) CompanyDetails(ctx context.Context, companyID string) (CompanyDetails, error) {
	if !core.ValidID(companyID) {
		return CompanyDetails{}, core.Malformed
	}
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return CompanyDetails{}, err
	}
	defer tx.Rollback(ctx)
	details := CompanyDetails{}
	if err = tx.QueryRow(ctx, `SELECT c.id,c.name,c.workspace_root,c.state,c.provider,c.model,c.effort,c.profile,
COALESCE(coverage.payload->>'template_sha256',''),COALESCE(coverage.payload->>'occurred_at','')
FROM companies c LEFT JOIN LATERAL (
  SELECT payload FROM events WHERE company_id=c.id AND kind='company.team_coverage.confirmed' ORDER BY company_seq DESC LIMIT 1
) coverage ON TRUE WHERE c.id=$1`, companyID).Scan(&details.ID, &details.Name, &details.WorkspaceRoot, &details.State, &details.Provider, &details.Model, &details.Effort, &details.Profile, &details.TeamCoverageConfirmationSHA256, &details.TeamCoverageConfirmedAt); errors.Is(err, pgx.ErrNoRows) {
		return CompanyDetails{}, core.OutOfScope
	} else if err != nil {
		return CompanyDetails{}, err
	}
	details.TeamCoverageConfirmed = details.TeamCoverageConfirmationSHA256 == spec.FixedTeamCoverageSHA256()
	rows, err := tx.Query(ctx, `SELECT id,display_name,role_name,model_profile FROM employees WHERE company_id=$1 AND enabled ORDER BY id`, companyID)
	if err != nil {
		return CompanyDetails{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var employee organization.EmployeeDraft
		if err = rows.Scan(&employee.ID, &employee.DisplayName, &employee.Role, &employee.ModelProfile); err != nil {
			return CompanyDetails{}, err
		}
		details.Roster = append(details.Roster, employee)
	}
	if err = rows.Err(); err != nil {
		return CompanyDetails{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return CompanyDetails{}, err
	}
	return details, nil
}

// ListCompanyDetails returns companies in stable ID order for the group
// switcher. Archived companies remain visible so users can recover context.
func (k *Kernel) ListCompanyDetails(ctx context.Context) ([]CompanyDetails, error) {
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT c.id,c.name,c.workspace_root,c.state,c.provider,c.model,c.effort,c.profile,
COALESCE(coverage.payload->>'template_sha256',''),COALESCE(coverage.payload->>'occurred_at','')
FROM companies c LEFT JOIN LATERAL (
  SELECT payload FROM events WHERE company_id=c.id AND kind='company.team_coverage.confirmed' ORDER BY company_seq DESC LIMIT 1
) coverage ON TRUE ORDER BY c.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	companies := make([]CompanyDetails, 0)
	for rows.Next() {
		var company CompanyDetails
		if err = rows.Scan(&company.ID, &company.Name, &company.WorkspaceRoot, &company.State, &company.Provider, &company.Model, &company.Effort, &company.Profile, &company.TeamCoverageConfirmationSHA256, &company.TeamCoverageConfirmedAt); err != nil {
			return nil, err
		}
		company.TeamCoverageConfirmed = company.TeamCoverageConfirmationSHA256 == spec.FixedTeamCoverageSHA256()
		companies = append(companies, company)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return companies, nil
}

// TXUpdateCompanyWithOrganization changes non-runtime organization settings
// and the fixed roster presentation in one scoped transaction.
func (k *Kernel) TXUpdateCompanyWithOrganization(ctx context.Context, draft organization.CompanyDraft, key string) (Receipt, error) {
	return k.txUpdateCompanyWithOrganization(ctx, draft, key, "")
}

// TXUpdateCompanyWithOrganizationAndCoverageConfirmation optionally records
// the installation owner's acknowledgment of the exact fixed-team draft in
// the same transaction as the organization update. It never qualifies a role.
func (k *Kernel) TXUpdateCompanyWithOrganizationAndCoverageConfirmation(ctx context.Context, draft organization.CompanyDraft, key, confirmationSHA256 string) (Receipt, error) {
	return k.txUpdateCompanyWithOrganization(ctx, draft, key, confirmationSHA256)
}

func (k *Kernel) txUpdateCompanyWithOrganization(ctx context.Context, draft organization.CompanyDraft, key, confirmationSHA256 string) (Receipt, error) {
	if err := organization.ValidateCompanyDraft(draft); err != nil {
		return Receipt{}, core.Malformed
	}
	if confirmationSHA256 != "" && (confirmationSHA256 != spec.FixedTeamCoverageSHA256() || !core.ValidID(key)) {
		return Receipt{}, core.Malformed
	}
	if confirmationSHA256 != "" {
		return k.TXWrite(ctx, Scope{company: draft.ID}, nil, key, "company.team_coverage.confirm", struct {
			TemplateSHA256 string
		}{TemplateSHA256: confirmationSHA256}, func(tx pgx.Tx) (Receipt, error) {
			var state string
			if err := tx.QueryRow(ctx, "SELECT state FROM companies WHERE id=$1", draft.ID).Scan(&state); errors.Is(err, pgx.ErrNoRows) {
				return Receipt{}, core.OutOfScope
			} else if err != nil {
				return Receipt{}, err
			} else if state == "archived" {
				return Receipt{}, core.Denied
			}
			if err := appendEvent(ctx, tx, Scope{company: draft.ID}, "company.team_coverage.confirmed", struct {
				RequestID      string `json:"request_id"`
				TemplateSHA256 string `json:"template_sha256"`
				Decision       string `json:"decision"`
				Qualification  string `json:"qualification"`
			}{RequestID: key, TemplateSHA256: confirmationSHA256, Decision: "installation_owner_confirmed_fixed_team_mapping", Qualification: "unverified"}); err != nil {
				return Receipt{}, err
			}
			return Receipt{ID: draft.ID, Status: "active"}, nil
		})
	}
	return k.TXWrite(ctx, Scope{company: draft.ID}, nil, key, "company.update", draft, func(tx pgx.Tx) (Receipt, error) {
		var state string
		if err := tx.QueryRow(ctx, "SELECT state FROM companies WHERE id=$1", draft.ID).Scan(&state); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		} else if state == "archived" {
			return Receipt{}, core.Denied
		}
		if err := ensureConfirmedTeamCoverageRolesUnchangedTX(ctx, tx, draft); err != nil {
			return Receipt{}, err
		}
		if _, err := tx.Exec(ctx, "UPDATE companies SET name=$2,workspace_root=$3 WHERE id=$1", draft.ID, strings.TrimSpace(draft.Name), strings.TrimSpace(draft.WorkspaceRoot)); err != nil {
			return Receipt{}, err
		}
		for _, employee := range draft.Roster {
			if _, err := tx.Exec(ctx, `UPDATE employees SET display_name=$3,role_name=$4,model_profile=$5
WHERE company_id=$1 AND id=$2 AND enabled`, draft.ID, employee.ID, strings.TrimSpace(employee.DisplayName), strings.TrimSpace(employee.Role), strings.TrimSpace(employee.ModelProfile)); err != nil {
				return Receipt{}, err
			}
		}
		return Receipt{ID: draft.ID, Status: "active"}, nil
	})
}

// ensureConfirmedTeamCoverageRolesUnchangedTX enforces the template's
// role_changes_at_runtime=false contract after the current fixed-team matrix
// has been acknowledged. Display names and model profiles remain editable.
func ensureConfirmedTeamCoverageRolesUnchangedTX(ctx context.Context, tx pgx.Tx, draft organization.CompanyDraft) error {
	var confirmationSHA256 string
	err := tx.QueryRow(ctx, `SELECT payload->>'template_sha256' FROM events
WHERE company_id=$1 AND kind='company.team_coverage.confirmed'
ORDER BY company_seq DESC LIMIT 1`, draft.ID).Scan(&confirmationSHA256)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && confirmationSHA256 != spec.FixedTeamCoverageSHA256()) {
		return nil
	}
	if err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT id,role_name FROM employees WHERE company_id=$1 AND enabled ORDER BY id`, draft.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	roles := make(map[string]string, len(draft.Roster))
	for rows.Next() {
		var employeeID, role string
		if err = rows.Scan(&employeeID, &role); err != nil {
			return err
		}
		roles[employeeID] = role
	}
	if err = rows.Err(); err != nil {
		return err
	}
	if len(roles) != len(draft.Roster) {
		return core.Integrity
	}
	for _, employee := range draft.Roster {
		role, ok := roles[employee.ID]
		if !ok {
			return core.Integrity
		}
		if strings.TrimSpace(role) != strings.TrimSpace(employee.Role) {
			return core.Denied
		}
	}
	return nil
}

// TXArchiveCompany archives an idle company without deleting its history or
// artifacts. Active work must be stopped through the normal command path first.
func (k *Kernel) TXArchiveCompany(ctx context.Context, companyID, key string) (Receipt, error) {
	if !core.ValidID(companyID) {
		return Receipt{}, core.Malformed
	}
	return k.TXWrite(ctx, Scope{company: companyID}, nil, key, "company.archive", companyID, func(tx pgx.Tx) (Receipt, error) {
		var state string
		if err := tx.QueryRow(ctx, "SELECT state FROM companies WHERE id=$1", companyID).Scan(&state); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if state == "archived" {
			return Receipt{ID: companyID, Status: "archived"}, nil
		}
		var activeMission bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM missions WHERE company_id=$1 AND state IN ('active','paused','closing'))", companyID).Scan(&activeMission); err != nil {
			return Receipt{}, err
		}
		if activeMission {
			return Receipt{}, core.ConflictError{Reason: "company has an active mission", CurrentState: "active"}
		}
		if _, err := tx.Exec(ctx, "UPDATE companies SET state='archived' WHERE id=$1", companyID); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: companyID, Status: "archived"}, nil
	})
}

// TXUpdateRuntimeSettings persists desired non-secret runtime configuration.
// The active WorkerAdapter is not restarted implicitly; callers observe the
// resulting restart_required state through the settings projection.
func (k *Kernel) TXUpdateRuntimeSettings(ctx context.Context, companyID, providerName, model, effort, profile, key string) (Receipt, error) {
	if !core.ValidID(companyID) || strings.TrimSpace(providerName) == "" || strings.TrimSpace(model) == "" || strings.TrimSpace(effort) == "" || strings.TrimSpace(profile) == "" {
		return Receipt{}, core.Malformed
	}
	return k.TXWrite(ctx, Scope{company: companyID}, nil, key, "runtime.settings.update", struct {
		Provider string
		Model    string
		Effort   string
		Profile  string
	}{providerName, model, effort, profile}, func(tx pgx.Tx) (Receipt, error) {
		var state string
		if err := tx.QueryRow(ctx, "SELECT state FROM companies WHERE id=$1", companyID).Scan(&state); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		} else if state == "archived" {
			return Receipt{}, core.Denied
		}
		if _, err := tx.Exec(ctx, `UPDATE companies SET provider=$2,model=$3,effort=$4,profile=$5 WHERE id=$1`, companyID, strings.TrimSpace(providerName), strings.TrimSpace(model), strings.TrimSpace(effort), strings.TrimSpace(profile)); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: companyID, Status: "restart_required"}, nil
	})
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
