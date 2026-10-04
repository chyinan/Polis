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
)

// CompanyDetails is the authorized organization projection used by product
// settings and the company switcher. It contains no provider credentials.
type CompanyDetails struct {
	ID            string
	Name          string
	WorkspaceRoot string
	State         string
	Provider      string
	Model         string
	Effort        string
	Profile       string
	Roster        []organization.EmployeeDraft
}

// TXCreateCompanyWithOrganization creates the fixed logical runtime roster in
// one transaction. It does not start a Mission or provider session.
func (k *Kernel) TXCreateCompanyWithOrganization(ctx context.Context, draft organization.CompanyDraft) (Scope, error) {
	if err := organization.ValidateCompanyDraft(draft); err != nil {
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
	if err = tx.QueryRow(ctx, `SELECT id,name,workspace_root,state,provider,model,effort,profile FROM companies WHERE id=$1`, companyID).Scan(&details.ID, &details.Name, &details.WorkspaceRoot, &details.State, &details.Provider, &details.Model, &details.Effort, &details.Profile); errors.Is(err, pgx.ErrNoRows) {
		return CompanyDetails{}, core.OutOfScope
	} else if err != nil {
		return CompanyDetails{}, err
	}
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
	rows, err := tx.Query(ctx, `SELECT id,name,workspace_root,state,provider,model,effort,profile FROM companies ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	companies := make([]CompanyDetails, 0)
	for rows.Next() {
		var company CompanyDetails
		if err = rows.Scan(&company.ID, &company.Name, &company.WorkspaceRoot, &company.State, &company.Provider, &company.Model, &company.Effort, &company.Profile); err != nil {
			return nil, err
		}
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
	if err := organization.ValidateCompanyDraft(draft); err != nil {
		return Receipt{}, core.Malformed
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
