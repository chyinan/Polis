package taskvalidation

import (
	"strings"
	"testing"
)

func TestBindResolvesPublicTaskPlaceholdersAndIsStable(t *testing.T) {
	contract := AcceptanceContract{
		Revision: AcceptanceContractRevision,
		RequiredText: []string{
			"Mission ID: {{mission_id}}",
			"Task ID: {{task_id}}",
			"Acknowledgement:",
		},
	}

	first, err := Bind("task-1", "mission-1", &contract)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Bind("task-1", "mission-1", &contract)
	if err != nil {
		t.Fatal(err)
	}
	if first.ConfigurationDigest == "" || first.ConfigurationDigest != second.ConfigurationDigest {
		t.Fatalf("binding digest is not stable: %q != %q", first.ConfigurationDigest, second.ConfigurationDigest)
	}
	if first.Contract.RequiredText[0] != "Mission ID: mission-1" || first.Contract.RequiredText[1] != "Task ID: task-1" {
		t.Fatalf("placeholders were not bound to task identity: %#v", first.Contract.RequiredText)
	}
	if contract.RequiredText[0] != "Mission ID: {{mission_id}}" {
		t.Fatal("Bind mutated the public Mission contract")
	}
	otherTask, err := Bind("task-2", "mission-1", &contract)
	if err != nil {
		t.Fatal(err)
	}
	if otherTask.ConfigurationDigest == first.ConfigurationDigest {
		t.Fatal("task identity must be included in the frozen validator configuration")
	}
}

func TestBindRejectsUnknownAndMalformedPlaceholders(t *testing.T) {
	for _, value := range []string{"", "{{employee_id}}", "prefix {{mission_id", "{{task_id}} suffix }}"} {
		_, err := Bind("task-1", "mission-1", &AcceptanceContract{
			Revision:     AcceptanceContractRevision,
			RequiredText: []string{value},
		})
		if err == nil {
			t.Errorf("Bind(%q) unexpectedly succeeded", value)
		}
	}
}

func TestValidateDistinguishesMissingPassFailAndUnavailable(t *testing.T) {
	registry := DefaultRegistry()
	missing := registry.Validate(nil, "anything")
	if missing.Status != StatusNotConfigured || missing.ReasonCode != "validation_not_configured" {
		t.Fatalf("missing binding result = %#v", missing)
	}

	binding, err := Bind("task-1", "mission-1", &AcceptanceContract{
		Revision:     AcceptanceContractRevision,
		RequiredText: []string{"Mission ID: {{mission_id}}", "Acknowledgement:"},
	})
	if err != nil {
		t.Fatal(err)
	}
	passed := registry.Validate(binding, "Mission ID: mission-1\nAcknowledgement: received")
	if passed.Status != StatusPass || len(passed.Findings) != 0 {
		t.Fatalf("pass result = %#v", passed)
	}
	failed := registry.Validate(binding, "Acknowledgement: received")
	if failed.Status != StatusFail || failed.ReasonCode != "required_text_missing" || len(failed.Findings) != 1 {
		t.Fatalf("fail result = %#v", failed)
	}
	if !strings.Contains(failed.Findings[0].Feedback, "Mission ID: mission-1") {
		t.Fatalf("feedback is not actionable/public: %#v", failed.Findings)
	}

	unknown := *binding
	unknown.RunnerKind = "unregistered"
	unavailable := registry.Validate(&unknown, "anything")
	if unavailable.Status != StatusValidatorUnavailable || unavailable.ReasonCode != "validator_unavailable" {
		t.Fatalf("unavailable result = %#v", unavailable)
	}
}

func TestValidateRejectsCorruptConfigurationDigest(t *testing.T) {
	binding, err := Bind("task-1", "mission-1", &AcceptanceContract{
		Revision:     AcceptanceContractRevision,
		RequiredText: []string{"ok"},
	})
	if err != nil {
		t.Fatal(err)
	}
	binding.ConfigurationDigest = strings.Repeat("0", 64)
	result := DefaultRegistry().Validate(binding, "ok")
	if result.Status != StatusInfrastructureError || result.ReasonCode != "binding_integrity_error" {
		t.Fatalf("corrupt binding result = %#v", result)
	}
}
