// pattern: Functional Core
package fixture

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"sort"
	"strings"
)

const BackendBindingContractRevision = "r03a-backend-binding@1"

type BackendBindingParameter struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type BackendBindingContract struct {
	Revision                    string                    `json:"revision"`
	Package                     string                    `json:"package"`
	Entrypoint                  string                    `json:"entrypoint"`
	DeclarationKind             string                    `json:"declaration_kind"`
	Parameters                  []BackendBindingParameter `json:"parameters"`
	ReturnType                  string                    `json:"return_type"`
	ReturnRepresentation        string                    `json:"return_representation,omitempty"`
	ReturnSchemaRef             string                    `json:"return_schema_ref,omitempty"`
	AllowedTopLevelDeclarations []string                  `json:"allowed_top_level_declarations"`
	ImportsAllowed              bool                      `json:"imports_allowed"`
	ReceiverAllowed             bool                      `json:"receiver_allowed"`
}

type BackendBindingActual struct {
	Package         string                    `json:"package,omitempty"`
	Entrypoint      string                    `json:"entrypoint,omitempty"`
	DeclarationKind string                    `json:"declaration_kind,omitempty"`
	Parameters      []BackendBindingParameter `json:"parameters,omitempty"`
	ReturnType      string                    `json:"return_type,omitempty"`
	Imports         []string                  `json:"imports,omitempty"`
	TopLevelKinds   []string                  `json:"top_level_kinds,omitempty"`
}

type BackendBindingReport struct {
	Passed            bool                   `json:"passed"`
	ReasonCode        string                 `json:"reason_code"`
	ActionableSummary string                 `json:"actionable_summary"`
	ContractRevision  string                 `json:"binding_contract_revision"`
	ExpectedBinding   BackendBindingContract `json:"expected_binding"`
	ActualBinding     BackendBindingActual   `json:"actual_binding"`
	ParserDiagnostics []ParserDiagnostic     `json:"parser_diagnostics,omitempty"`
}

func BackendBindingContractV1() BackendBindingContract {
	return BackendBindingContract{
		Revision:        BackendBindingContractRevision,
		Package:         "backend",
		Entrypoint:      "FetchItems",
		DeclarationKind: "function",
		Parameters: []BackendBindingParameter{
			{Name: "cursor", Type: "string"},
			{Name: "limit", Type: "int"},
		},
		ReturnType:                  "string",
		AllowedTopLevelDeclarations: []string{"const"},
		ImportsAllowed:              false,
		ReceiverAllowed:             false,
	}
}

const BackendBindingContractRevisionV2 = "r03a-backend-binding@2"

func BackendBindingContractV2() BackendBindingContract {
	contract := BackendBindingContractV1()
	contract.Revision = BackendBindingContractRevisionV2
	contract.ReturnRepresentation = "utf8_json"
	contract.ReturnSchemaRef = "r03a-pagination-contract@3#response"
	return contract
}

func CheckBackendPublicBinding(source string) BackendBindingReport {
	return checkBackendPublicBinding(source, BackendBindingContractV1())
}

func CheckBackendPublicBindingV2(source string) BackendBindingReport {
	return checkBackendPublicBinding(source, BackendBindingContractV2())
}

func checkBackendPublicBinding(source string, expected BackendBindingContract) BackendBindingReport {
	report := BackendBindingReport{ContractRevision: expected.Revision, ExpectedBinding: expected}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "candidate.go", source, parser.SkipObjectResolution)
	if err != nil {
		report.ReasonCode = "syntax_invalid"
		report.ActionableSummary = "fix the bounded parser diagnostics, then provide the public backend binding"
		report.ParserDiagnostics = publicParserDiagnostics(err)
		return report
	}

	actual := BackendBindingActual{Package: file.Name.Name}
	for _, importSpec := range file.Imports {
		if importSpec.Path != nil {
			actual.Imports = append(actual.Imports, boundedPublicText(importSpec.Path.Value, maxPublicShapeTextRunes))
		}
	}
	sort.Strings(actual.Imports)
	unsupported := false
	for _, declaration := range file.Decls {
		switch declaration := declaration.(type) {
		case *ast.GenDecl:
			kind := declaration.Tok.String()
			actual.TopLevelKinds = append(actual.TopLevelKinds, kind)
			if declaration.Tok != token.CONST {
				unsupported = true
			}
		case *ast.FuncDecl:
			if declaration.Name.Name == expected.Entrypoint {
				actual.Entrypoint = declaration.Name.Name
				actual.DeclarationKind = "function"
				actual.Parameters = bindingParameters(declaration.Type.Params)
				actual.ReturnType = bindingResults(declaration.Type.Results)
				if declaration.Recv != nil {
					actual.DeclarationKind = "method"
					unsupported = true
				}
				if !declaration.Name.IsExported() {
					unsupported = true
				}
				continue
			}
			if actual.Entrypoint == "" {
				actual.Entrypoint = declaration.Name.Name
			}
			actual.TopLevelKinds = append(actual.TopLevelKinds, "func "+declaration.Name.Name)
			unsupported = true
		default:
			actual.TopLevelKinds = append(actual.TopLevelKinds, "unsupported")
			unsupported = true
		}
	}
	sort.Strings(actual.TopLevelKinds)
	report.ActualBinding = actual

	if actual.Package != expected.Package {
		return bindingFailure(report, "package_missing", "declare the candidate in public package backend")
	}
	if len(actual.Imports) > 0 || !expected.ImportsAllowed {
		if len(actual.Imports) > 0 {
			return bindingFailure(report, "unsupported_declaration", "remove imports; the public fixture permits only the declared package and const declarations")
		}
	}
	if actual.DeclarationKind == "" {
		return bindingFailure(report, "entrypoint_missing", "declare public func FetchItems(cursor string, limit int) string")
	}
	if actual.DeclarationKind != expected.DeclarationKind {
		return bindingFailure(report, "declaration_kind_invalid", "declare FetchItems as a top-level public function without a receiver")
	}
	if unsupported {
		return bindingFailure(report, "unsupported_declaration", "remove non-const top-level declarations and additional functions from the public fixture")
	}
	if len(actual.Parameters) != len(expected.Parameters) {
		return bindingFailure(report, "parameter_count_mismatch", "declare exactly two parameters: cursor string and limit int")
	}
	for index, parameter := range expected.Parameters {
		actualParameter := actual.Parameters[index]
		if actualParameter.Type != parameter.Type {
			return bindingFailure(report, "parameter_type_mismatch", "parameter "+parameter.Name+" must have public type "+parameter.Type)
		}
	}
	if actual.ReturnType != expected.ReturnType {
		return bindingFailure(report, "return_type_mismatch", "return exactly one public string response")
	}
	report.Passed = true
	report.ReasonCode = "public_binding_valid"
	report.ActionableSummary = "public backend binding matches r03a-backend-binding@1; continue to pagination behavior checking"
	return report
}

func bindingFailure(report BackendBindingReport, reasonCode, summary string) BackendBindingReport {
	report.Passed = false
	report.ReasonCode = reasonCode
	report.ActionableSummary = summary
	return report
}

func bindingParameters(fields *ast.FieldList) []BackendBindingParameter {
	if fields == nil {
		return nil
	}
	parameters := make([]BackendBindingParameter, 0)
	for _, field := range fields.List {
		typeName := bindingTypeString(field.Type)
		if len(field.Names) == 0 {
			parameters = append(parameters, BackendBindingParameter{Type: typeName})
			continue
		}
		for _, name := range field.Names {
			parameters = append(parameters, BackendBindingParameter{Name: name.Name, Type: typeName})
		}
	}
	return parameters
}

func bindingResults(fields *ast.FieldList) string {
	results := bindingParameters(fields)
	if len(results) == 0 {
		return ""
	}
	parts := make([]string, 0, len(results))
	for _, result := range results {
		parts = append(parts, result.Type)
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

func bindingTypeString(expression ast.Expr) string {
	var buffer bytes.Buffer
	if err := format.Node(&buffer, token.NewFileSet(), expression); err != nil {
		return "unknown"
	}
	return boundedPublicText(buffer.String(), maxPublicShapeTextRunes)
}
