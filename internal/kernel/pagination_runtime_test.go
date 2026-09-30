// pattern: Imperative Shell
package kernel

import (
	"context"
	"strings"
	"testing"

	"polis/internal/fixture"
)

func TestPaginationWindowsPathToWSLAcceptsExplicitWSLViews(t *testing.T) {
	for _, path := range []string{
		`\\wsl.localhost\Ubuntu-22.04\home\chyinan\cas`,
		`\\wsl$\Ubuntu-22.04\home\chyinan\cas`,
		`/home/chyinan/cas`,
		`D:\Programs\Polis\runtime`,
	} {
		converted, err := paginationWindowsPathToWSL(path)
		if err != nil {
			t.Fatalf("path=%q err=%v", path, err)
		}
		if !strings.HasPrefix(converted, "/") {
			t.Fatalf("path=%q converted=%q", path, converted)
		}
	}
}

func TestPaginationWindowsPathToWSLRejectsImplicitOrRelativePath(t *testing.T) {
	for _, path := range []string{"relative/runtime", "C:relative", ""} {
		if _, err := paginationWindowsPathToWSL(path); err == nil {
			t.Fatalf("invalid path accepted: %q", path)
		}
	}
}

func TestPaginationCandidateSourceRunnerRejectsFrozenH1Candidates(t *testing.T) {
	contract := fixture.MustPeerPaginationContract()
	backend := `package backend
const Endpoint = "GET /items?cursor={cursor}&limit={limit}"
func FetchItems(cursor string, limit int) string {
	_ = cursor
	_ = limit
	return "items,next_cursor"
}
`
	frontend := `package frontend
func ConsumeItems(body string) string {
	_ = body
	return "items; items[].id/name; next_cursor"
}
`
	report, err := VerifyPaginationCandidateSources(context.Background(), t.TempDir(), backend, frontend, contract)
	if err != nil {
		t.Fatal(err)
	}
	if report.Passed {
		t.Fatalf("frozen H1 candidates unexpectedly passed: %+v", report)
	}
}

func TestPaginationCandidateSourceRunnerAcceptsIndependentCorrectReference(t *testing.T) {
	contract := fixture.MustPeerPaginationContract()
	backend := `package backend
const Endpoint = "GET /items?cursor={cursor}&limit={limit}"
func FetchItems(cursor string, limit int) string {
	if limit != 2 { return "{\"items\":[],\"next_cursor\":null}" }
	if cursor == "" { return "{\"items\":[{\"id\":\"i1\",\"name\":\"one\"},{\"id\":\"i2\",\"name\":\"two\"}],\"next_cursor\":\"cursor-2\"}" }
	return "{\"items\":[{\"id\":\"i3\",\"name\":\"three\"}],\"next_cursor\":null}"
}
`
	frontend := `package frontend
func ConsumeItems(body string) string {
	if body == "{\"items\":[{\"id\":\"i1\",\"name\":\"one\"},{\"id\":\"i2\",\"name\":\"two\"}],\"next_cursor\":\"cursor-2\"}" { return "{\"rendered_ids\":[\"i1\",\"i2\"],\"rendered_names\":[\"one\",\"two\"],\"next_cursor\":\"cursor-2\"}" }
	return "{\"rendered_ids\":[\"i3\"],\"rendered_names\":[\"three\"],\"next_cursor\":null}"
}
`
	report, err := VerifyPaginationCandidateSources(context.Background(), t.TempDir(), backend, frontend, contract)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Passed {
		t.Fatalf("correct source reference failed: %+v", report)
	}
}

func TestPaginationCandidateSourceRunnerRejectsUnsafeImports(t *testing.T) {
	contract := fixture.MustPeerPaginationContract()
	unsafe := `package backend
import "os"
func FetchItems(cursor string, limit int) string { _ = os.Args; return "{}" }
`
	if _, err := VerifyPaginationCandidateSources(context.Background(), t.TempDir(), unsafe, unsafe, contract); err == nil {
		t.Fatal("candidate imports were not rejected by the runtime boundary")
	}
}

func TestPaginationBehaviorV2SeparatesRepresentationAndBehaviorFailures(t *testing.T) {
	contract := fixture.MustPeerPaginationContract()
	plain := `package backend
const Endpoint = "GET /items"
func FetchItems(cursor string, limit int) string { return "plain" }
`
	if !fixture.CheckBackendPublicBindingV2(plain).Passed {
		t.Fatal("plain-string fixture should pass source binding")
	}
	representation, err := VerifyPaginationBackendCandidateSourceV2(context.Background(), t.TempDir(), plain, contract)
	if err != nil || representation.Passed || len(representation.Criteria) != 1 || representation.Criteria[0].ReasonCode != "return_representation_invalid" {
		t.Fatalf("plain string was not classified as representation failure: report=%+v err=%v", representation, err)
	}

	wrongShape := `package backend
const Endpoint = "GET /items"
func FetchItems(cursor string, limit int) string {
	if cursor == "" { return "{\"items\":[{\"id\":\"i1\"}],\"next_cursor\":\"cursor-2\"}" }
	return "{\"items\":[{\"id\":\"i3\",\"name\":\"three\"}],\"next_cursor\":null}"
}
`
	shape, err := VerifyPaginationBackendCandidateSourceV2(context.Background(), t.TempDir(), wrongShape, contract)
	if err != nil || shape.Passed {
		t.Fatalf("wrong public response shape unexpectedly passed: report=%+v err=%v", shape, err)
	}
	if shape.Criteria[0].ReasonCode == "return_representation_invalid" {
		t.Fatalf("JSON shape failure was conflated with representation failure: %+v", shape)
	}

	correct, err := VerifyPaginationBackendCandidateSourceV2(context.Background(), t.TempDir(), fixture.PeerBackendPaginationReference, contract)
	if err != nil || !correct.Passed || correct.VerifierRevision != fixture.PeerPaginationBehaviorVerifierRevisionV2 || correct.BindingContractRevision != fixture.BackendBindingContractRevisionV2 {
		t.Fatalf("correct binding and behavior did not pass behavior@2: report=%+v err=%v", correct, err)
	}
}
