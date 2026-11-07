// pattern: Functional Core
package fixture

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

// PeerPaginationContractV3 is a public future contract fixture. Its probe
// cases are public inputs for deterministic qualification, not implementation
// instructions supplied to an employee prompt.
const PeerPaginationContractV3 = `{"request":{"method":"GET","path":"/items","cursor":{"name":"cursor","type":"string|null","required":false,"semantics":"initial null; subsequent requests use the prior response next_cursor exactly"},"limit":{"name":"limit","type":"integer","required":true,"minimum":1,"semantics":"maximum number of items requested"}},"response":{"items":{"type":"array<object>","required_fields":["id","name"]},"next_cursor":{"type":"string|null","required":true}},"pagination":{"next_page":"when next_cursor is a string, issue the same request with cursor exactly equal to next_cursor","termination":"when next_cursor is null, stop and issue no further request"},"probe_cases":[{"name":"first_page","cursor":null,"limit":2,"expected_items":[{"id":"i1","name":"one"},{"id":"i2","name":"two"}],"expected_next_cursor":"cursor-2"},{"name":"last_page","cursor":"cursor-2","limit":2,"expected_items":[{"id":"i3","name":"three"}],"expected_next_cursor":null}]}`

const PeerPaginationContractRevision = "r03a-pagination-contract@3"
const PeerPaginationBehaviorVerifierRevision = "r03a-pagination-behavior@1"
const PeerPaginationBehaviorVerifierRevisionV2 = "r03a-pagination-behavior@2"

const PeerBackendPaginationReference = `package backend
const Endpoint = "GET /items?cursor={cursor}&limit={limit}"
func FetchItems(cursor string, limit int) string {
	if limit != 2 { return "{\"items\":[],\"next_cursor\":null}" }
	if cursor == "" { return "{\"items\":[{\"id\":\"i1\",\"name\":\"one\"},{\"id\":\"i2\",\"name\":\"two\"}],\"next_cursor\":\"cursor-2\"}" }
	return "{\"items\":[{\"id\":\"i3\",\"name\":\"three\"}],\"next_cursor\":null}"
}
`

const PeerFrontendPaginationReference = `package frontend
func ConsumeItems(body string) string {
	if body == "{\"items\":[{\"id\":\"i1\",\"name\":\"one\"},{\"id\":\"i2\",\"name\":\"two\"}],\"next_cursor\":\"cursor-2\"}" { return "{\"rendered_ids\":[\"i1\",\"i2\"],\"rendered_names\":[\"one\",\"two\"],\"next_cursor\":\"cursor-2\"}" }
	return "{\"rendered_ids\":[\"i3\"],\"rendered_names\":[\"three\"],\"next_cursor\":null}"
}
`

type PaginationBehaviorContract struct {
	Request    PaginationRequestContract     `json:"request"`
	Response   PaginationResponseContract    `json:"response"`
	Pagination PaginationSemantics           `json:"pagination"`
	ProbeCases []PaginationBehaviorProbeCase `json:"probe_cases"`
}

type PaginationRequestContract struct {
	Method string                      `json:"method"`
	Path   string                      `json:"path"`
	Cursor PaginationParameterContract `json:"cursor"`
	Limit  PaginationParameterContract `json:"limit"`
}

type PaginationParameterContract struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Required  bool   `json:"required"`
	Minimum   int    `json:"minimum,omitempty"`
	Semantics string `json:"semantics"`
}

type PaginationResponseContract struct {
	Items      PaginationItemsContract `json:"items"`
	NextCursor PaginationFieldContract `json:"next_cursor"`
}

type PaginationItemsContract struct {
	Type           string   `json:"type"`
	RequiredFields []string `json:"required_fields"`
}

type PaginationFieldContract struct {
	Type     string `json:"type"`
	Required bool   `json:"required"`
}

type PaginationSemantics struct {
	NextPage    string `json:"next_page"`
	Termination string `json:"termination"`
}

type PaginationBehaviorProbeCase struct {
	Name               string           `json:"name"`
	Cursor             *string          `json:"cursor"`
	Limit              int              `json:"limit"`
	ExpectedItems      []PaginationItem `json:"expected_items"`
	ExpectedNextCursor *string          `json:"expected_next_cursor"`
}

type PaginationItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type PaginationResponse struct {
	Items      []PaginationItem `json:"items"`
	NextCursor *string          `json:"next_cursor"`
}

type FrontendPage struct {
	RenderedIDs   []string `json:"rendered_ids"`
	RenderedNames []string `json:"rendered_names"`
	NextCursor    *string  `json:"next_cursor"`
}

type PaginationBackendFunc func(cursor *string, limit int) PaginationResponse

func (f PaginationBackendFunc) Fetch(cursor *string, limit int) PaginationResponse {
	return f(cursor, limit)
}

type PaginationFrontendFunc func(response PaginationResponse) FrontendPage

func (f PaginationFrontendFunc) Consume(response PaginationResponse) FrontendPage {
	return f(response)
}

type PaginationBehaviorReport struct {
	Passed                   bool                          `json:"passed"`
	BusinessContractRevision string                        `json:"business_contract_revision,omitempty"`
	BindingContractRevision  string                        `json:"binding_contract_revision,omitempty"`
	VerifierRevision         string                        `json:"verifier_revision,omitempty"`
	Criteria                 []PaginationBehaviorCriterion `json:"criteria"`
}

type PaginationBehaviorCriterion struct {
	CriterionID             string `json:"criterion_id"`
	Passed                  bool   `json:"passed"`
	ReasonCode              string `json:"reason_code,omitempty"`
	Case                    string `json:"case,omitempty"`
	Reason                  string `json:"public_reason"`
	ExpectedRepresentation  string `json:"expected_representation,omitempty"`
	BindingContractRevision string `json:"binding_contract_revision,omitempty"`
	VerifierRevision        string `json:"verifier_revision,omitempty"`
}

type PaginationBackend interface {
	Fetch(cursor *string, limit int) PaginationResponse
}

type PaginationFrontend interface {
	Consume(response PaginationResponse) FrontendPage
}

func ParsePaginationBehaviorContract(raw string) (PaginationBehaviorContract, error) {
	var contract PaginationBehaviorContract
	if err := json.Unmarshal([]byte(raw), &contract); err != nil {
		return contract, fmt.Errorf("invalid public pagination contract: %w", err)
	}
	if err := validatePaginationBehaviorContract(contract); err != nil {
		return contract, err
	}
	return contract, nil
}

func MustPeerPaginationContract() PaginationBehaviorContract {
	contract, err := ParsePaginationBehaviorContract(PeerPaginationContractV3)
	if err != nil {
		panic(err)
	}
	return contract
}

func validatePaginationBehaviorContract(contract PaginationBehaviorContract) error {
	if contract.Request.Method != "GET" || contract.Request.Path == "" || contract.Request.Cursor.Name != "cursor" || contract.Request.Limit.Name != "limit" {
		return errors.New("public pagination contract request shape is incomplete")
	}
	if contract.Request.Cursor.Type != "string|null" || contract.Request.Limit.Type != "integer" || contract.Request.Limit.Minimum < 1 || contract.Request.Cursor.Semantics == "" || contract.Request.Limit.Semantics == "" {
		return errors.New("public pagination contract request semantics are incomplete")
	}
	if contract.Response.Items.Type != "array<object>" || !sameStrings(contract.Response.Items.RequiredFields, []string{"id", "name"}) || contract.Response.NextCursor.Type != "string|null" || !contract.Response.NextCursor.Required {
		return errors.New("public pagination contract response shape is incomplete")
	}
	if contract.Pagination.NextPage == "" || contract.Pagination.Termination == "" || len(contract.ProbeCases) < 2 {
		return errors.New("public pagination contract lifecycle semantics are incomplete")
	}
	for _, probe := range contract.ProbeCases {
		if probe.Name == "" || probe.Limit < contract.Request.Limit.Minimum || len(probe.ExpectedItems) == 0 {
			return errors.New("public pagination contract probe case is incomplete")
		}
		for _, item := range probe.ExpectedItems {
			if item.ID == "" || item.Name == "" {
				return errors.New("public pagination contract probe item is incomplete")
			}
		}
	}
	return nil
}

func VerifyPaginationBehavior(contract PaginationBehaviorContract, backend PaginationBackend, frontend PaginationFrontend) PaginationBehaviorReport {
	report := PaginationBehaviorReport{Passed: true}
	if len(contract.ProbeCases) < 2 || backend == nil || frontend == nil {
		return failedPaginationReport(report, "pagination_behavior_inputs", "public pagination behavior inputs are incomplete")
	}
	responses := make([]PaginationResponse, 0, len(contract.ProbeCases))
	for _, probe := range contract.ProbeCases {
		response := backend.Fetch(probe.Cursor, probe.Limit)
		responses = append(responses, response)
	}
	backendReport := VerifyPaginationBackendResponses(contract, responses)
	report.Criteria = append(report.Criteria, backendReport.Criteria...)
	frontendPages := make([]FrontendPage, 0, len(responses))
	for _, response := range responses {
		frontendPages = append(frontendPages, frontend.Consume(response))
	}
	frontendReport := VerifyPaginationFrontendPages(contract, responses, frontendPages)
	report.Criteria = append(report.Criteria, frontendReport.Criteria...)
	report.Passed = backendReport.Passed && frontendReport.Passed
	return report
}

func VerifyPaginationBackendResponses(contract PaginationBehaviorContract, responses []PaginationResponse) PaginationBehaviorReport {
	report := PaginationBehaviorReport{Passed: true}
	if len(responses) != len(contract.ProbeCases) {
		return failedPaginationReport(report, "backend_probe_cardinality", "backend did not produce one response for each public pagination probe")
	}
	for index, response := range responses {
		probe := contract.ProbeCases[index]
		shapeOK := validPaginationResponse(response, contract.Response)
		matches := shapeOK && sameItems(response.Items, probe.ExpectedItems) && sameStringPointer(response.NextCursor, probe.ExpectedNextCursor)
		report.Criteria = append(report.Criteria,
			PaginationBehaviorCriterion{CriterionID: "backend_uses_cursor_and_limit", Passed: matches, Case: probe.Name, Reason: paginationReason(matches, "backend behavior matches the public cursor/limit probe", "backend did not use the public cursor/limit probe")},
			PaginationBehaviorCriterion{CriterionID: "backend_response_matches_public_contract", Passed: matches, Case: probe.Name, Reason: paginationReason(matches, "backend response matches the public response contract", "backend did not return the declared public response")},
		)
		if !matches {
			report.Passed = false
		}
	}
	if len(responses) >= 2 && reflect.DeepEqual(responses[0], responses[1]) {
		return failedPaginationReport(report, "pagination_behavior_changes_with_cursor", "pagination behavior did not change across distinct public cursor probes")
	}
	if responses[len(responses)-1].NextCursor != nil {
		return failedPaginationReport(report, "pagination_termination", "the public terminal probe did not return next_cursor=null")
	}
	return report
}

func VerifyPaginationFrontendPages(contract PaginationBehaviorContract, responses []PaginationResponse, pages []FrontendPage) PaginationBehaviorReport {
	report := PaginationBehaviorReport{Passed: true}
	if len(responses) != len(contract.ProbeCases) || len(pages) != len(responses) {
		return failedPaginationReport(report, "frontend_probe_cardinality", "frontend did not produce one page result for each public response probe")
	}
	for index, page := range pages {
		probe := contract.ProbeCases[index]
		expected := expectedFrontendPage(responses[index].Items, responses[index].NextCursor)
		passed := reflect.DeepEqual(page, expected)
		report.Criteria = append(report.Criteria, PaginationBehaviorCriterion{CriterionID: "frontend_consumes_response_fields", Passed: passed, Case: probe.Name, Reason: paginationReason(passed, "frontend output reflects public item and next_cursor fields", "frontend output does not reflect the public response fields")})
		if !passed {
			report.Passed = false
		}
	}
	terminationOK := pages[len(pages)-1].NextCursor == nil
	report.Criteria = append(report.Criteria, PaginationBehaviorCriterion{CriterionID: "pagination_termination", Passed: terminationOK, Case: contract.ProbeCases[len(contract.ProbeCases)-1].Name, Reason: paginationReason(terminationOK, "null next_cursor is surfaced as the public termination state", "frontend did not surface null next_cursor as termination")})
	if !terminationOK {
		report.Passed = false
	}
	return report
}

func expectedFrontendPage(items []PaginationItem, nextCursor *string) FrontendPage {
	page := FrontendPage{NextCursor: nextCursor}
	for _, item := range items {
		page.RenderedIDs = append(page.RenderedIDs, item.ID)
		page.RenderedNames = append(page.RenderedNames, item.Name)
	}
	return page
}

func failedPaginationReport(report PaginationBehaviorReport, criterionID, reason string) PaginationBehaviorReport {
	report.Passed = false
	report.Criteria = append(report.Criteria, PaginationBehaviorCriterion{CriterionID: criterionID, Reason: reason})
	return report
}

func paginationReason(passed bool, success, failure string) string {
	if passed {
		return success
	}
	return failure
}

func validPaginationResponse(response PaginationResponse, contract PaginationResponseContract) bool {
	if len(response.Items) == 0 && response.NextCursor != nil {
		return false
	}
	for _, item := range response.Items {
		if item.ID == "" || item.Name == "" {
			return false
		}
	}
	return contract.Items.Type == "array<object>" && sameStrings(contract.Items.RequiredFields, []string{"id", "name"}) && contract.NextCursor.Type == "string|null" && contract.NextCursor.Required
}

func sameItems(left, right []PaginationItem) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func sameStringPointer(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func (report PaginationBehaviorReport) HasCriterion(id string) bool {
	for _, criterion := range report.Criteria {
		if criterion.CriterionID == id {
			return true
		}
	}
	return false
}
