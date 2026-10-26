// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"polis/internal/core"
	"polis/internal/pgfixture"
)

func TestTaskSemanticRevisionOwnerDecisionPersistsWithoutQualifyingExecution(t *testing.T) {
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
	companyID := fmt.Sprintf("task-semantic-owner-decision-%d", time.Now().UnixNano())
	scope, err := createPGTestCompany(k, ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	task, err := createPGTestProbe(k, ctx, scope, "task-semantic-owner-decision-mission-"+fmt.Sprint(time.Now().UnixNano()))
	if err != nil {
		t.Fatal(err)
	}
	if task.Owner != core.EmployeeBackendID || task.Kind != core.TaskKindCompat {
		t.Fatalf("test fixture Task=%+v, want exact fixed Backend compat assignment", task)
	}
	var roleRevisionSHA256 string
	if err = k.pool.QueryRow(ctx, `SELECT revision_sha256 FROM employee_role_revisions WHERE company_id=$1 AND employee_id=$2 ORDER BY created_at DESC LIMIT 1`, companyID, core.EmployeeBackendID).Scan(&roleRevisionSHA256); err != nil {
		t.Fatal(err)
	}
	bindingSHA256 := strings.Repeat("b", 64)
	if _, err = k.pool.Exec(ctx, `INSERT INTO task_semantic_revisions(company_id,task_id,revision,binding_sha256,role_revision_sha256,task_type,task_kind,owner_employee_id,qualification,requires_human,reason_code)
VALUES($1,$2,1,$3,$4,'backend','compat',$5,'unverified',true,'task_revision_unqualified')`, companyID, task.ID, bindingSHA256, roleRevisionSHA256, core.EmployeeBackendID); err != nil {
		t.Fatal(err)
	}
	input := TaskSemanticRevisionDecisionInput{
		TaskID: task.ID, Revision: 1, BindingSHA256: bindingSHA256,
		Decision: "approved", Rationale: "Owner reviewed the exact descriptive TaskRevision mapping.",
		RequestID: "task-semantic-owner-approve",
	}
	approved, err := k.TXDecideTaskSemanticRevision(ctx, companyID, input)
	if err != nil || approved.Status != "approved" || approved.Revision != 1 {
		t.Fatalf("approval receipt=%+v err=%v", approved, err)
	}
	var qualification string
	var requiresHuman bool
	if err = k.pool.QueryRow(ctx, `SELECT qualification,requires_human FROM task_semantic_revisions WHERE company_id=$1 AND task_id=$2 AND revision=1`, companyID, task.ID).Scan(&qualification, &requiresHuman); err != nil {
		t.Fatal(err)
	}
	if qualification != "unverified" || !requiresHuman {
		t.Fatalf("owner approval changed semantic qualification or human requirement: qualification=%s requiresHuman=%t", qualification, requiresHuman)
	}
	input.Decision = "revoked"
	input.Rationale = "Revoking the descriptive approval after review."
	input.RequestID = "task-semantic-owner-revoke"
	revoked, err := k.TXDecideTaskSemanticRevision(ctx, companyID, input)
	if err != nil || revoked.Status != "revoked" {
		t.Fatalf("revocation receipt=%+v err=%v", revoked, err)
	}
	input.Decision = "approved"
	input.RequestID = "task-semantic-owner-resurrect"
	if _, err = k.TXDecideTaskSemanticRevision(ctx, companyID, input); !errors.Is(err, core.Conflict) {
		t.Fatalf("revoked TaskRevision decision was resurrected: %v", err)
	}
	var decisionCount int
	if err = k.pool.QueryRow(ctx, `SELECT count(*) FROM task_semantic_revision_decisions WHERE company_id=$1 AND task_id=$2 AND revision=1`, companyID, task.ID).Scan(&decisionCount); err != nil {
		t.Fatal(err)
	}
	if decisionCount != 2 {
		t.Fatalf("owner decision event count=%d, want approve+revoke only", decisionCount)
	}
}
