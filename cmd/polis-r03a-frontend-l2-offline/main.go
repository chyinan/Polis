// pattern: Imperative Shell
package main

import (
	"flag"
	"fmt"
	"os"
	"polis/internal/probe"
)

func main() {
	evidence := flag.String("evidence", "evidence/development/r0.3a-current-binary-frontend-l2", "new Frontend L2 evidence directory")
	l1Evidence := flag.String("l1-evidence", "evidence/development/r0.3a-current-binary-l1", "sealed current-binary L1 evidence directory")
	baseL2Evidence := flag.String("base-l2-evidence", "evidence/development/r0.3a-current-binary-l2", "sealed current-binary L2 base-factor evidence directory")
	handoverProtocol := flag.String("handover-protocol", "evidence/development/r0.3a-real-frontend-handover-v4/2b7a6fd917d335459b0dbf32dda34158/protocol.jsonl", "read-only historical Frontend surface protocol")
	flag.Parse()

	if _, err := probe.RecordR03AFrontendL2Offline(*evidence, *l1Evidence, *baseL2Evidence, *handoverProtocol); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := probe.VerifyR03AFrontendL2Offline(*evidence); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("current-binary revised Frontend L2 offline qualification passed: %s\n", *evidence)
}
