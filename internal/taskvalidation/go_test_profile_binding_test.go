// pattern: Functional Core
package taskvalidation

import "testing"

func TestTaskValidationBindingPinsOfflineGoTestProfile(t *testing.T) {
	contract := AcceptanceContract{
		Revision:              AcceptanceContractGoTestRevision,
		RequiredText:          []string{"Mission ID: {{mission_id}}", "Task ID: {{task_id}}", "Task summary:"},
		GoTestProfileRevision: GoTestFixtureProfileRevision,
	}
	binding, err := Bind("task-1", "mission-1", &contract)
	if err != nil {
		t.Fatal(err)
	}
	if binding.Contract.GoTestProfileRevision != GoTestFixtureProfileRevision || binding.ConfigurationDigest == "" {
		t.Fatalf("Go profile was not included in the immutable Task binding: %+v", binding)
	}

	changed := contract
	changed.GoTestProfileRevision = "unqualified-profile@1"
	if _, err = Bind("task-1", "mission-1", &changed); err == nil {
		t.Fatal("unsupported Go profile was accepted into TaskValidationBinding")
	}
}

func TestNewGoBindingSelectsExplicitGoRunnerWithoutTextFallback(t *testing.T) {
	b, err := Bind("task", "mission", &AcceptanceContract{Revision: AcceptanceContractGoTestRevision, GoTestProfileRevision: GoTestFixtureProfileRevision, RequiredText: []string{"package formatter"}})
	if err != nil {
		t.Fatal(err)
	}
	if b.RunnerKind != "fixed_go_test" || b.RunnerRevision != GoTestFixtureProfileRevision {
		t.Fatalf("Go binding retained text-only runner: %+v", b)
	}
	if result := DefaultRegistry().Validate(b, "package formatter"); result.Status == StatusPass {
		t.Fatal("a Go binding passed without execution evidence")
	}
}

func TestGoTestAcceptanceBindingCannotPassThroughTextOnlyValidator(t *testing.T) {
	contract := AcceptanceContract{
		Revision:              AcceptanceContractGoTestRevision,
		GoTestProfileRevision: GoTestFixtureProfileRevision,
		RequiredText:          []string{"the imported commit remains immutable"},
	}
	binding, err := Bind("task-1", "mission-1", &contract)
	if err != nil {
		t.Fatal(err)
	}
	result := DefaultRegistry().Validate(binding, contract.RequiredText[0])
	if result.Status != StatusValidatorUnavailable || result.ReasonCode != "validator_unavailable" {
		t.Fatalf("Go-profile task incorrectly passed through text-only acceptance: %+v", result)
	}
}
