// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
	"polis/spec"
)

// persistProductTaskSemanticRevisionTX records the semantic binding for the
// current product Task when Schema122 is available. The existing Schema121
// fixed-team admission gate still applies; this helper only skips the optional
// Schema122 write when that newer table is absent.
func persistProductTaskSemanticRevisionTX(ctx context.Context, tx pgx.Tx, companyID string, task Task) error {
	var available bool
	if err := tx.QueryRow(ctx, "SELECT to_regclass('public.task_semantic_revisions') IS NOT NULL").Scan(&available); err != nil {
		return err
	}
	if !available {
		return nil
	}
	if task.Kind != core.TaskKindCompat || task.Owner != core.EmployeeBackendID || !core.ValidID(companyID) || !core.ValidID(task.ID) {
		return nil
	}
	var roleRevisionSHA256, ownerDecision, qualification string
	err := tx.QueryRow(ctx, `SELECT revision_sha256,owner_decision,qualification
FROM employee_role_revisions
WHERE company_id=$1 AND employee_id=$2
ORDER BY created_at DESC,revision_sha256 DESC LIMIT 1`, companyID, task.Owner).Scan(&roleRevisionSHA256, &ownerDecision, &qualification)
	if errors.Is(err, pgx.ErrNoRows) {
		return core.Denied
	}
	if err != nil {
		return err
	}
	roleRevision, err := spec.CompileFixedTeamCoverageRoleRevision(spec.FixedTeamCoverageDraftSnapshot(), roleRevisionSHA256, ownerDecision, qualification)
	if err != nil {
		return core.Integrity
	}
	binding := roleRevision.TaskRevisionFor("backend", task.Owner, string(task.Kind))
	if binding.ReasonCode != "task_revision_unqualified" || !binding.RequiresHuman || binding.Qualification != "unverified" {
		return core.Denied
	}
	if _, err = tx.Exec(ctx, `INSERT INTO task_semantic_revisions(
company_id,task_id,revision,binding_sha256,role_revision_sha256,task_type,task_kind,owner_employee_id,qualification,requires_human,reason_code)
VALUES($1,$2,1,$3,$4,$5,$6,$7,$8,$9,$10)
ON CONFLICT(company_id,task_id,revision) DO NOTHING`, companyID, task.ID, binding.RevisionSHA256, binding.RoleRevisionSHA256, binding.TaskType, binding.TaskKind, binding.Owner, binding.Qualification, binding.RequiresHuman, binding.ReasonCode); err != nil {
		return err
	}
	var storedBinding, storedRole, storedType, storedKind, storedOwner, storedQualification, storedReason string
	var storedHuman bool
	if err = tx.QueryRow(ctx, `SELECT binding_sha256,role_revision_sha256,task_type,task_kind,owner_employee_id,qualification,requires_human,reason_code
FROM task_semantic_revisions WHERE company_id=$1 AND task_id=$2 AND revision=1`, companyID, task.ID).Scan(&storedBinding, &storedRole, &storedType, &storedKind, &storedOwner, &storedQualification, &storedHuman, &storedReason); err != nil {
		return err
	}
	if storedBinding != binding.RevisionSHA256 || storedRole != binding.RoleRevisionSHA256 || storedType != binding.TaskType || storedKind != binding.TaskKind || storedOwner != binding.Owner || storedQualification != binding.Qualification || !storedHuman || storedReason != binding.ReasonCode {
		return core.Integrity
	}
	return nil
}
