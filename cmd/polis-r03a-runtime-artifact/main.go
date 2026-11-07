// pattern: Imperative Shell
package main

import (
	"flag"
	"fmt"
	"os"
	"polis/internal/runner"
)

func main() {
	binary := flag.String("binary", "", "source codex.exe")
	helper := flag.String("helper", "", "source codex-code-mode-host.exe")
	destination := flag.String("destination", "", "repo/evidence-external controlled runtime directory")
	version := flag.String("version", "", "version returned by codex.exe --version")
	historicalBinary := flag.String("historical-binary-sha", "", "frozen historical binary SHA256")
	historicalHelper := flag.String("historical-helper-sha", "", "frozen historical helper SHA256")
	sourceClass := flag.String("source-class", "windows_native_controlled_staged", "runtime source classification")
	flag.Parse()
	if *binary == "" || *helper == "" || *destination == "" || *version == "" {
		fmt.Fprintln(os.Stderr, "binary, helper, destination and version are required")
		os.Exit(2)
	}
	manifest, err := runner.StageWindowsRuntimeArtifact(*binary, *helper, *destination, *version, *historicalBinary, *historicalHelper, *sourceClass)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if _, err = runner.LoadAndVerifyWindowsRuntimeArtifact(manifest.ManifestPath); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("controlled Windows runtime artifact staged: %s\n", manifest.ManifestPath)
}
