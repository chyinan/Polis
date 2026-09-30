// pattern: Imperative Shell
package main

import (
	"flag"
	"fmt"
	"os"

	"polis/internal/probe"
)

func main() {
	evidence := flag.String("evidence", "evidence/development/r0.3a-current-binary-l2", "new current-binary L2 evidence directory")
	l1Evidence := flag.String("l1-evidence", "evidence/development/r0.3a-current-binary-l1", "sealed current-binary L1 evidence directory")
	t21dEvidence := flag.String("t21d-evidence", "evidence/development/r0.3a-t21d", "immutable T21D evidence directory")
	flag.Parse()

	if _, err := probe.RecordR03ACurrentBinaryL2Offline(*evidence, *l1Evidence, *t21dEvidence); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := probe.VerifyR03ACurrentBinaryL2Offline(*evidence); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("current-binary revised 11-tool L2 offline qualification passed: %s\n", *evidence)
}
