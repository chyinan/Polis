// pattern: Imperative Shell
package main

import (
	"flag"
	"fmt"
	"os"
	"polis/internal/probe"
)

func main() {
	evidence := flag.String("evidence", "evidence/development/r0.3a-t21d", "new T21D evidence directory")
	parent := flag.String("parent", "evidence/development/r0.3a-t21b", "immutable T21B evidence")
	t21c := flag.String("t21c", "evidence/development/r0.3a-t21c", "immutable T21C evidence")
	flag.Parse()
	if _, err := probe.RecordR03AT21DOfflineQualification(*evidence, *parent, *t21c); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := probe.VerifyR03AT21DOfflineQualification(*evidence); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("T21D offline qualification passed: %s\n", *evidence)
}
