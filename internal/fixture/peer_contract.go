// pattern: Functional Core
package fixture

import (
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"sort"
	"strconv"
	"strings"
)

// PeerContractSpec is the public semantic contract supplied by the kernel to
// the candidate checker. It contains no private verifier implementation.
type PeerContractSpec struct {
	Endpoint               string
	Schema                 string
	ForbidUnexpectedFields bool
}

const (
	maxPublicParserDiagnostics  = 4
	maxPublicParserMessageRunes = 240
	maxPublicShapeFields        = 32
	maxPublicShapeTextRunes     = 128
	maxPublicUnexpectedFields   = 16
	maxPublicFeedbackBytes      = 8192
)

type ParserDiagnostic struct {
	PublicCategory string `json:"public_category"`
	Line           int    `json:"line,omitempty"`
	Column         int    `json:"column,omitempty"`
	Message        string `json:"message"`
}

type PublicTypeMismatch struct {
	Field    string `json:"field"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
}

type PeerCandidateCriterion struct {
	CriterionID               string               `json:"criterion_id"`
	Passed                    bool                 `json:"passed"`
	PublicReasonCode          string               `json:"public_reason_code"`
	ActionableSummary         string               `json:"actionable_summary"`
	ParserDiagnostics         []ParserDiagnostic   `json:"parser_diagnostics,omitempty"`
	ExpectedPublicShape       map[string]string    `json:"expected_public_shape,omitempty"`
	ActualShapeSummary        string               `json:"actual_shape_summary,omitempty"`
	MissingRequiredFields     []string             `json:"missing_required_fields,omitempty"`
	TypeMismatches            []PublicTypeMismatch `json:"type_mismatches,omitempty"`
	UnexpectedFields          []string             `json:"unexpected_fields,omitempty"`
	RelevantContractRevision  string               `json:"-"`
	RelevantWorkspaceRevision int64                `json:"-"`
}

func CandidateCriteriaPassed(criteria []PeerCandidateCriterion) bool {
	if len(criteria) == 0 {
		return false
	}
	for _, criterion := range criteria {
		if !criterion.Passed {
			return false
		}
	}
	return true
}

func CheckPeerBackendCandidate(content string, contract PeerContractSpec) []PeerCandidateCriterion {
	inspection := inspectCandidate(content, "Endpoint", "FetchItems")
	expected := publicContractShape(contract.Schema)
	shape := evaluateResponseShape(inspection.Response, expected, inspection.SyntaxOK, contract.ForbidUnexpectedFields)
	criteria := []PeerCandidateCriterion{
		{
			CriterionID:       "candidate_syntax",
			Passed:            inspection.SyntaxOK,
			PublicReasonCode:  "candidate_syntax_valid",
			ActionableSummary: "provide a compilable backend candidate with the declared endpoint and response function",
			ParserDiagnostics: inspection.ParserDiagnostics,
		},
		{
			CriterionID:       "endpoint_binding",
			Passed:            inspection.SyntaxOK && inspection.Endpoint == contract.Endpoint,
			PublicReasonCode:  "endpoint_matches_accepted_contract",
			ActionableSummary: "declare the endpoint required by the accepted contract revision",
		},
		{
			CriterionID:           "response_shape_compatibility",
			Passed:                inspection.SyntaxOK && shape.Passed,
			PublicReasonCode:      "response_shape_matches_accepted_contract",
			ActionableSummary:     "return all fields required by the accepted contract response schema",
			ExpectedPublicShape:   shape.ExpectedPublicShape,
			ActualShapeSummary:    shape.ActualShapeSummary,
			MissingRequiredFields: shape.MissingRequiredFields,
			TypeMismatches:        shape.TypeMismatches,
			UnexpectedFields:      shape.UnexpectedFields,
		},
	}
	return normalizeCriteria(criteria)
}

func CheckPeerFrontendCandidate(content string, contract PeerContractSpec) []PeerCandidateCriterion {
	inspection := inspectCandidate(content, "", "ConsumeItems")
	shape := evaluateResponseShape(inspection.Response, publicContractShape(contract.Schema), inspection.SyntaxOK, contract.ForbidUnexpectedFields)
	criteria := []PeerCandidateCriterion{
		{
			CriterionID:       "candidate_syntax",
			Passed:            inspection.SyntaxOK,
			PublicReasonCode:  "candidate_syntax_valid",
			ActionableSummary: "provide a compilable frontend candidate with the response consumer function",
			ParserDiagnostics: inspection.ParserDiagnostics,
		},
		{
			CriterionID:           "response_shape_compatibility",
			Passed:                inspection.SyntaxOK && shape.Passed,
			PublicReasonCode:      "response_shape_matches_accepted_contract",
			ActionableSummary:     "consume all fields required by the accepted contract response schema",
			ExpectedPublicShape:   shape.ExpectedPublicShape,
			ActualShapeSummary:    shape.ActualShapeSummary,
			MissingRequiredFields: shape.MissingRequiredFields,
			TypeMismatches:        shape.TypeMismatches,
			UnexpectedFields:      shape.UnexpectedFields,
		},
	}
	return normalizeCriteria(criteria)
}

func normalizeCriteria(criteria []PeerCandidateCriterion) []PeerCandidateCriterion {
	for i := range criteria {
		if !criteria[i].Passed {
			criteria[i].PublicReasonCode = failureReason(criteria[i].CriterionID)
		}
	}
	return criteria
}

func failureReason(criterionID string) string {
	switch criterionID {
	case "candidate_syntax":
		return "candidate_syntax_invalid"
	case "endpoint_binding":
		return "endpoint_mismatch"
	case "response_shape_compatibility":
		return "response_shape_mismatch"
	default:
		return "candidate_contract_incompatible"
	}
}

type candidateInspection struct {
	Endpoint          string
	Response          string
	SyntaxOK          bool
	ParserDiagnostics []ParserDiagnostic
}

func inspectCandidate(content, constName, functionName string) candidateInspection {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "candidate.go", content, parser.SkipObjectResolution)
	if err != nil {
		return candidateInspection{ParserDiagnostics: publicParserDiagnostics(err)}
	}
	inspection := candidateInspection{SyntaxOK: true}
	if constName != "" {
		inspection.Endpoint = stringConstant(file, constName)
		if inspection.Endpoint == "" {
			inspection.SyntaxOK = false
			inspection.ParserDiagnostics = append(inspection.ParserDiagnostics, ParserDiagnostic{PublicCategory: "missing_public_declaration", Line: 1, Column: 1, Message: "required public endpoint constant Endpoint is missing or is not a string constant"})
		}
	}
	inspection.Response = functionReturnConstant(file, functionName)
	if inspection.Response == "" {
		inspection.SyntaxOK = false
		inspection.ParserDiagnostics = append(inspection.ParserDiagnostics, ParserDiagnostic{PublicCategory: "missing_public_declaration", Line: 1, Column: 1, Message: "required public response function return value is missing or is not a string constant"})
	}
	inspection.ParserDiagnostics = normalizeParserDiagnostics(inspection.ParserDiagnostics)
	return inspection
}

func stringConstant(file *ast.File, name string) string {
	for _, declaration := range file.Decls {
		gen, ok := declaration.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, specification := range gen.Specs {
			values, ok := specification.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, identifier := range values.Names {
				if identifier.Name == name && i < len(values.Values) {
					return unquote(values.Values[i])
				}
			}
		}
	}
	return ""
}

func functionReturnConstant(file *ast.File, name string) string {
	var response string
	ast.Inspect(file, func(node ast.Node) bool {
		function, ok := node.(*ast.FuncDecl)
		if !ok || function.Name.Name != name || function.Body == nil {
			return true
		}
		for _, statement := range function.Body.List {
			returnStatement, ok := statement.(*ast.ReturnStmt)
			if !ok || len(returnStatement.Results) == 0 {
				continue
			}
			response = unquote(returnStatement.Results[0])
			return false
		}
		return false
	})
	return response
}

func unquote(expression ast.Expr) string {
	literal, ok := expression.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return ""
	}
	value, err := strconv.Unquote(literal.Value)
	if err != nil {
		return ""
	}
	return value
}

type responseShapeEvaluation struct {
	Passed                bool
	ExpectedPublicShape   map[string]string
	ActualShapeSummary    string
	MissingRequiredFields []string
	TypeMismatches        []PublicTypeMismatch
	UnexpectedFields      []string
}

type actualResponseShape struct {
	Fields map[string]string
}

func evaluateResponseShape(response string, expected map[string]string, syntaxOK, forbidUnexpected bool) responseShapeEvaluation {
	evaluation := responseShapeEvaluation{ExpectedPublicShape: expected}
	if !syntaxOK {
		evaluation.ActualShapeSummary = "unavailable: candidate did not parse"
		return evaluation
	}
	actual := parseResponseShape(response)
	if len(actual.Fields) == 0 {
		evaluation.ActualShapeSummary = "no public response fields declared"
	} else {
		evaluation.ActualShapeSummary = actualResponseShapeSummary(actual)
	}
	fields := sortedShapeFields(expected)
	for _, field := range fields {
		actualType, present := actual.Fields[field]
		if !present {
			evaluation.MissingRequiredFields = append(evaluation.MissingRequiredFields, field)
			continue
		}
		if actualType != "" && !publicTypeCompatible(expected[field], actualType) {
			evaluation.TypeMismatches = append(evaluation.TypeMismatches, PublicTypeMismatch{Field: field, Expected: expected[field], Actual: actualType})
		}
	}
	if forbidUnexpected {
		for field := range actual.Fields {
			if _, present := expected[field]; !present {
				evaluation.UnexpectedFields = append(evaluation.UnexpectedFields, boundedPublicText(field, maxPublicShapeTextRunes))
			}
		}
		sort.Strings(evaluation.UnexpectedFields)
		if len(evaluation.UnexpectedFields) > maxPublicUnexpectedFields {
			evaluation.UnexpectedFields = evaluation.UnexpectedFields[:maxPublicUnexpectedFields]
		}
	}
	evaluation.Passed = len(evaluation.MissingRequiredFields) == 0 && len(evaluation.TypeMismatches) == 0 && len(evaluation.UnexpectedFields) == 0 && len(expected) > 0
	return evaluation
}

func parseResponseShape(response string) actualResponseShape {
	shape := actualResponseShape{Fields: map[string]string{}}
	for _, token := range strings.FieldsFunc(response, func(r rune) bool { return r == ',' || r == ' ' || r == ';' || r == '|' }) {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		field, fieldType := token, ""
		if index := strings.IndexByte(token, ':'); index > 0 {
			field, fieldType = strings.TrimSpace(token[:index]), strings.TrimSpace(token[index+1:])
		}
		if field == "" {
			continue
		}
		field = boundedPublicText(field, maxPublicShapeTextRunes)
		if _, exists := shape.Fields[field]; !exists || shape.Fields[field] == "" {
			shape.Fields[field] = boundedPublicText(fieldType, maxPublicShapeTextRunes)
		}
	}
	return shape
}

func actualResponseShapeSummary(shape actualResponseShape) string {
	fields := make([]string, 0, len(shape.Fields))
	for field, fieldType := range shape.Fields {
		if fieldType == "" {
			fields = append(fields, field)
		} else {
			fields = append(fields, field+":"+fieldType)
		}
	}
	sort.Strings(fields)
	if len(fields) > maxPublicShapeFields {
		fields = fields[:maxPublicShapeFields]
	}
	return "declared response fields: " + strings.Join(fields, ", ")
}

func publicContractShape(schema string) map[string]string {
	var object map[string]json.RawMessage
	if json.Unmarshal([]byte(schema), &object) != nil || len(object) == 0 || len(object) > maxPublicShapeFields {
		return map[string]string{}
	}
	fields := sortedRawMessageFields(object)
	shape := make(map[string]string, len(fields))
	for _, field := range fields {
		shape[field] = publicJSONShape(object[field], 0)
	}
	return shape
}

func sortedRawMessageFields(object map[string]json.RawMessage) []string {
	fields := make([]string, 0, len(object))
	for field := range object {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	return fields
}

func sortedShapeFields(shape map[string]string) []string {
	fields := make([]string, 0, len(shape))
	for field := range shape {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	return fields
}

func publicJSONShape(raw json.RawMessage, depth int) string {
	if depth > 4 {
		return "nested"
	}
	var value any
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if decoder.Decode(&value) != nil {
		return "invalid"
	}
	switch value := value.(type) {
	case string:
		if value == "" {
			return "string"
		}
		return boundedPublicText(value, maxPublicShapeTextRunes)
	case []any:
		if len(value) == 0 {
			return "array"
		}
		encoded, _ := json.Marshal(value[0])
		return "array<" + publicJSONShape(encoded, depth+1) + ">"
	case map[string]any:
		fields := make([]string, 0, len(value))
		for field := range value {
			fields = append(fields, field)
		}
		sort.Strings(fields)
		if len(fields) > maxPublicShapeFields {
			fields = fields[:maxPublicShapeFields]
		}
		parts := make([]string, 0, len(fields))
		for _, field := range fields {
			encoded, _ := json.Marshal(value[field])
			parts = append(parts, field+":"+publicJSONShape(encoded, depth+1))
		}
		return "object{" + strings.Join(parts, ",") + "}"
	case json.Number:
		return "number"
	case bool:
		return "boolean"
	case nil:
		return "null"
	default:
		return "unknown"
	}
}

func publicTypeCompatible(expected, actual string) bool {
	for _, option := range strings.Split(expected, "|") {
		if strings.TrimSpace(option) == actual {
			return true
		}
	}
	return false
}

func publicParserDiagnostics(err error) []ParserDiagnostic {
	var list scanner.ErrorList
	if errors.As(err, &list) {
		diagnostics := make([]ParserDiagnostic, 0, len(list))
		for _, item := range list {
			if item == nil {
				continue
			}
			diagnostics = append(diagnostics, ParserDiagnostic{PublicCategory: "go_parser", Line: item.Pos.Line, Column: item.Pos.Column, Message: boundedPublicText(item.Msg, maxPublicParserMessageRunes)})
		}
		return normalizeParserDiagnostics(diagnostics)
	}
	return []ParserDiagnostic{{PublicCategory: "go_parser", Message: boundedPublicText(err.Error(), maxPublicParserMessageRunes)}}
}

func normalizeParserDiagnostics(diagnostics []ParserDiagnostic) []ParserDiagnostic {
	sort.SliceStable(diagnostics, func(i, j int) bool {
		if diagnostics[i].Line != diagnostics[j].Line {
			return diagnostics[i].Line < diagnostics[j].Line
		}
		if diagnostics[i].Column != diagnostics[j].Column {
			return diagnostics[i].Column < diagnostics[j].Column
		}
		if diagnostics[i].PublicCategory != diagnostics[j].PublicCategory {
			return diagnostics[i].PublicCategory < diagnostics[j].PublicCategory
		}
		return diagnostics[i].Message < diagnostics[j].Message
	})
	if len(diagnostics) > maxPublicParserDiagnostics {
		diagnostics = diagnostics[:maxPublicParserDiagnostics]
	}
	return diagnostics
}

func boundedPublicText(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) > limit {
		return string(runes[:limit])
	}
	return value
}
