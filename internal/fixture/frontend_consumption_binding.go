// pattern: Functional Core
package fixture

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
)

type FrontendBindingReport struct {
	Passed            bool     `json:"passed"`
	ContractRevision  string   `json:"binding_contract_revision"`
	ReasonCode        string   `json:"reason_code"`
	ActionableSummary string   `json:"actionable_summary"`
	ActualPackage     string   `json:"actual_package,omitempty"`
	ActualEntrypoint  string   `json:"actual_entrypoint,omitempty"`
	ActualImports     []string `json:"actual_imports,omitempty"`
}

type FrontendBindingContract struct {
	Revision                   string   `json:"revision"`
	Package                    string   `json:"package"`
	Entrypoint                 string   `json:"entrypoint"`
	ParameterName              string   `json:"parameter_name"`
	ParameterType              string   `json:"parameter_type"`
	ReturnType                 string   `json:"return_type"`
	ReturnRepresentation       string   `json:"return_representation"`
	InputSchemaRef             string   `json:"input_schema_ref"`
	OutputSchemaRef            string   `json:"output_schema_ref"`
	AllowedTopLevelDecls      []string `json:"allowed_top_level_declarations"`
	AllowedImports             []string `json:"allowed_imports"`
}

func FrontendBindingContractV1() FrontendBindingContract {
	return FrontendBindingContract{
		Revision:              FrontendBindingContractRevision,
		Package:               "frontend",
		Entrypoint:            "ConsumeItems",
		ParameterName:         "step_json",
		ParameterType:         "string",
		ReturnType:            "string",
		ReturnRepresentation:  "utf8_json",
		InputSchemaRef:        FrontendConsumptionContractRevision + "#step_input",
		OutputSchemaRef:       FrontendConsumptionContractRevision + "#step_output",
		AllowedTopLevelDecls:  []string{"type", "func ConsumeItems"},
		AllowedImports:        []string{"encoding/json"},
	}
}

func CheckFrontendPublicBindingV1(source string) FrontendBindingReport {
	report := FrontendBindingReport{ContractRevision: FrontendBindingContractRevision}
	file, err := parser.ParseFile(token.NewFileSet(), "candidate.go", []byte(source), parser.SkipObjectResolution)
	if err != nil {
		report.ReasonCode = "syntax_invalid"
		report.ActionableSummary = "provide a parseable package frontend with public ConsumeItems(string) string"
		return report
	}
	report.ActualPackage = file.Name.Name
	if file.Name.Name != "frontend" {
		return frontendBindingFailure(report, "package_invalid", "declare package frontend")
	}
	for _, importSpec := range file.Imports {
		path, err := strconv.Unquote(importSpec.Path.Value)
		if err != nil || path != "encoding/json" {
			report.ActualImports = append(report.ActualImports, importSpec.Path.Value)
			return frontendBindingFailure(report, "import_not_allowed", "only the standard encoding/json import is allowed")
		}
	}
	found := false
	for _, declaration := range file.Decls {
		switch declaration := declaration.(type) {
		case *ast.GenDecl:
			if declaration.Tok != token.TYPE && declaration.Tok != token.IMPORT {
				return frontendBindingFailure(report, "declaration_not_allowed", "only type declarations and ConsumeItems are allowed")
			}
		case *ast.FuncDecl:
			report.ActualEntrypoint = declaration.Name.Name
			if declaration.Name.Name != "ConsumeItems" || declaration.Recv != nil {
				return frontendBindingFailure(report, "entrypoint_invalid", "declare public func ConsumeItems(step_json string) string without a receiver")
			}
			if declaration.Type.Params == nil || len(declaration.Type.Params.List) != 1 || len(declaration.Type.Params.List[0].Names) != 1 || declaration.Type.Params.List[0].Names[0].Name == "" || frontendTypeName(declaration.Type.Params.List[0].Type) != "string" {
				return frontendBindingFailure(report, "parameter_invalid", "ConsumeItems must accept exactly one string step JSON parameter")
			}
			if declaration.Type.Results == nil || len(declaration.Type.Results.List) != 1 || frontendTypeName(declaration.Type.Results.List[0].Type) != "string" {
				return frontendBindingFailure(report, "return_type_invalid", "ConsumeItems must return exactly one string JSON result")
			}
			found = true
		default:
			return frontendBindingFailure(report, "declaration_not_allowed", "only type declarations and ConsumeItems are allowed")
		}
	}
	if !found {
		return frontendBindingFailure(report, "entrypoint_missing", "declare public func ConsumeItems(step_json string) string")
	}
	report.Passed = true
	report.ReasonCode = "public_binding_valid"
	report.ActionableSummary = fmt.Sprintf("public frontend binding matches %s", FrontendBindingContractRevision)
	return report
}

func frontendBindingFailure(report FrontendBindingReport, reason, summary string) FrontendBindingReport {
	report.Passed = false
	report.ReasonCode = reason
	report.ActionableSummary = summary
	return report
}

func frontendTypeName(expression ast.Expr) string {
	if identifier, ok := expression.(*ast.Ident); ok {
		return identifier.Name
	}
	return ""
}
