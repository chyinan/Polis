// pattern: Imperative Shell
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"polis/internal/probe"
)

func main() {
	dsn := flag.String("dsn", "", "read-only PostgreSQL DSN")
	bindingOutput := flag.String("binding-output", "", "RuntimeDatabaseBinding output")
	viewOutput := flag.String("view-output", "", "DatabaseAccessView output")
	strategy := flag.String("strategy", probe.WindowsDatabaseAccessStrategyRevision, "database access strategy revision")
	host := flag.String("host", "127.0.0.1", "consumer access host")
	port := flag.Int("port", 55432, "consumer access port")
	distribution := flag.String("target-wsl-distribution", "", "resolved WSL distribution name")
	resolution := flag.String("resolution-method", "", "runtime endpoint resolution method")
	hbaAddress := flag.String("hba-source-address", "", "narrow PostgreSQL host source address")
	flag.Parse()
	if *dsn == "" || *bindingOutput == "" || *viewOutput == "" {
		fail("dsn, binding-output and view-output are required")
	}
	binding, err := probe.DiscoverRuntimeDatabaseBinding(context.Background(), *dsn)
	if err != nil {
		fail(err.Error())
	}
	viewVersion := "r03a-database-access-view@1"
	if *strategy == probe.WindowsWSLDirectTCPStrategyRevision {
		viewVersion = "r03a-database-access-view@2"
	}
	view := probe.DatabaseAccessView{SchemaVersion: viewVersion, ConsumerOS: "windows", Transport: "tcp", Host: *host, Port: *port, EndpointSource: "v7-qualified-wsl-direct-tcp", StrategyRevision: *strategy, RuntimeBindingFingerprint: binding.Fingerprint, TargetWSLDistribution: *distribution, ResolutionMethod: *resolution, HBAAddress: *hbaAddress}
	if err := view.Validate(binding); err != nil {
		fail(err.Error())
	}
	write(*bindingOutput, binding)
	write(*viewOutput, view)
}

func write(path string, value any) {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fail(err.Error())
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0600); err != nil {
		fail(err.Error())
	}
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
