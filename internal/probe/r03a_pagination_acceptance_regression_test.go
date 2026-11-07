// pattern: Imperative Shell
package probe

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"polis/internal/fixture"
	"polis/internal/kernel"
)

func TestH1FrozenCandidateHistoricalStaticPassAndBehavioralVerifierFail(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		root = filepath.Dir(root)
	}
	packagePath := filepath.Join(root, "evidence", "development", "r0.3a-h1-independent-peer-collaboration-review", "review-package.json")
	raw, err := os.ReadFile(packagePath)
	if err != nil {
		t.Fatal(err)
	}
	var reviewPackage struct {
		Subject struct {
			Contract struct {
				Endpoint string `json:"endpoint"`
				Schema   string `json:"schema"`
			} `json:"Contract"`
			BackendContent  string `json:"BackendContent"`
			FrontendContent string `json:"FrontendContent"`
		} `json:"subject"`
	}
	if err := json.Unmarshal(raw, &reviewPackage); err != nil {
		t.Fatal(err)
	}
	legacyContract := fixture.PeerContractSpec{Endpoint: reviewPackage.Subject.Contract.Endpoint, Schema: reviewPackage.Subject.Contract.Schema}
	if !fixture.CandidateCriteriaPassed(fixture.CheckPeerBackendCandidate(reviewPackage.Subject.BackendContent, legacyContract)) || !fixture.CandidateCriteriaPassed(fixture.CheckPeerFrontendCandidate(reviewPackage.Subject.FrontendContent, legacyContract)) {
		t.Fatal("frozen H1 subject no longer reproduces the historical static verifier pass")
	}
	behavioral, err := kernel.VerifyPaginationCandidateSources(context.Background(), t.TempDir(), reviewPackage.Subject.BackendContent, reviewPackage.Subject.FrontendContent, fixture.MustPeerPaginationContract())
	if err != nil {
		t.Fatal(err)
	}
	if behavioral.Passed {
		t.Fatalf("new behavioral verifier accepted the frozen H1 candidate: %+v", behavioral)
	}
}
