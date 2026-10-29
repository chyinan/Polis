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

func TestFixedTeamTaskRevisionFailsClosedForMissingStaleRevokedOrMutatedRoleRevision(t *testing.T) {
	compiled, err := CompileFixedTeamCoverageRoleRevision(FixedTeamCoverageDraftSnapshot(), FixedTeamCoverageSHA256(), FixedTeamCoverageOwnerDecision, "unverified")
	if err != nil {
		t.Fatal(err)
	}
	stale := compiled
	stale.RevisionSHA256 = "0000000000000000000000000000000000000000000000000000000000000000"
	stale.Qualification = "qualified"
	revoked := compiled
	revoked.OwnerDecision = "revoked"
	revoked.Qualification = "qualified"
	mutated := compiled
	for index := range mutated.Contract.Coverage {
		if mutated.Contract.Coverage[index].TaskType == "backend" {
			mutated.Contract.Coverage[index].Owner = "emp-frontend"
		}
	}

	for _, test := range []struct {
		name     string
		revision FixedTeamCoverageRoleRevision
		taskType string
		owner    string
	}{
		{name: "missing role revision", revision: FixedTeamCoverageRoleRevision{}, taskType: "backend", owner: "emp-backend"},
		{name: "stale role digest", revision: stale, taskType: "backend", owner: "emp-backend"},
		{name: "revoked role decision", revision: revoked, taskType: "backend", owner: "emp-backend"},
		{name: "mutated compiled role contract", revision: mutated, taskType: "backend", owner: "emp-frontend"},
	} {
		t.Run(test.name, func(t *testing.T) {
			binding := test.revision.TaskRevisionFor(test.taskType, test.owner, "compat")
			if binding.ReasonCode != "role_revision_unavailable" || binding.Qualification != "unverified" || !binding.RequiresHuman {
				t.Fatalf("invalid RoleRevision produced TaskRevision=%+v, want unavailable/unverified/human-gated", binding)
			}
			admission := test.revision.AdmissionFor(test.taskType, test.owner)
			if admission.Covered || !admission.RequiresHuman {
				t.Fatalf("invalid RoleRevision admission=%+v, want uncovered human path", admission)
			}
		})
	}
}

func TestCompileFixedTeamCoverageRoleRevisionRejectsRevokedOrQualifiedRows(t *testing.T) {
	for _, test := range []struct {
		name          string
		ownerDecision string
		qualification string
	}{
		{name: "revoked owner decision", ownerDecision: "revoked", qualification: "unverified"},
		{name: "qualified without qualification evidence", ownerDecision: FixedTeamCoverageOwnerDecision, qualification: "qualified"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := CompileFixedTeamCoverageRoleRevision(FixedTeamCoverageDraftSnapshot(), FixedTeamCoverageSHA256(), test.ownerDecision, test.qualification); err == nil {
				t.Fatalf("compiled invalid RoleRevision decision=%q qualification=%q", test.ownerDecision, test.qualification)
			}
		})
	}
}
