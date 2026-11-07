// pattern: Imperative Shell
package main

import (
	"flag"
	"fmt"
	"os"

	"polis/internal/probe"
)

func main() {
	evidence := flag.String("evidence", "evidence/development/r0.3a-frontend-checker-feedback-l2", "fresh offline checker-feedback qualification evidence directory")
	previous := flag.String("previous-manifest", "evidence/development/r0.3a-current-binary-frontend-l2/execution-manifest.json", "read-only prior Frontend L2 execution manifest")
	frozen := flag.String("frozen-successor-protocol", "evidence/development/r0.3a-real-frontend-handover-revised/115be34ae3cf91e1a760d8dd8034e3e9/protocol.jsonl", "read-only frozen successor protocol")
	flag.Parse()
	if _, err := probe.RecordR03AFrontendCheckerFeedbackL2Offline(*evidence, *previous, *frozen); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := probe.VerifyR03AFrontendCheckerFeedbackL2Offline(*evidence); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Frontend checker feedback offline qualification passed: %s\n", *evidence)
}
