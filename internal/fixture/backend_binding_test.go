// pattern: Functional Core
package fixture

import "testing"

func TestBackendBindingContractAcceptsPublicReference(t *testing.T) {
	source := `package backend
const Endpoint = "GET /items"
func FetchItems(cursor string, limit int) string { return "{}" }
`
	report := CheckBackendPublicBinding(source)
	if !report.Passed || report.ReasonCode != "public_binding_valid" {
		t.Fatalf("correct public binding rejected: %+v", report)
	}
	if report.ContractRevision != BackendBindingContractRevision {
		t.Fatalf("binding contract revision missing: %+v", report)
	}
}

func TestBackendBindingContractReportsFunctionNameMismatch(t *testing.T) {
	source := `package backend
const Endpoint = "GET /items"
func FetchPage(cursor string, limit int) string { return "{}" }
`
	report := CheckBackendPublicBinding(source)
	if report.Passed || report.ReasonCode != "entrypoint_missing" {
		t.Fatalf("wrong function name was not classified: %+v", report)
	}
	if report.ExpectedBinding.Entrypoint != "FetchItems" || report.ActualBinding.Entrypoint != "FetchPage" {
		t.Fatalf("expected/actual entrypoint was not exposed: %+v", report)
	}
}

func TestBackendBindingContractReportsParameterAndReturnMismatch(t *testing.T) {
	wrongParameter := CheckBackendPublicBinding(`package backend
const Endpoint = "GET /items"
func FetchItems(cursor *string, limit int) string { return "{}" }
`)
	if wrongParameter.Passed || wrongParameter.ReasonCode != "parameter_type_mismatch" {
		t.Fatalf("wrong parameter type was not classified: %+v", wrongParameter)
	}
	wrongReturn := CheckBackendPublicBinding(`package backend
const Endpoint = "GET /items"
func FetchItems(cursor string, limit int) map[string]string { return nil }
`)
	if wrongReturn.Passed || wrongReturn.ReasonCode != "return_type_mismatch" {
		t.Fatalf("wrong return type was not classified: %+v", wrongReturn)
	}
}

func TestBackendBindingContractReportsUnsupportedAndSyntaxFailures(t *testing.T) {
	unsupported := CheckBackendPublicBinding(`package backend
type Item struct{ ID string }
const Endpoint = "GET /items"
func FetchItems(cursor string, limit int) string { return "{}" }
`)
	if unsupported.Passed || unsupported.ReasonCode != "unsupported_declaration" {
		t.Fatalf("unsupported declaration was not classified: %+v", unsupported)
	}
	syntax := CheckBackendPublicBinding("package backend\nfunc FetchItems(cursor string, limit int) string {\n")
	if syntax.Passed || syntax.ReasonCode != "syntax_invalid" || len(syntax.ParserDiagnostics) == 0 {
		t.Fatalf("syntax failure did not include bounded parser diagnostics: %+v", syntax)
	}
	method := CheckBackendPublicBinding(`package backend
const Endpoint = "GET /items"
type receiver struct{}
func (receiver) FetchItems(cursor string, limit int) string { return "{}" }
`)
	if method.Passed || method.ReasonCode != "declaration_kind_invalid" {
		t.Fatalf("method binding was not classified: %+v", method)
	}
}

func TestBackendBindingContractPassesBeforeBehaviorCheck(t *testing.T) {
	wrongBehavior := CheckBackendPublicBinding(`package backend
const Endpoint = "GET /items"
func FetchItems(cursor string, limit int) string { return "items,next_cursor" }
`)
	if !wrongBehavior.Passed {
		t.Fatalf("behaviorally wrong but ABI-correct candidate failed binding: %+v", wrongBehavior)
	}
}

func TestBackendBindingContractV2PublishesReturnRepresentationBridge(t *testing.T) {
	contract := BackendBindingContractV2()
	if contract.Revision != BackendBindingContractRevisionV2 || contract.ReturnRepresentation != "utf8_json" || contract.ReturnSchemaRef != "r03a-pagination-contract@3#response" {
		t.Fatalf("binding@2 did not publish the representation bridge: %+v", contract)
	}
	report := CheckBackendPublicBindingV2(`package backend
const Endpoint = "GET /items"
func FetchItems(cursor string, limit int) string { return "{}" }
`)
	if !report.Passed || report.ExpectedBinding.ReturnRepresentation != "utf8_json" || report.ContractRevision != BackendBindingContractRevisionV2 {
		t.Fatalf("binding@2 rejected a valid ABI: %+v", report)
	}
}
