// pattern: Functional Core
package fixture

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

const FrontendConsumptionContractRevision = "r03a-frontend-consumption-contract@1"
const FrontendBindingContractRevision = "r03a-frontend-binding@1"
const FrontendPaginationBehaviorVerifierRevision = "r03a-frontend-pagination-behavior@1"

// FrontendConsumptionContractV1 is the language-independent step contract.
// It observes the frontend decision after one structured response, rather
// than inferring behavior from source strings or a verifier-only page shape.
const FrontendConsumptionContractV1 = `{"input":{"current_cursor":"string|null","requested_limit":"integer >= 1","response":{"items":[{"id":"string","name":"string"}],"next_cursor":"string|null"}},"output":{"rendered_items":[{"id":"string","name":"string"}],"next_action":{"kind":"request|stop","cursor":"string when request","limit":"same requested_limit when request"}},"semantics":{"consume":"read every response item id and name","next_page":"when response.next_cursor is non-null, request the exact cursor with the same limit","termination":"when response.next_cursor is null, return stop and issue no further request"}}`

// FrozenV6Revision12FrontendCandidate is copied from immutable V6 protocol
// evidence as a regression fixture. It is not historical evidence storage.
const FrozenV6Revision12FrontendCandidate = `package frontend

func ConsumeItems(body string) string {
	start := -1
	for i := 0; i+9 <= len(body); i++ {
		if body[i:i+9] == "{\"items\"" {
			start = i
			break
		}
	}
	if start >= 0 {
		depth := 0
		for i := start; i < len(body); i++ {
			if body[i] == '{' {
				depth++
			}
			if body[i] == '}' {
				depth--
				if depth == 0 {
					return body[start : i+1]
				}
			}
		}
	}
	firstPage := false
	for i := 0; i < len(body); i++ {
		if i+5 <= len(body) && body[i:i+5] == "first" {
			firstPage = true
		}
		if i+2 <= len(body) && (body[i:i+2] == "i1" || body[i:i+2] == "i2") {
			firstPage = true
		}
	}
	if firstPage {
		return "{\"items\":[{\"id\":\"i1\",\"name\":\"one\"},{\"id\":\"i2\",\"name\":\"two\"}],\"next_cursor\":\"cursor-2\"}"
	}
	return "{\"items\":[{\"id\":\"i3\",\"name\":\"three\"}],\"next_cursor\":null}"
}
`

const FrontendConsumptionReferenceSource = `package frontend

import "encoding/json"

type Item struct {
	ID string ` + "`json:\"id\"`" + `
	Name string ` + "`json:\"name\"`" + `
}

type Response struct {
	Items []Item ` + "`json:\"items\"`" + `
	NextCursor *string ` + "`json:\"next_cursor\"`" + `
}

type Input struct {
	CurrentCursor *string ` + "`json:\"current_cursor\"`" + `
	RequestedLimit int ` + "`json:\"requested_limit\"`" + `
	Response Response ` + "`json:\"response\"`" + `
}

type OutputItem struct {
	ID string ` + "`json:\"id\"`" + `
	Name string ` + "`json:\"name\"`" + `
}

type Action struct {
	Kind string ` + "`json:\"kind\"`" + `
	Cursor *string ` + "`json:\"cursor,omitempty\"`" + `
	Limit int ` + "`json:\"limit,omitempty\"`" + `
}

type Output struct {
	RenderedItems []OutputItem ` + "`json:\"rendered_items\"`" + `
	NextAction Action ` + "`json:\"next_action\"`" + `
}

func ConsumeItems(stepJSON string) string {
	var input Input
	if json.Unmarshal([]byte(stepJSON), &input) != nil {
		return "{}"
	}
	output := Output{NextAction: Action{Kind: "stop"}}
	for _, item := range input.Response.Items {
		output.RenderedItems = append(output.RenderedItems, OutputItem{ID: item.ID, Name: item.Name})
	}
	if input.Response.NextCursor != nil {
		output.NextAction = Action{Kind: "request", Cursor: input.Response.NextCursor, Limit: input.RequestedLimit}
	}
	raw, _ := json.Marshal(output)
	return string(raw)
}
`

type FrontendConsumptionStepInput struct {
	CurrentCursor  *string                     `json:"current_cursor"`
	RequestedLimit int                         `json:"requested_limit"`
	Response       FrontendConsumptionResponse `json:"response"`
}

type FrontendConsumptionResponse struct {
	Items      []FrontendConsumptionItem `json:"items"`
	NextCursor *string                   `json:"next_cursor"`
}

type FrontendConsumptionItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type FrontendConsumptionStepOutput struct {
	RenderedItems []FrontendConsumptionItem `json:"rendered_items"`
	NextAction    FrontendNextAction        `json:"next_action"`
}

type FrontendNextAction struct {
	Kind   string  `json:"kind"`
	Cursor *string `json:"cursor,omitempty"`
	Limit  int     `json:"limit,omitempty"`
}

type FrontendConsumptionCriterion struct {
	CriterionID string `json:"criterion_id"`
	Passed      bool   `json:"passed"`
	ReasonCode  string `json:"reason_code"`
	Expected    string `json:"expected_public_behavior"`
	Actual      string `json:"actual_observed_behavior"`
	FailingStep string `json:"failing_step"`
}

type FrontendConsumptionReport struct {
	Passed           bool                           `json:"passed"`
	ContractRevision string                         `json:"contract_revision"`
	BindingRevision  string                         `json:"binding_revision"`
	VerifierRevision string                         `json:"verifier_revision"`
	Criteria         []FrontendConsumptionCriterion `json:"criteria"`
}

type FrontendConsumptionStepFunc func(FrontendConsumptionStepInput) FrontendConsumptionStepOutput

func ParseFrontendConsumptionContract(raw string) (map[string]any, error) {
	var contract map[string]any
	if err := json.Unmarshal([]byte(raw), &contract); err != nil {
		return nil, fmt.Errorf("invalid frontend consumption contract: %w", err)
	}
	if len(contract) == 0 {
		return nil, errors.New("frontend consumption contract is empty")
	}
	return contract, nil
}

func FrontendConsumptionProbeInputs() []FrontendConsumptionStepInput {
	next := "cursor-2"
	return []FrontendConsumptionStepInput{
		{CurrentCursor: nil, RequestedLimit: 2, Response: FrontendConsumptionResponse{Items: []FrontendConsumptionItem{{ID: "i1", Name: "one"}, {ID: "i2", Name: "two"}}, NextCursor: &next}},
		{CurrentCursor: &next, RequestedLimit: 2, Response: FrontendConsumptionResponse{Items: []FrontendConsumptionItem{{ID: "i3", Name: "three"}}, NextCursor: nil}},
		{CurrentCursor: nil, RequestedLimit: 7, Response: FrontendConsumptionResponse{Items: []FrontendConsumptionItem{{ID: "x7", Name: "seven"}, {ID: "x8", Name: "eight"}}, NextCursor: frontendStringPointer("next-x")}},
	}
}

func VerifyFrontendConsumptionBehavior(fn FrontendConsumptionStepFunc) FrontendConsumptionReport {
	contract := FrontendConsumptionReport{Passed: true, ContractRevision: FrontendConsumptionContractRevision, BindingRevision: FrontendBindingContractRevision, VerifierRevision: FrontendPaginationBehaviorVerifierRevision}
	for index, input := range FrontendConsumptionProbeInputs() {
		step := fmt.Sprintf("step-%d", index+1)
		output := fn(input)
		itemsOK := reflect.DeepEqual(output.RenderedItems, input.Response.Items)
		contract.Criteria = append(contract.Criteria, frontendCriterion("items_consumed", "response_items_not_consumed", itemsOK, "render every response item id", outputSummary(output.RenderedItems), step))
		namesOK := itemsNamesEqual(output.RenderedItems, input.Response.Items)
		contract.Criteria = append(contract.Criteria, frontendCriterion("names_consumed", "response_name_not_consumed", namesOK, "preserve every response item name", outputSummary(output.RenderedItems), step))
		if input.Response.NextCursor != nil {
			requestOK := output.NextAction.Kind == "request"
			contract.Criteria = append(contract.Criteria, frontendCriterion("next_cursor_consumed", "next_cursor_not_consumed", requestOK, "consume non-null next_cursor as a request action", output.NextAction.Kind, step))
			cursorOK := requestOK && output.NextAction.Cursor != nil && *output.NextAction.Cursor == *input.Response.NextCursor
			contract.Criteria = append(contract.Criteria, frontendCriterion("cursor_propagated", "cursor_not_propagated", cursorOK, "request cursor equals response.next_cursor exactly", pointerSummary(output.NextAction.Cursor), step))
			limitOK := requestOK && output.NextAction.Limit == input.RequestedLimit
			contract.Criteria = append(contract.Criteria, frontendCriterion("limit_propagated", "limit_not_propagated", limitOK, "request limit equals requested_limit", fmt.Sprint(output.NextAction.Limit), step))
		} else {
			stopOK := output.NextAction.Kind == "stop" && output.NextAction.Cursor == nil && output.NextAction.Limit == 0
			contract.Criteria = append(contract.Criteria, frontendCriterion("termination_observed", "termination_not_observed", stopOK, "null next_cursor produces stop with no further request", fmt.Sprintf("kind=%s cursor=%s limit=%d", output.NextAction.Kind, pointerSummary(output.NextAction.Cursor), output.NextAction.Limit), step))
		}
	}
	for _, criterion := range contract.Criteria {
		if !criterion.Passed {
			contract.Passed = false
		}
	}
	return contract
}

func VerifyFrontendConsumptionOutputsForRuntime(inputs []FrontendConsumptionStepInput, outputs []FrontendConsumptionStepOutput) FrontendConsumptionReport {
	if len(inputs) != len(outputs) {
		return FrontendConsumptionReport{Passed: false, ContractRevision: FrontendConsumptionContractRevision, BindingRevision: FrontendBindingContractRevision, VerifierRevision: FrontendPaginationBehaviorVerifierRevision, Criteria: []FrontendConsumptionCriterion{{CriterionID: "step_cardinality", Passed: false, ReasonCode: "step_cardinality_mismatch", Expected: "one result per public step", Actual: fmt.Sprintf("inputs=%d outputs=%d", len(inputs), len(outputs))}}}
	}
	report := FrontendConsumptionReport{Passed: true, ContractRevision: FrontendConsumptionContractRevision, BindingRevision: FrontendBindingContractRevision, VerifierRevision: FrontendPaginationBehaviorVerifierRevision}
	for index, input := range inputs {
		step := fmt.Sprintf("step-%d", index+1)
		output := outputs[index]
		itemsOK := reflect.DeepEqual(output.RenderedItems, input.Response.Items)
		report.Criteria = append(report.Criteria, frontendCriterion("items_consumed", "response_items_not_consumed", itemsOK, "render every response item id", outputSummary(output.RenderedItems), step))
		namesOK := itemsNamesEqual(output.RenderedItems, input.Response.Items)
		report.Criteria = append(report.Criteria, frontendCriterion("names_consumed", "response_name_not_consumed", namesOK, "preserve every response item name", outputSummary(output.RenderedItems), step))
		if input.Response.NextCursor != nil {
			requestOK := output.NextAction.Kind == "request"
			report.Criteria = append(report.Criteria, frontendCriterion("next_cursor_consumed", "next_cursor_not_consumed", requestOK, "consume non-null next_cursor as a request action", output.NextAction.Kind, step))
			cursorOK := requestOK && output.NextAction.Cursor != nil && *output.NextAction.Cursor == *input.Response.NextCursor
			report.Criteria = append(report.Criteria, frontendCriterion("cursor_propagated", "cursor_not_propagated", cursorOK, "request cursor equals response.next_cursor exactly", pointerSummary(output.NextAction.Cursor), step))
			limitOK := requestOK && output.NextAction.Limit == input.RequestedLimit
			report.Criteria = append(report.Criteria, frontendCriterion("limit_propagated", "limit_not_propagated", limitOK, "request limit equals requested_limit", fmt.Sprint(output.NextAction.Limit), step))
		} else {
			stopOK := output.NextAction.Kind == "stop" && output.NextAction.Cursor == nil && output.NextAction.Limit == 0
			report.Criteria = append(report.Criteria, frontendCriterion("termination_observed", "termination_not_observed", stopOK, "null next_cursor produces stop with no further request", fmt.Sprintf("kind=%s cursor=%s limit=%d", output.NextAction.Kind, pointerSummary(output.NextAction.Cursor), output.NextAction.Limit), step))
		}
	}
	for _, criterion := range report.Criteria {
		if !criterion.Passed {
			report.Passed = false
		}
	}
	return report
}

func frontendCriterion(criterionID, reason string, passed bool, expected, actual, step string) FrontendConsumptionCriterion {
	if passed {
		reason = ""
	}
	return FrontendConsumptionCriterion{CriterionID: criterionID, Passed: passed, ReasonCode: reason, Expected: expected, Actual: actual, FailingStep: step}
}

func itemsNamesEqual(actual, expected []FrontendConsumptionItem) bool {
	if len(actual) != len(expected) {
		return false
	}
	for index := range expected {
		if actual[index].Name != expected[index].Name {
			return false
		}
	}
	return true
}

func outputSummary(items []FrontendConsumptionItem) string {
	raw, _ := json.Marshal(items)
	return string(raw)
}

func pointerSummary(value *string) string {
	if value == nil {
		return "null"
	}
	return *value
}

func frontendStringPointer(value string) *string { return &value }
