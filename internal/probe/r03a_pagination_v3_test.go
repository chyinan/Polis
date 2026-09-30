// pattern: Functional Core
package probe

import (
	"polis/internal/codex"
	"strings"
	"testing"
)

func TestR03APaginationV3BackendPromptNamesPublicContractWithoutSourceAnswer(t *testing.T) {
	prompt := r03aPaginationV3BackendPrompt()
	for _, required := range []string{"r03a-pagination-contract@3", "cursor", "limit", "next_cursor", "null", "collab_send"} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("backend prompt omitted public semantic %q: %s", required, prompt)
		}
	}
	for _, forbidden := range []string{"PeerBackendPaginationReference", "func FetchItems", "return \"items,next_cursor\""} {
		if strings.Contains(prompt, forbidden) {
			t.Fatalf("backend prompt leaked fixture implementation %q: %s", forbidden, prompt)
		}
	}
}

func TestR03APaginationV3ResultRequiresAllThreeMediumSessionsBeforePass(t *testing.T) {
	if r03aPaginationV3BusinessPassed(map[string]any{"medium_started": 2}) {
		t.Fatal("two Medium sessions were accepted as a three-session business pass")
	}
	if !r03aPaginationV3BusinessPassed(map[string]any{"medium_started": 3}) {
		t.Fatal("three Medium sessions were not recognized as eligible for finalization")
	}
}

func TestR03APaginationV3BackendOnlyRequiresOneMediumAndNoFrontendQualification(t *testing.T) {
	cfg := R03APaginationV3Config{
		Config:                       Config{Model: "gpt-5.6-luna", MediumLimit: 1, HighLimit: 0, ToolCallLimit: 48, Evidence: "evidence", Root: "runtime", AuthFile: "auth", Binary: "codex", CodeModeHost: "helper", SelectedConfigPath: "config", BlobDurabilityQualificationPath: "blob", CheckerFeedbackQualificationPath: "checker", CurrentL1EvidencePath: "l1", TransportPolicy: codex.DefaultTransportPolicy()},
		ProblemKey:                   R03AProblemKey,
		RunPurpose:                   R03APaginationV3Purpose,
		AcceptanceQualificationPath:  "acceptance",
		BackendExecutionManifestPath: "backend-manifest",
		BackendQualificationPath:     "backend-qualification",
		BackendL2LivePath:            "backend-live",
		AuthorizationBindingPath:     "binding",
		AllowancePath:                "allowance",
		BackendResultPath:            "backend-result",
		PostgresSnapshotPath:         "snapshot",
		ExecutionEnvelopeFingerprint: "envelope",
	}
	if err := validateR03APaginationV3BackendOnlyConfig(cfg); err != nil {
		t.Fatalf("backend-only configuration was rejected: %v", err)
	}
	cfg.MediumLimit = 3
	if err := validateR03APaginationV3BackendOnlyConfig(cfg); err == nil {
		t.Fatal("backend-only configuration accepted three Medium turns")
	}
}
