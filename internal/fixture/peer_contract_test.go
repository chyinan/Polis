// pattern: Functional Core
package fixture

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPeerBackendCandidateUsesPublicContractSemantics(t *testing.T) {
	contract := PeerContractSpec{Endpoint: "GET /items?cursor=...", Schema: `{"items":[],"next_cursor":""}`}
	cases := []struct {
		name    string
		content string
		passed  bool
		reason  string
	}{
		{"semantic wrong response", `package backend
const Endpoint = "GET /items?cursor=..."
func FetchItems() string { return "items" }
`, false, "response_shape_mismatch"},
		{"neighboring wrong endpoint", `package backend
const Endpoint = "GET /items?offset=..."
func FetchItems() string { return "items,next_cursor" }
`, false, "endpoint_mismatch"},
		{"correct implementation without marker", `package backend
const Endpoint = "GET /items?cursor=..."
func FetchItems() string { return "items,next_cursor" }
`, true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			criteria := CheckPeerBackendCandidate(tc.content, contract)
			if CandidateCriteriaPassed(criteria) != tc.passed {
				t.Fatalf("passed=%v, want %v: %+v", CandidateCriteriaPassed(criteria), tc.passed, criteria)
			}
			if !tc.passed {
				found := false
				for _, criterion := range criteria {
					if !criterion.Passed && criterion.PublicReasonCode == tc.reason {
						found = true
					}
				}
				if !found {
					t.Fatalf("missing actionable reason %q: %+v", tc.reason, criteria)
				}
			}
			raw, err := json.Marshal(criteria)
			if err != nil {
				t.Fatal(err)
			}
			for _, forbidden := range []string{"contract-v2", "applied-contract-revision", "hidden", "oracle", "sentinel"} {
				if strings.Contains(string(raw), forbidden) {
					t.Fatalf("public criteria leaked hidden implementation %q: %s", forbidden, raw)
				}
			}
		})
	}
}

func TestPeerFrontendCandidateUsesResponseSemantics(t *testing.T) {
	contract := PeerContractSpec{Endpoint: "GET /items?cursor=...", Schema: `{"items":[],"next_cursor":""}`}
	wrong := CheckPeerFrontendCandidate(`package frontend
func ConsumeItems(body string) string { return "items" }
`, contract)
	if CandidateCriteriaPassed(wrong) {
		t.Fatalf("semantic frontend candidate unexpectedly passed: %+v", wrong)
	}
	correct := CheckPeerFrontendCandidate(`package frontend
func ConsumeItems(body string) string { return "items,next_cursor" }
`, contract)
	if !CandidateCriteriaPassed(correct) {
		t.Fatalf("semantic frontend candidate was rejected: %+v", correct)
	}
}

func TestPeerFrontendCandidateReportsBoundedPublicSyntaxDiagnostics(t *testing.T) {
	contract := PeerContractSpec{Schema: `{"items":[{"id":"string","name":"string"}],"next_cursor":"string|null"}`}
	criteria := CheckPeerFrontendCandidate("package frontend\nfunc ConsumeItems(", contract)
	if CandidateCriteriaPassed(criteria) {
		t.Fatalf("invalid syntax unexpectedly passed: %+v", criteria)
	}
	syntax := criterionByID(criteria, "candidate_syntax")
	if syntax.Passed || syntax.PublicReasonCode != "candidate_syntax_invalid" || len(syntax.ParserDiagnostics) == 0 {
		t.Fatalf("syntax diagnostic missing: %+v", syntax)
	}
	diagnostic := syntax.ParserDiagnostics[0]
	if diagnostic.PublicCategory == "" || diagnostic.Line == 0 || diagnostic.Column == 0 || diagnostic.Message == "" || len([]rune(diagnostic.Message)) > maxPublicParserMessageRunes {
		t.Fatalf("syntax diagnostic is not bounded/actionable: %+v", diagnostic)
	}
	if len(syntax.ParserDiagnostics) > maxPublicParserDiagnostics {
		t.Fatalf("too many parser diagnostics: %d", len(syntax.ParserDiagnostics))
	}
	if stringContainsHiddenCheckerText(criteria) {
		t.Fatalf("hidden checker details leaked: %+v", criteria)
	}
}

func TestPeerFrontendCandidateReportsMissingPublicField(t *testing.T) {
	contract := PeerContractSpec{Schema: `{"items":[{"id":"string","name":"string"}],"next_cursor":"string|null"}`}
	criteria := CheckPeerFrontendCandidate(`package frontend
func ConsumeItems(body string) string { return "items" }
`, contract)
	shape := criterionByID(criteria, "response_shape_compatibility")
	if shape.Passed || !containsString(shape.MissingRequiredFields, "next_cursor") {
		t.Fatalf("missing public field was not identified: %+v", shape)
	}
	if shape.ExpectedPublicShape["next_cursor"] != "string|null" || shape.ActualShapeSummary == "" {
		t.Fatalf("public shape evidence missing: %+v", shape)
	}
}

func TestPeerFrontendCandidateReportsPublicTypeMismatch(t *testing.T) {
	contract := PeerContractSpec{Schema: `{"items":[{"id":"string","name":"string"}],"next_cursor":"string|null"}`}
	criteria := CheckPeerFrontendCandidate(`package frontend
func ConsumeItems(body string) string { return "items:array,next_cursor:int" }
`, contract)
	shape := criterionByID(criteria, "response_shape_compatibility")
	if shape.Passed || len(shape.TypeMismatches) != 2 {
		t.Fatalf("public type mismatch was not identified: %+v", shape)
	}
	if shape.TypeMismatches[0].Expected == "" || shape.TypeMismatches[0].Actual == "" {
		t.Fatalf("type mismatch lacks expected/actual type: %+v", shape.TypeMismatches)
	}
}

func TestPeerFrontendCandidateReportsUnexpectedFieldsOnlyWhenContractForbidsThem(t *testing.T) {
	content := `package frontend
func ConsumeItems(body string) string { return "items,next_cursor,debug" }
`
	allowed := CheckPeerFrontendCandidate(content, PeerContractSpec{Schema: `{"items":[],"next_cursor":"string"}`})
	if len(criterionByID(allowed, "response_shape_compatibility").UnexpectedFields) != 0 {
		t.Fatalf("unexpected field was reported without a public prohibition: %+v", allowed)
	}
	forbidden := CheckPeerFrontendCandidate(content, PeerContractSpec{Schema: `{"items":[],"next_cursor":"string"}`, ForbidUnexpectedFields: true})
	shape := criterionByID(forbidden, "response_shape_compatibility")
	if shape.Passed || !containsString(shape.UnexpectedFields, "debug") || len(shape.UnexpectedFields) > maxPublicUnexpectedFields {
		t.Fatalf("publicly forbidden unexpected field was not bounded/reported: %+v", shape)
	}
}

func TestPeerFrontendCandidateFeedbackIsDeterministicAndBounded(t *testing.T) {
	contract := PeerContractSpec{Schema: `{"items":[{"id":"string","name":"string"}],"next_cursor":"string|null"}`}
	content := "package frontend\nfunc ConsumeItems(body string) string { return \"items:wrong,next_cursor:int,unknown-a,unknown-b,unknown-c,unknown-d,unknown-e\" }\n"
	first := CheckPeerFrontendCandidate(content, contract)
	second := CheckPeerFrontendCandidate(content, contract)
	firstJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	secondJSON, err := json.Marshal(second)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("feedback is not deterministic:\n%s\n%s", firstJSON, secondJSON)
	}
	if len(firstJSON) > maxPublicFeedbackBytes {
		t.Fatalf("feedback is not bounded: %d bytes", len(firstJSON))
	}
}

func criterionByID(criteria []PeerCandidateCriterion, id string) PeerCandidateCriterion {
	for _, criterion := range criteria {
		if criterion.CriterionID == id {
			return criterion
		}
	}
	return PeerCandidateCriterion{}
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func stringContainsHiddenCheckerText(criteria []PeerCandidateCriterion) bool {
	raw, err := json.Marshal(criteria)
	if err != nil {
		return true
	}
	text := string(raw)
	for _, forbidden := range []string{"contract-v2", "applied-contract-revision", "hidden", "oracle", "sentinel", "literal"} {
		if strings.Contains(text, forbidden) {
			return true
		}
	}
	return false
}
