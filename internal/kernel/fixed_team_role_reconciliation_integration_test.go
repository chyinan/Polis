// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"polis/internal/core"
	"polis/internal/pgfixture"
)

func TestFixedTeamRoleReconciliationUsesOwnerSnapshotAndPreservesUnverifiedQualification(t *testing.T) {
	dsn := pgfixture.DSN(t, "POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	k, err := Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	companyID := fmt.Sprintf("fixed-role-reconcile-%d", time.Now().UnixNano())
	_, err = createPGTestCompany(k, ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.pool.Exec(ctx, `UPDATE employees SET role_name='legacy-backend' WHERE company_id=$1 AND id='emp-backend'`, companyID); err != nil {
		t.Fatal(err)
	}
	before, err := k.CompanyDetails(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	preview := before.FixedTeamRoleReconciliation
	if !preview.Eligible || preview.ReasonCode != "owner_reconciliation_required" || len(preview.Mismatches) != 1 || preview.Mismatches[0].EmployeeID != core.EmployeeBackendID {
		t.Fatalf("fixed-role drift preview=%+v", preview)
	}
	if _, err = k.pool.Exec(ctx, `UPDATE employees SET role_name='legacy-backend-v2' WHERE company_id=$1 AND id='emp-backend'`, companyID); err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXReconcileFixedTeamRoleAssignments(ctx, companyID, FixedTeamRoleReconciliationInput{
		ExpectedSnapshotSHA256: preview.SnapshotSHA256,
		Rationale:              "owner reviewed the current backend role drift",
		RequestID:              "fixed-role-reconcile-stale-snapshot",
	}); !errors.Is(err, core.Conflict) {
		t.Fatalf("stale reconciliation snapshot error=%v, want conflict", err)
	}
	current, err := k.CompanyDetails(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	if current.FixedTeamRoleReconciliation.SnapshotSHA256 == preview.SnapshotSHA256 {
		t.Fatal("changed legacy role did not alter the preview snapshot digest")
	}
	receipt, err := k.TXReconcileFixedTeamRoleAssignments(ctx, companyID, FixedTeamRoleReconciliationInput{
		ExpectedSnapshotSHA256: current.FixedTeamRoleReconciliation.SnapshotSHA256,
		Rationale:              "Owner reviewed the legacy backend role and confirms the canonical fixed mapping.",
		RequestID:              "fixed-role-reconcile-exact-snapshot",
	})
	if err != nil || receipt.Status != "reconciled" {
		t.Fatalf("reconciliation receipt=%+v err=%v", receipt, err)
	}
	after, err := k.CompanyDetails(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	if !after.TeamCoverageConfirmed || after.FixedTeamRoleReconciliation.ReasonCode != "no_role_drift" || len(after.FixedTeamRoleReconciliation.Mismatches) != 0 {
		t.Fatalf("reconciled Company projection=%+v", after)
	}
	if after.TeamCoverageRoleRevision == nil || after.TeamCoverageRoleRevision.Qualification != "unverified" {
		t.Fatalf("role reconciliation altered or omitted qualification state: %+v", after.TeamCoverageRoleRevision)
	}
	var auditCount int
	if err = k.pool.QueryRow(ctx, `SELECT count(*) FROM events WHERE company_id=$1 AND kind='company.team_roles.reconciled'`, companyID).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 {
		t.Fatalf("owner reconciliation audit event count=%d, want 1", auditCount)
	}
	if _, err = k.TXReconcileFixedTeamRoleAssignments(ctx, companyID, FixedTeamRoleReconciliationInput{
		ExpectedSnapshotSHA256: after.FixedTeamRoleReconciliation.SnapshotSHA256,
		Rationale:              "there is no remaining role drift",
		RequestID:              "fixed-role-reconcile-noop",
	}); !errors.Is(err, core.Conflict) {
		t.Fatalf("no-op role reconciliation error=%v, want conflict", err)
	}
}

func TestFixedTeamRoleReconciliationBlocksActiveMission(t *testing.T) {
	dsn := pgfixture.DSN(t, "POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	k, err := Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	companyID := fmt.Sprintf("fixed-role-reconcile-active-%d", time.Now().UnixNano())
	scope, err := createPGTestCompany(k, ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	missionID := "fixed-role-reconcile-mission-" + fmt.Sprint(time.Now().UnixNano())
	if err = createPGTestMission(k, ctx, scope, missionID); err != nil {
		t.Fatal(err)
	}
	if _, err = k.pool.Exec(ctx, `UPDATE missions SET state='active',activation_id=$3 WHERE company_id=$1 AND id=$2`, companyID, missionID, "fixed-role-active"); err != nil {
		t.Fatal(err)
	}
	if _, err = k.pool.Exec(ctx, `UPDATE employees SET role_name='legacy-planner' WHERE company_id=$1 AND id='emp-planning'`, companyID); err != nil {
		t.Fatal(err)
	}
	company, err := k.CompanyDetails(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	if company.FixedTeamRoleReconciliation.Eligible || company.FixedTeamRoleReconciliation.ReasonCode != "active_work_present" {
		t.Fatalf("active Mission did not block reconciliation preview: %+v", company.FixedTeamRoleReconciliation)
	}
	if _, err = k.TXReconcileFixedTeamRoleAssignments(ctx, companyID, FixedTeamRoleReconciliationInput{
		ExpectedSnapshotSHA256: company.FixedTeamRoleReconciliation.SnapshotSHA256,
		Rationale:              "attempt should be blocked while active work exists",
		RequestID:              "fixed-role-reconcile-active-blocked",
	}); !errors.Is(err, core.Conflict) {
		t.Fatalf("active Mission reconciliation error=%v, want conflict", err)
	}
}
