// pattern: Imperative Shell
package main

import (
	"flag"
	"fmt"
	"os"

	"polis/internal/probe"
)

func main() {
	evidence := flag.String("evidence", "evidence/development/r0.3a-h1-independent-peer-collaboration-review", "fresh H1 review evidence directory")
	flag.Parse()
	cfg := probe.H1Config{
		Config: probe.Config{
			Binary:                os.Getenv("POLIS_CODEX_BINARY"),
			CodeModeHost:          os.Getenv("POLIS_CODEX_CODE_MODE_HOST"),
			AuthFile:              os.Getenv("POLIS_CODEX_AUTH_FILE"),
			SelectedConfigPath:    os.Getenv("POLIS_SELECTED_CODEX_CONFIG"),
			CurrentL1EvidencePath: os.Getenv("POLIS_CURRENT_L1_EVIDENCE"),
			Root:                  os.Getenv("POLIS_H1_RUNTIME_ROOT"),
			Evidence:              *evidence,
			ProxyURL:              os.Getenv("POLIS_NATIVE_PROXY"),
			ExpectedNativeVersion: "0.154.0-alpha.6.2",
			Model:                 "gpt-5.6-luna",
			MediumLimit:           0,
			HighLimit:             1,
		},
		ProblemKey:            "r03a-real-peer-collaboration-v1",
		SubjectRevision:       "e8c48b1b8536d9c5c55c8469bfed0c60b5cdad0c",
		SourceStatePath:       os.Getenv("POLIS_H1_SOURCE_STATE"),
		FrontendResultPath:    os.Getenv("POLIS_H1_FRONTEND_RESULT"),
		FrontendStatePath:     os.Getenv("POLIS_H1_FRONTEND_STATE"),
		BackendBlobRoot:       os.Getenv("POLIS_H1_BACKEND_BLOBS"),
		FrontendBlobRoot:      os.Getenv("POLIS_H1_FRONTEND_BLOBS"),
		OldWriterEvidencePath: os.Getenv("POLIS_H1_OLD_WRITER_EVIDENCE"),
	}
	result, err := probe.RunR03AH1Review(cfg)
	if result != nil {
		fmt.Printf("r0.3a-h1 status=%v reviewer=%v hidden=%v isolation=%v composite=%v high=%v provider=%v\n", result["status"], result["reviewer_verdict"], result["hidden_verifier_result"], result["review_isolation_result"], result["r0_3a_composite"], result["high_started"], result["provider_egress"])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
