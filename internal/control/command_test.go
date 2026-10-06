// pattern: Functional Core
package control

import (
	"testing"

	"polis/internal/core"
	"polis/internal/taskvalidation"
)

func TestMissionCommandValidationRejectsMissingHumanInputs(t *testing.T) {
	cases := []CreateMissionRequest{
		{Title: "", Goal: "goal", RequestID: "request-1"},
		{Title: "title", Goal: "", RequestID: "request-1"},
		{Title: "title", Goal: "goal", RequestID: ""},
	}
	for _, request := range cases {
		if err := validateCreateMissionRequest(request); err != core.Malformed {
			t.Fatalf("request %+v returned %v, want %s", request, err, core.Malformed)
		}
	}
}

func TestMissionCommandValidationAcceptsBoundedInputs(t *testing.T) {
	request := CreateMissionRequest{Title: "title", Goal: "goal", ProtocolToolCallLimit: 8, RequestID: "request-1"}
	if err := validateCreateMissionRequest(request); err != nil {
		t.Fatalf("valid request returned %v", err)
	}
	request.AcceptanceContract = &taskvalidation.AcceptanceContract{Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{"Mission ID: {{mission_id}}"}}
	if err := validateCreateMissionRequest(request); err != nil {
		t.Fatalf("valid public acceptance contract returned %v", err)
	}
	request.AcceptanceContract.RequiredText[0] = "Mission ID: {{company_id}}"
	if err := validateCreateMissionRequest(request); err != core.Malformed {
		t.Fatalf("invalid public acceptance contract returned %v, want %s", err, core.Malformed)
	}
}
