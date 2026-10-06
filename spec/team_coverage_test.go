// pattern: Functional Core
package spec

import "testing"

func TestFixedTeamCoverageTaskKindMappingIsExplicitButStillHumanGated(t *testing.T) {
	want := map[string]string{
		"planning": "bootstrap_plan", "frontend": "peer_frontend", "backend": "compat",
		"environment_plan": "compute", "regression_test_overlay": "review", "design_change": "bootstrap_plan", "delivery_assembly": "compute",
	}
	revision, err := CompileFixedTeamCoverageRoleRevision(FixedTeamCoverageDraftSnapshot(), FixedTeamCoverageSHA256(), FixedTeamCoverageOwnerDecision, "unverified")
	if err != nil {
		t.Fatal(err)
	}
	for taskType, taskKind := range want {
		if got := FixedTeamCoverageTaskKind(taskType); got != taskKind {
			t.Fatalf("task type %q maps to %q, want %q", taskType, got, taskKind)
		}
		admission := revision.AdmissionFor(taskType, "emp-"+taskType)
		if admission.TaskKind != taskKind || !admission.RequiresHuman {
			t.Fatalf("admission for %q=%+v, want kind %q and human gate", taskType, admission, taskKind)
		}
	}
	if FixedTeamCoverageTaskKind("unknown") != "" {
		t.Fatal("unknown semantic task type received a runtime kind")
	}
}

func TestFixedTeamTaskRevisionBindingIsContentAddressedAndHumanGated(t *testing.T) {
	revision, err := CompileFixedTeamCoverageRoleRevision(FixedTeamCoverageDraftSnapshot(), FixedTeamCoverageSHA256(), FixedTeamCoverageOwnerDecision, "unverified")
	if err != nil {
		t.Fatal(err)
	}
	binding := revision.TaskRevisionFor("backend", "emp-backend", "compat")
	if binding.RoleRevisionSHA256 != revision.RevisionSHA256 || binding.TaskType != "backend" || binding.TaskKind != "compat" || binding.Owner != "emp-backend" {
		t.Fatalf("exact task revision binding=%+v", binding)
	}
	if binding.Qualification != "unverified" || !binding.RequiresHuman || binding.ReasonCode != "task_revision_unqualified" || len(binding.RevisionSHA256) != 64 {
		t.Fatalf("exact task revision must remain human-gated and content-addressed: %+v", binding)
	}
	if again := revision.TaskRevisionFor("backend", "emp-backend", "compat"); again.RevisionSHA256 != binding.RevisionSHA256 {
		t.Fatalf("same task revision binding changed across calls: %q vs %q", binding.RevisionSHA256, again.RevisionSHA256)
	}
}

func TestFixedTeamTaskRevisionBindingRejectsSemanticMismatches(t *testing.T) {
	revision, err := CompileFixedTeamCoverageRoleRevision(FixedTeamCoverageDraftSnapshot(), FixedTeamCoverageSHA256(), FixedTeamCoverageOwnerDecision, "unverified")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, taskType, owner, taskKind, reason string
	}{
		{name: "owner mismatch", taskType: "backend", owner: "emp-frontend", taskKind: "compat", reason: "task_owner_mismatch"},
		{name: "kind mismatch", taskType: "backend", owner: "emp-backend", taskKind: "compute", reason: "task_kind_mismatch"},
		{name: "uncovered type", taskType: "unknown", owner: "emp-backend", taskKind: "compat", reason: "task_type_uncovered"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			binding := revision.TaskRevisionFor(test.taskType, test.owner, test.taskKind)
			if binding.ReasonCode != test.reason || !binding.RequiresHuman || len(binding.RevisionSHA256) != 64 {
				t.Fatalf("binding=%+v, want reason=%q and human gate", binding, test.reason)
			}
		})
	}
}
