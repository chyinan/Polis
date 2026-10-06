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
