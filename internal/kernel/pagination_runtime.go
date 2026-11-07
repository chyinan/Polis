// pattern: Imperative Shell
package kernel

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"polis/internal/fixture"
)

const paginationCandidateRuntimeTimeout = 30 * time.Second

type PaginationRuntimePathError struct {
	ReasonCode string
	Path       string
}

func (e *PaginationRuntimePathError) Error() string {
	return fmt.Sprintf("%s: %s", e.ReasonCode, e.Path)
}

func ValidatePaginationRuntimeAccess(root string, required []CASRequiredBlob) (CASSourceBindingReport, error) {
	if root == "" {
		return CASSourceBindingReport{}, &PaginationRuntimePathError{ReasonCode: "runtime_path_binding_invalid", Path: root}
	}
	report, err := ValidateCASSourceBinding(root, required)
	if err != nil {
		return report, &PaginationRuntimePathError{ReasonCode: "runtime_resource_unavailable", Path: err.Error()}
	}
	return report, nil
}

// VerifyPaginationCandidateSources executes the frozen candidate sources in a
// bounded, import-free candidate process and feeds its observable values into
// the pure pagination behavior verifier. The source is never executed in the
// kernel process and candidate imports are rejected before a process starts.
func VerifyPaginationCandidateSources(ctx context.Context, root, backendSource, frontendSource string, contract fixture.PaginationBehaviorContract) (fixture.PaginationBehaviorReport, error) {
	if err := validatePaginationCandidateSource(backendSource, "backend", "FetchItems", true); err != nil {
		return fixture.PaginationBehaviorReport{}, err
	}
	if err := validatePaginationCandidateSource(frontendSource, "frontend", "ConsumeItems", false); err != nil {
		return fixture.PaginationBehaviorReport{}, err
	}
	if len(contract.ProbeCases) < 2 {
		return fixture.PaginationBehaviorReport{}, errors.New("pagination behavior contract has insufficient probe cases")
	}

	root, err := filepath.Abs(root)
	if err != nil {
		return fixture.PaginationBehaviorReport{}, fmt.Errorf("resolve pagination runtime root: %w", err)
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return fixture.PaginationBehaviorReport{}, fmt.Errorf("create pagination runtime root: %w", err)
	}
	runtimeRoot, err := os.MkdirTemp(root, "pagination-runtime-")
	if err != nil {
		return fixture.PaginationBehaviorReport{}, fmt.Errorf("create pagination candidate runtime: %w", err)
	}
	defer os.RemoveAll(runtimeRoot)

	backendLines, backendFailed, err := runPaginationCandidate(ctx, runtimeRoot, backendSource, "backend", backendHarness(contract))
	if err != nil {
		return fixture.PaginationBehaviorReport{}, err
	}
	if backendFailed {
		return paginationRuntimeFailure("backend_behavior_execution", "backend candidate did not produce executable public pagination behavior"), nil
	}
	responses, err := decodeBackendResponses(backendLines)
	if err != nil {
		return paginationRuntimeFailure("backend_response_shape", "backend candidate did not return a structured public pagination response"), nil
	}
	if len(responses) != len(contract.ProbeCases) {
		return paginationRuntimeFailure("backend_probe_cardinality", "backend candidate did not produce one response for each public pagination probe"), nil
	}

	frontendBodies := make([]string, 0, len(backendLines))
	for _, line := range backendLines {
		var body string
		if err := json.Unmarshal([]byte(line), &body); err != nil {
			return paginationRuntimeFailure("backend_response_encoding", "backend candidate output was not a public response string"), nil
		}
		frontendBodies = append(frontendBodies, body)
	}
	frontendLines, frontendFailed, err := runPaginationCandidate(ctx, runtimeRoot, frontendSource, "frontend", frontendHarness(frontendBodies))
	if err != nil {
		return fixture.PaginationBehaviorReport{}, err
	}
	if frontendFailed {
		return paginationRuntimeFailure("frontend_behavior_execution", "frontend candidate did not produce executable public pagination behavior"), nil
	}
	pages, err := decodeFrontendPages(frontendLines)
	if err != nil || len(pages) != len(responses) {
		return paginationRuntimeFailure("frontend_response_shape", "frontend candidate did not return a structured public page result"), nil
	}

	backendCall := 0
	backend := fixture.PaginationBackendFunc(func(cursor *string, limit int) fixture.PaginationResponse {
		if backendCall >= len(responses) {
			return fixture.PaginationResponse{}
		}
		response := responses[backendCall]
		backendCall++
		return response
	})
	frontendCall := 0
	frontend := fixture.PaginationFrontendFunc(func(response fixture.PaginationResponse) fixture.FrontendPage {
		if frontendCall >= len(pages) {
			return fixture.FrontendPage{}
		}
		page := pages[frontendCall]
		frontendCall++
		return page
	})
	return fixture.VerifyPaginationBehavior(contract, backend, frontend), nil
}

func VerifyPaginationBackendCandidateSource(ctx context.Context, root, source string, contract fixture.PaginationBehaviorContract) (fixture.PaginationBehaviorReport, error) {
	if err := validatePaginationCandidateSource(source, "backend", "FetchItems", true); err != nil {
		return fixture.PaginationBehaviorReport{}, err
	}
	lines, failed, err := runPaginationCandidate(ctx, root, source, "backend", backendHarness(contract))
	if err != nil {
		return fixture.PaginationBehaviorReport{}, err
	}
	if failed {
		return paginationRuntimeFailure("backend_behavior_execution", "backend candidate did not produce executable public pagination behavior"), nil
	}
	responses, err := decodeBackendResponses(lines)
	if err != nil {
		return paginationRuntimeFailure("backend_response_shape", "backend candidate did not return a structured public pagination response"), nil
	}
	return fixture.VerifyPaginationBackendResponses(contract, responses), nil
}

func VerifyPaginationBackendCandidateSourceV2(ctx context.Context, root, source string, contract fixture.PaginationBehaviorContract) (fixture.PaginationBehaviorReport, error) {
	base := fixture.PaginationBehaviorReport{BusinessContractRevision: fixture.PeerPaginationContractRevision, BindingContractRevision: fixture.BackendBindingContractRevisionV2, VerifierRevision: fixture.PeerPaginationBehaviorVerifierRevisionV2}
	binding := fixture.CheckBackendPublicBindingV2(source)
	if !binding.Passed {
		base.Criteria = []fixture.PaginationBehaviorCriterion{{
			CriterionID:             "public_binding_check",
			Passed:                  false,
			ReasonCode:              binding.ReasonCode,
			Reason:                  binding.ActionableSummary,
			BindingContractRevision: fixture.BackendBindingContractRevisionV2,
			VerifierRevision:        fixture.PeerPaginationBehaviorVerifierRevisionV2,
		}}
		return base, nil
	}
	lines, failed, err := runPaginationCandidate(ctx, root, source, "backend", backendHarness(contract))
	if err != nil {
		return fixture.PaginationBehaviorReport{}, err
	}
	if failed {
		base.Criteria = []fixture.PaginationBehaviorCriterion{{
			CriterionID:             "public_return_representation",
			Passed:                  false,
			ReasonCode:              "return_representation_invalid",
			Reason:                  "FetchItems must return UTF-8 JSON text that can be decoded against the public response schema",
			ExpectedRepresentation:  "utf8_json",
			BindingContractRevision: fixture.BackendBindingContractRevisionV2,
			VerifierRevision:        fixture.PeerPaginationBehaviorVerifierRevisionV2,
		}}
		return base, nil
	}
	responses, err := decodeBackendResponses(lines)
	if err != nil {
		base.Criteria = []fixture.PaginationBehaviorCriterion{{
			CriterionID:             "public_return_representation",
			Passed:                  false,
			ReasonCode:              "return_representation_invalid",
			Reason:                  "FetchItems must return UTF-8 JSON text that can be decoded against the public response schema",
			ExpectedRepresentation:  "utf8_json",
			BindingContractRevision: fixture.BackendBindingContractRevisionV2,
		}}
		return base, nil
	}
	if len(responses) != len(contract.ProbeCases) {
		base.Criteria = []fixture.PaginationBehaviorCriterion{{
			CriterionID: "backend_probe_cardinality", Passed: false, ReasonCode: "pagination_behavior_mismatch",
			Reason:           "backend did not produce one response for each public pagination probe",
			VerifierRevision: fixture.PeerPaginationBehaviorVerifierRevisionV2,
		}}
		return base, nil
	}
	behavior := fixture.VerifyPaginationBackendResponses(contract, responses)
	behavior.BusinessContractRevision = fixture.PeerPaginationContractRevision
	behavior.BindingContractRevision = fixture.BackendBindingContractRevisionV2
	behavior.VerifierRevision = fixture.PeerPaginationBehaviorVerifierRevisionV2
	for i := range behavior.Criteria {
		behavior.Criteria[i].VerifierRevision = fixture.PeerPaginationBehaviorVerifierRevisionV2
		if behavior.Criteria[i].ReasonCode != "" {
			continue
		}
		behavior.Criteria[i].ReasonCode = "pagination_behavior_mismatch"
		if behavior.Criteria[i].Passed {
			behavior.Criteria[i].ReasonCode = "pagination_behavior_matches_public_contract"
		}
	}
	return behavior, nil
}

func VerifyPaginationFrontendCandidateSource(ctx context.Context, root, source string, contract fixture.PaginationBehaviorContract) (fixture.PaginationBehaviorReport, error) {
	if err := validatePaginationCandidateSource(source, "frontend", "ConsumeItems", false); err != nil {
		return fixture.PaginationBehaviorReport{}, err
	}
	responses := make([]fixture.PaginationResponse, 0, len(contract.ProbeCases))
	bodies := make([]string, 0, len(contract.ProbeCases))
	for _, probe := range contract.ProbeCases {
		response := fixture.PaginationResponse{Items: probe.ExpectedItems, NextCursor: probe.ExpectedNextCursor}
		responses = append(responses, response)
		raw, err := json.Marshal(response)
		if err != nil {
			return fixture.PaginationBehaviorReport{}, fmt.Errorf("encode public frontend probe: %w", err)
		}
		bodies = append(bodies, string(raw))
	}
	lines, failed, err := runPaginationCandidate(ctx, root, source, "frontend", frontendHarness(bodies))
	if err != nil {
		return fixture.PaginationBehaviorReport{}, err
	}
	if failed {
		return paginationRuntimeFailure("frontend_behavior_execution", "frontend candidate did not produce executable public pagination behavior"), nil
	}
	pages, err := decodeFrontendPages(lines)
	if err != nil {
		return paginationRuntimeFailure("frontend_response_shape", "frontend candidate did not return a structured public page result"), nil
	}
	return fixture.VerifyPaginationFrontendPages(contract, responses, pages), nil
}

func validatePaginationCandidateSource(source, packageName, functionName string, backend bool) error {
	if len(source) == 0 || len(source) > 16384 {
		return errors.New("pagination candidate source is empty or too large")
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "candidate.go", source, parser.SkipObjectResolution)
	if err != nil {
		return fmt.Errorf("pagination candidate source is not parseable: %w", err)
	}
	if file.Name.Name != packageName || len(file.Imports) != 0 {
		return errors.New("pagination candidate runtime accepts only the declared package without imports")
	}
	foundFunction := false
	for _, declaration := range file.Decls {
		switch declaration := declaration.(type) {
		case *ast.GenDecl:
			if declaration.Tok != token.CONST || !backend {
				return errors.New("pagination candidate contains an unsupported declaration")
			}
		case *ast.FuncDecl:
			if declaration.Name.Name != functionName || declaration.Recv != nil {
				return errors.New("pagination candidate function binding does not match the public runtime entrypoint")
			}
			foundFunction = true
		default:
			return errors.New("pagination candidate contains an unsupported declaration")
		}
	}
	if !foundFunction {
		return errors.New("pagination candidate public runtime entrypoint is missing")
	}
	return nil
}

func backendHarness(contract fixture.PaginationBehaviorContract) string {
	var calls []string
	for _, probe := range contract.ProbeCases {
		cursor := ""
		if probe.Cursor != nil {
			cursor = *probe.Cursor
		}
		calls = append(calls, "FetchItems("+strconv.Quote(cursor)+", "+strconv.Itoa(probe.Limit)+")")
	}
	return "package main\n\nimport (\n\t\"encoding/json\"\n\t\"os\"\n)\n\nfunc main() {\n\tvalues := []string{" + strings.Join(calls, ",") + "}\n\tencoder := json.NewEncoder(os.Stdout)\n\tfor _, value := range values {\n\t\t_ = encoder.Encode(value)\n\t}\n}\n"
}

func frontendHarness(bodies []string) string {
	quoted := make([]string, 0, len(bodies))
	for _, body := range bodies {
		quoted = append(quoted, strconv.Quote(body))
	}
	return "package main\n\nimport (\n\t\"encoding/json\"\n\t\"os\"\n)\n\nfunc main() {\n\tbodies := []string{" + strings.Join(quoted, ",") + "}\n\tencoder := json.NewEncoder(os.Stdout)\n\tfor _, body := range bodies {\n\t\t_ = encoder.Encode(ConsumeItems(body))\n\t}\n}\n"
}

func runPaginationCandidate(ctx context.Context, root, source, packageName, harness string) ([]string, bool, error) {
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, false, fmt.Errorf("create pagination runtime root: %w", err)
	}
	dir, err := os.MkdirTemp(root, packageName+"-")
	if err != nil {
		return nil, false, fmt.Errorf("create %s candidate runtime: %w", packageName, err)
	}
	defer os.RemoveAll(dir)
	runtimeSource := strings.Replace(source, "package "+packageName, "package main", 1)
	if err := os.WriteFile(filepath.Join(dir, "candidate.go"), []byte(runtimeSource), 0600); err != nil {
		return nil, false, fmt.Errorf("write %s candidate runtime: %w", packageName, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "harness.go"), []byte(harness), 0600); err != nil {
		return nil, false, fmt.Errorf("write %s pagination harness: %w", packageName, err)
	}
	goBinary, err := paginationGoBinary()
	if err != nil {
		return nil, false, err
	}
	runCtx, cancel := context.WithTimeout(ctx, paginationCandidateRuntimeTimeout)
	defer cancel()
	command, err := paginationCandidateCommand(runCtx, goBinary, dir)
	if err != nil {
		return nil, false, err
	}
	command.Env = paginationCandidateEnvironment()
	output, err := command.Output()
	if err != nil {
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			return nil, true, nil
		}
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return nil, true, nil
		}
		return nil, false, fmt.Errorf("run %s pagination candidate: %w", packageName, err)
	}
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	lines := make([]string, 0, 4)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, false, fmt.Errorf("read %s pagination output: %w", packageName, err)
	}
	return lines, false, nil
}

func paginationCandidateCommand(ctx context.Context, goBinary, directory string) (*exec.Cmd, error) {
	if runtime.GOOS != "windows" {
		command := exec.CommandContext(ctx, goBinary, "run", "candidate.go", "harness.go")
		command.Dir = directory
		return command, nil
	}
	wslDirectory, err := paginationWindowsPathToWSL(directory)
	if err != nil {
		return nil, err
	}
	wslGoBinary := goBinary
	if len(wslGoBinary) >= 3 && wslGoBinary[1] == ':' {
		wslGoBinary, err = paginationWindowsPathToWSL(wslGoBinary)
		if err != nil {
			return nil, err
		}
	}
	return exec.CommandContext(ctx, "wsl.exe", "--cd", wslDirectory, "--", "env", "GO111MODULE=off", "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", wslGoBinary, "run", "candidate.go", "harness.go"), nil
}

func paginationWindowsPathToWSL(path string) (string, error) {
	if strings.HasPrefix(path, "/") {
		return filepath.ToSlash(path), nil
	}
	normalized := strings.ReplaceAll(path, "/", "\\")
	if strings.HasPrefix(normalized, `\\wsl.localhost\`) || strings.HasPrefix(normalized, `\\wsl$\`) {
		prefixEnd := strings.Index(normalized[len(`\\wsl.localhost\`):], "\\")
		if strings.HasPrefix(normalized, `\\wsl$\`) {
			prefixEnd = strings.Index(normalized[len(`\\wsl$\`):], "\\")
		}
		if prefixEnd < 0 {
			return "", &PaginationRuntimePathError{ReasonCode: "runtime_path_binding_invalid", Path: path}
		}
		base := len(`\\wsl.localhost\`)
		if strings.HasPrefix(normalized, `\\wsl$\`) {
			base = len(`\\wsl$\`)
		}
		return "/" + strings.ReplaceAll(normalized[base+prefixEnd+1:], "\\", "/"), nil
	}
	if len(path) < 3 || path[1] != ':' || (path[2] != '\\' && path[2] != '/') {
		return "", &PaginationRuntimePathError{ReasonCode: "runtime_path_binding_invalid", Path: path}
	}
	return "/mnt/" + strings.ToLower(string(path[0])) + "/" + strings.ReplaceAll(path[2:], "\\", "/"), nil
}

func paginationCandidateEnvironment() []string {
	env := make([]string, 0, len(os.Environ())+4)
	for _, value := range os.Environ() {
		name, _, found := strings.Cut(value, "=")
		if found && (name == "GO111MODULE" || name == "GOTOOLCHAIN" || name == "GOPROXY" || name == "GOSUMDB") {
			continue
		}
		env = append(env, value)
	}
	return append(env, "GO111MODULE=off", "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off")
}

func decodeBackendResponses(lines []string) ([]fixture.PaginationResponse, error) {
	responses := make([]fixture.PaginationResponse, 0, len(lines))
	for _, line := range lines {
		var body string
		if err := json.Unmarshal([]byte(line), &body); err != nil {
			return nil, err
		}
		var response fixture.PaginationResponse
		if err := json.Unmarshal([]byte(body), &response); err != nil {
			return nil, err
		}
		responses = append(responses, response)
	}
	return responses, nil
}

func decodeFrontendPages(lines []string) ([]fixture.FrontendPage, error) {
	pages := make([]fixture.FrontendPage, 0, len(lines))
	for _, line := range lines {
		var body string
		if err := json.Unmarshal([]byte(line), &body); err != nil {
			return nil, err
		}
		var page fixture.FrontendPage
		if err := json.Unmarshal([]byte(body), &page); err != nil {
			return nil, err
		}
		pages = append(pages, page)
	}
	return pages, nil
}

func paginationRuntimeFailure(id, reason string) fixture.PaginationBehaviorReport {
	return fixture.PaginationBehaviorReport{Passed: false, Criteria: []fixture.PaginationBehaviorCriterion{{CriterionID: id, Passed: false, Reason: reason}}}
}

func paginationGoBinary() (string, error) {
	if explicit := os.Getenv("POLIS_GO_BINARY"); explicit != "" {
		return explicit, nil
	}
	if goRoot := os.Getenv("POLIS_GO_ROOT"); goRoot != "" {
		candidate := filepath.Join(goRoot, "bin", "go")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		if runtime.GOOS == "windows" {
			return candidate, nil
		}
	}
	workingDirectory, err := os.Getwd()
	if err == nil {
		candidate := filepath.Join(workingDirectory, ".tools", "go", "bin", "go")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	if runtime.GOOS == "windows" {
		return "/mnt/d/Programs/Polis/.tools/go/bin/go", nil
	}
	goBinary, err := exec.LookPath("go")
	if err != nil {
		return "", errors.New("pagination behavior verifier requires a Go runtime")
	}
	return goBinary, nil
}
