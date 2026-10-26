package organization

import (
	"errors"
	"testing"
)

func TestBuildFixedTeamRoleReconciliationPreviewBindsExactLegacyDrift(t *testing.T) {
	roster := DefaultRoster()
	roster[1].Role = "legacy-backend"
	preview, err := BuildFixedTeamRoleReconciliationPreview("company-1", "active", roster, false)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Eligible || preview.ReasonCode != "owner_reconciliation_required" || len(preview.Mismatches) != 1 ||
		preview.Mismatches[0] != (FixedTeamRoleMismatch{EmployeeID: EmployeeBackend, CurrentRole: "legacy-backend", ExpectedRole: "backend"}) {
		t.Fatalf("legacy role drift preview=%+v", preview)
	}
	reordered := []EmployeeDraft{roster[3], roster[1], roster[0], roster[2]}
	reorderedPreview, err := BuildFixedTeamRoleReconciliationPreview("company-1", "active", reordered, false)
	if err != nil || reorderedPreview.SnapshotSHA256 != preview.SnapshotSHA256 {
		t.Fatalf("roster ordering changed canonical digest: original=%+v reordered=%+v err=%v", preview, reorderedPreview, err)
	}
	changed := append([]EmployeeDraft(nil), roster...)
	changed[1].Role = "another-legacy-role"
	changedPreview, err := BuildFixedTeamRoleReconciliationPreview("company-1", "active", changed, false)
	if err != nil || changedPreview.SnapshotSHA256 == preview.SnapshotSHA256 {
		t.Fatalf("role drift change did not alter CAS digest: original=%s changed=%s err=%v", preview.SnapshotSHA256, changedPreview.SnapshotSHA256, err)
	}
}

func TestBuildFixedTeamRoleReconciliationPreviewBlocksUnsafeOrUnneededRepair(t *testing.T) {
	roster := DefaultRoster()
	roster[0].Role = "legacy-planner"
	for _, test := range []struct {
		name         string
		state        string
		activeWork   bool
		wantEligible bool
		wantReason   string
	}{
		{name: "idle active company", state: "active", wantEligible: true, wantReason: "owner_reconciliation_required"},
		{name: "active mission", state: "active", activeWork: true, wantReason: "active_work_present"},
		{name: "archived company", state: "archived", wantReason: "company_not_active"},
	} {
		t.Run(test.name, func(t *testing.T) {
			preview, err := BuildFixedTeamRoleReconciliationPreview("company-1", test.state, roster, test.activeWork)
			if err != nil || preview.Eligible != test.wantEligible || preview.ReasonCode != test.wantReason {
				t.Fatalf("preview=%+v err=%v", preview, err)
			}
		})
	}
	preview, err := BuildFixedTeamRoleReconciliationPreview("company-1", "active", DefaultRoster(), false)
	if err != nil || preview.Eligible || preview.ReasonCode != "no_role_drift" {
		t.Fatalf("canonical roster preview=%+v err=%v", preview, err)
	}
	if _, err = BuildFixedTeamRoleReconciliationPreview("company-1", "active", roster[:3], false); !errors.Is(err, errFixedRoleReconciliation) {
		t.Fatalf("incomplete roster error=%v", err)
	}
}
