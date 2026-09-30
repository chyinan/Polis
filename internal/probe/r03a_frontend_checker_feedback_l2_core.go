// pattern: Functional Core
package probe

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"

	"polis/internal/codex"
	"polis/internal/fixture"
)

type FrontendRegistrationChange struct {
	Location string `json:"location"`
	Old      any    `json:"old"`
	Current  any    `json:"current"`
}

type FrontendRegistrationDiff struct {
	Equivalent bool                         `json:"equivalent"`
	Changes    []FrontendRegistrationChange `json:"changes"`
}

type FrontendPolicyChange struct {
	Field   string `json:"field"`
	Old     string `json:"old"`
	Current string `json:"current"`
}

type FrontendPolicyDiff struct {
	OnlyCheckerRevisionChanged bool                   `json:"only_checker_revision_changed"`
	Changes                    []FrontendPolicyChange `json:"changes"`
}

type CheckerFeedbackRegressionCase struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail"`
}

func compareFrontendRegistrationSurface(old, current codex.ToolSurfaceManifest) FrontendRegistrationDiff {
	diff := FrontendRegistrationDiff{Equivalent: true}
	add := func(location string, previous, actual any) {
		if reflect.DeepEqual(previous, actual) {
			return
		}
		diff.Equivalent = false
		diff.Changes = append(diff.Changes, FrontendRegistrationChange{Location: location, Old: previous, Current: actual})
	}
	add("surface_id", old.SurfaceID, current.SurfaceID)
	add("tool_count", old.ToolCount, current.ToolCount)
	add("aggregate_manifest_digest", old.AggregateManifestDigest, current.AggregateManifestDigest)
	add("aggregate_schema_digest", old.AggregateSchemaDigest, current.AggregateSchemaDigest)
	add("aggregate_schema_bytes", old.AggregateSchemaBytes, current.AggregateSchemaBytes)
	add("thread_start_payload_digest", old.ThreadStartPayloadDigest, current.ThreadStartPayloadDigest)
	add("thread_start_payload_bytes", old.ThreadStartPayloadBytes, current.ThreadStartPayloadBytes)
	add("business_write_policy", old.BusinessWritePolicy, current.BusinessWritePolicy)
	for index := 0; index < len(old.Tools) && index < len(current.Tools); index++ {
		previous, actual := old.Tools[index], current.Tools[index]
		ordinal := index + 1
		add(toolRegistrationLocation(ordinal, "name"), previous.Name, actual.Name)
		add(toolRegistrationLocation(ordinal, "schema_digest"), previous.SchemaDigest, actual.SchemaDigest)
		add(toolRegistrationLocation(ordinal, "schema_bytes"), previous.SchemaBytes, actual.SchemaBytes)
		add(toolRegistrationLocation(ordinal, "description_digest"), previous.DescriptionDigest, actual.DescriptionDigest)
		add(toolRegistrationLocation(ordinal, "binding_identity"), previous.BindingIdentity, actual.BindingIdentity)
		add(toolRegistrationLocation(ordinal, "authorization_class"), previous.AuthorizationClass, actual.AuthorizationClass)
		add(toolRegistrationLocation(ordinal, "registration_ordinal"), previous.RegistrationOrdinal, actual.RegistrationOrdinal)
	}
	if len(old.Tools) != len(current.Tools) {
		add("tools.length", len(old.Tools), len(current.Tools))
	}
	return diff
}

func toolRegistrationLocation(ordinal int, field string) string {
	return "tools[" + formatOrdinal(ordinal) + "]." + field
}

func formatOrdinal(ordinal int) string {
	return strconv.Itoa(ordinal)
}

func compareFrontendPolicyRevisions(old, current codex.ToolSurfaceManifest) FrontendPolicyDiff {
	fields := []struct {
		name string
		old  string
		new  string
	}{
		{"checkpoint_policy_revision", old.CheckpointPolicyRevision, current.CheckpointPolicyRevision},
		{"artifact_eligibility_policy_revision", old.ArtifactEligibilityPolicyRevision, current.ArtifactEligibilityPolicyRevision},
		{"contract_supersession_policy_revision", old.ContractSupersessionPolicyRevision, current.ContractSupersessionPolicyRevision},
		{"acceptance_checker_revision", old.AcceptanceCheckerRevision, current.AcceptanceCheckerRevision},
	}
	diff := FrontendPolicyDiff{OnlyCheckerRevisionChanged: true}
	for _, field := range fields {
		if field.old == field.new {
			continue
		}
		diff.Changes = append(diff.Changes, FrontendPolicyChange{Field: field.name, Old: field.old, Current: field.new})
		if field.name != "acceptance_checker_revision" {
			diff.OnlyCheckerRevisionChanged = false
		}
	}
	if len(diff.Changes) != 1 {
		diff.OnlyCheckerRevisionChanged = false
	}
	return diff
}

func frontendCheckerFeedbackRegression() []CheckerFeedbackRegressionCase {
	contract := fixture.PeerContractSpec{Schema: `{"items":[{"id":"string","name":"string"}],"next_cursor":"string|null"}`}
	invalidSyntax := fixture.CheckPeerFrontendCandidate("package frontend\nfunc ConsumeItems(", contract)
	missingField := fixture.CheckPeerFrontendCandidate(`package frontend
func ConsumeItems(body string) string { return "items" }
`, contract)
	wrongType := fixture.CheckPeerFrontendCandidate(`package frontend
func ConsumeItems(body string) string { return "items:array,next_cursor:int" }
`, contract)
	unexpectedCandidate := `package frontend
func ConsumeItems(body string) string { return "items,next_cursor,debug" }
`
	unexpectedAllowed := fixture.CheckPeerFrontendCandidate(unexpectedCandidate, contract)
	contract.ForbidUnexpectedFields = true
	unexpectedForbidden := fixture.CheckPeerFrontendCandidate(unexpectedCandidate, contract)
	valid := fixture.CheckPeerFrontendCandidate(`package frontend
func ConsumeItems(body string) string { return "items,next_cursor" }
`, contract)
	cases := []CheckerFeedbackRegressionCase{
		{Name: "invalid_syntax_public_diagnostic", Passed: !fixture.CandidateCriteriaPassed(invalidSyntax) && len(invalidSyntax[0].ParserDiagnostics) > 0, Detail: "candidate_syntax has bounded parser diagnostics"},
		{Name: "missing_required_public_field", Passed: !fixture.CandidateCriteriaPassed(missingField) && containsPublicString(missingField[1].MissingRequiredFields, "next_cursor"), Detail: "response_shape identifies next_cursor"},
		{Name: "wrong_public_type", Passed: !fixture.CandidateCriteriaPassed(wrongType) && len(wrongType[1].TypeMismatches) == 2, Detail: "response_shape identifies expected and actual types"},
		{Name: "public_unexpected_field_policy", Passed: fixture.CandidateCriteriaPassed(unexpectedAllowed) && !fixture.CandidateCriteriaPassed(unexpectedForbidden) && containsPublicCriterionField(unexpectedForbidden, "debug"), Detail: "unexpected fields are reported only when the public contract forbids them"},
		{Name: "valid_candidate_passes", Passed: fixture.CandidateCriteriaPassed(valid), Detail: "valid public contract candidate remains accepted"},
	}
	first, _ := json.Marshal(missingField)
	second, _ := json.Marshal(fixture.CheckPeerFrontendCandidate(`package frontend
func ConsumeItems(body string) string { return "items" }
`, contract))
	cases = append(cases, CheckerFeedbackRegressionCase{Name: "deterministic_bounded_feedback", Passed: string(first) == string(second) && len(first) <= 8192, Detail: "stable ordering and bounded public payload"})
	forbidden, _ := json.Marshal(invalidSyntax)
	for _, word := range []string{"hidden verifier", "oracle implementation", "sentinel literal", "contract-v2"} {
		if strings.Contains(string(forbidden), word) {
			cases = append(cases, CheckerFeedbackRegressionCase{Name: "hidden_oracle_isolated", Passed: false, Detail: "forbidden checker detail leaked"})
			return cases
		}
	}
	cases = append(cases, CheckerFeedbackRegressionCase{Name: "hidden_oracle_isolated", Passed: true, Detail: "only public parser and contract facts are exposed"})
	return cases
}

func containsPublicString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func containsPublicCriterionField(criteria []fixture.PeerCandidateCriterion, wanted string) bool {
	for _, criterion := range criteria {
		if containsPublicString(criterion.UnexpectedFields, wanted) {
			return true
		}
	}
	return false
}

func checkerFeedbackRegressionPassed(cases []CheckerFeedbackRegressionCase) bool {
	for _, item := range cases {
		if !item.Passed {
			return false
		}
	}
	return len(cases) > 0
}
