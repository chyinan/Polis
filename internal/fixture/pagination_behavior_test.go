// pattern: Functional Core
package fixture

import (
	"encoding/json"
	"testing"
)

func TestPublicPaginationContractRoundTripAndSemantics(t *testing.T) {
	contract, err := ParsePaginationBehaviorContract(PeerPaginationContractV3)
	if err != nil {
		t.Fatal(err)
	}
	if contract.Request.Method != "GET" || contract.Request.Path != "/items" || contract.Request.Cursor.Name != "cursor" || contract.Request.Limit.Name != "limit" {
		t.Fatalf("request contract lost public fields: %+v", contract.Request)
	}
	if contract.Request.Cursor.Semantics != "initial null; subsequent requests use the prior response next_cursor exactly" || contract.Request.Limit.Minimum != 1 {
		t.Fatalf("request pagination semantics are not explicit: %+v", contract.Request)
	}
	if contract.Response.NextCursor.Type != "string|null" || !contract.Response.NextCursor.Required || contract.Response.Items.RequiredFields[0] != "id" || contract.Response.Items.RequiredFields[1] != "name" {
		t.Fatalf("response contract lost public fields: %+v", contract.Response)
	}
	if contract.Pagination.NextPage != "when next_cursor is a string, issue the same request with cursor exactly equal to next_cursor" || contract.Pagination.Termination != "when next_cursor is null, stop and issue no further request" {
		t.Fatalf("pagination lifecycle semantics are not explicit: %+v", contract.Pagination)
	}
	roundTrip, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParsePaginationBehaviorContract(string(roundTrip))
	if err != nil {
		t.Fatal(err)
	}
	if string(roundTrip) != mustJSON(t, parsed) {
		t.Fatalf("pagination contract serialization is not canonical: %s != %s", roundTrip, mustJSON(t, parsed))
	}
}

func TestPaginationBehaviorVerifierRejectsFrozenH1MarkerBehavior(t *testing.T) {
	contract := MustPeerPaginationContract()
	backend := PaginationBackendFunc(func(cursor *string, limit int) PaginationResponse {
		return PaginationResponse{Items: nil, NextCursor: stringPointer("2")}
	})
	frontend := PaginationFrontendFunc(func(response PaginationResponse) FrontendPage {
		return FrontendPage{RenderedIDs: nil, RenderedNames: nil, NextCursor: stringPointer("2")}
	})
	report := VerifyPaginationBehavior(contract, backend, frontend)
	if report.Passed {
		t.Fatalf("frozen marker behavior unexpectedly passed: %+v", report)
	}
	if !report.HasCriterion("backend_uses_cursor_and_limit") || !report.HasCriterion("frontend_consumes_response_fields") {
		t.Fatalf("behavioral failures were not reported: %+v", report)
	}
}

func TestPaginationBehaviorVerifierAcceptsIndependentCorrectReference(t *testing.T) {
	contract := MustPeerPaginationContract()
	backend := PaginationBackendFunc(func(cursor *string, limit int) PaginationResponse {
		if limit != 2 {
			return PaginationResponse{}
		}
		if cursor == nil {
			return PaginationResponse{Items: []PaginationItem{{ID: "i1", Name: "one"}, {ID: "i2", Name: "two"}}, NextCursor: stringPointer("cursor-2")}
		}
		if *cursor == "cursor-2" {
			return PaginationResponse{Items: []PaginationItem{{ID: "i3", Name: "three"}}, NextCursor: nil}
		}
		return PaginationResponse{}
	})
	frontend := PaginationFrontendFunc(func(response PaginationResponse) FrontendPage {
		page := FrontendPage{NextCursor: response.NextCursor}
		for _, item := range response.Items {
			page.RenderedIDs = append(page.RenderedIDs, item.ID)
			page.RenderedNames = append(page.RenderedNames, item.Name)
		}
		return page
	})
	report := VerifyPaginationBehavior(contract, backend, frontend)
	if !report.Passed {
		t.Fatalf("correct reference failed: %+v", report)
	}
}

func TestPaginationBehaviorVerifierRejectsNeighboringWrongImplementations(t *testing.T) {
	contract := MustPeerPaginationContract()
	wrongBackend := PaginationBackendFunc(func(cursor *string, limit int) PaginationResponse {
		return PaginationResponse{Items: []PaginationItem{{ID: "i1", Name: "one"}, {ID: "i2", Name: "two"}}, NextCursor: stringPointer("cursor-2")}
	})
	correctFrontend := PaginationFrontendFunc(func(response PaginationResponse) FrontendPage {
		page := FrontendPage{NextCursor: response.NextCursor}
		for _, item := range response.Items {
			page.RenderedIDs = append(page.RenderedIDs, item.ID)
			page.RenderedNames = append(page.RenderedNames, item.Name)
		}
		return page
	})
	if report := VerifyPaginationBehavior(contract, wrongBackend, correctFrontend); report.Passed {
		t.Fatalf("cursor-insensitive backend passed: %+v", report)
	}

	correctBackend := PaginationBackendFunc(func(cursor *string, limit int) PaginationResponse {
		if cursor == nil {
			return PaginationResponse{Items: []PaginationItem{{ID: "i1", Name: "one"}, {ID: "i2", Name: "two"}}, NextCursor: stringPointer("cursor-2")}
		}
		return PaginationResponse{Items: []PaginationItem{{ID: "i3", Name: "three"}}, NextCursor: nil}
	})
	fixedFrontend := PaginationFrontendFunc(func(response PaginationResponse) FrontendPage {
		return FrontendPage{RenderedIDs: []string{"i1", "i2"}, RenderedNames: []string{"one", "two"}, NextCursor: stringPointer("2")}
	})
	if report := VerifyPaginationBehavior(contract, correctBackend, fixedFrontend); report.Passed {
		t.Fatalf("response-insensitive frontend passed: %+v", report)
	}
}

func TestPeerPaginationContractRejectsAmbiguousFutureRevision(t *testing.T) {
	ambiguous := `{"request":{"method":"GET","path":"/items"},"response":{"items":{"type":"array<object>","required_fields":["id","name"]},"next_cursor":{"type":"string|null","required":true}},"pagination":{"next_page":"","termination":""},"probe_cases":[]}`
	if _, err := ParsePaginationBehaviorContract(ambiguous); err == nil {
		t.Fatal("ambiguous pagination contract was accepted")
	}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func stringPointer(value string) *string { return &value }
