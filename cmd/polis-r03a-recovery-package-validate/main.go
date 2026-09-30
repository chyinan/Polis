package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"polis/internal/recovery"
)

func main() {
	ref := recovery.RecoveryPackageRef{}
	flag.StringVar(&ref.PackageCompleteManifestPath, "package-complete-manifest", "", "explicit immutable package-complete manifest")
	flag.StringVar(&ref.SemanticClosureEvidencePath, "semantic-closure", "", "explicit semantic closure evidence")
	flag.StringVar(&ref.CASRoot, "cas-root", "", "explicit package CAS root")
	flag.StringVar(&ref.RuntimeRoot, "runtime-root", "", "runtime root metadata only; never used for package discovery")
	flag.StringVar(&ref.PackageEvidenceRoot, "package-evidence-root", "", "package evidence root metadata only")
	flag.StringVar(&ref.ExpectedPackageGenerationID, "expected-generation", "", "expected package generation ID")
	flag.StringVar(&ref.ExpectedPackageManifestSHA256, "expected-package-manifest-sha256", "", "expected package-complete manifest hash")
	flag.StringVar(&ref.ExpectedDumpSHA256, "expected-dump-sha256", "", "expected dump hash")
	flag.StringVar(&ref.ExpectedCASManifestSHA256, "expected-cas-manifest-sha256", "", "expected CAS manifest hash")
	output := flag.String("output", "", "validation output")
	flag.Parse()
	if *output == "" {
		fail("output is required")
	}
	report, err := recovery.ValidateRecoveryPackageRef(ref)
	if err != nil {
		fail(err.Error())
	}
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fail(err.Error())
	}
	if err := os.WriteFile(*output, append(raw, '\n'), 0o600); err != nil {
		fail(err.Error())
	}
}

func fail(message string) { fmt.Fprintln(os.Stderr, message); os.Exit(1) }
