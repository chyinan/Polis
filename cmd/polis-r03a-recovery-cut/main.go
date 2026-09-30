// pattern: Imperative Shell
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"polis/internal/kernel"
	"time"
)

func main() {
	dsn := flag.String("dsn", "", "preserved source database DSN")
	root := flag.String("root", "", "preserved CAS blob root")
	output := flag.String("output", "", "recovery-cut evidence directory")
	flag.Parse()
	if *dsn == "" || *root == "" || *output == "" {
		fail("dsn, root and output are required")
	}
	if err := run(*dsn, *root, *output); err != nil {
		fail(err.Error())
	}
}

func run(dsn, root, output string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := os.MkdirAll(output, 0700); err != nil {
		return err
	}
	k, err := kernel.Open(ctx, dsn, root)
	if err != nil {
		return err
	}
	defer k.Close()
	anchors, err := k.PeerRecoveryAnchors(ctx, "")
	if err != nil {
		return err
	}
	b, err := k.BindFake(ctx, k.LocalScope(anchors.CompanyID), "emp-backend")
	if err != nil {
		return err
	}
	// Deliberately pass the stale task-local revision from the original
	// sample-5 orchestration. Backend snapshot eligibility must use persisted
	// anchors instead of this metadata.
	staleHandover := kernel.PeerHandoverBundle{
		EmployeeID:        "emp-backend",
		TaskID:            anchors.BackendTaskID,
		WorkspaceRevision: 1,
	}
	snapshot, err := k.PeerHandoverBoundarySnapshot(ctx, b, staleHandover)
	if err != nil {
		return err
	}
	if snapshot.RecoveryAnchors.ObligationState != "pending" || snapshot.Handover.ObligationState != "pending" || snapshot.Workspace.Revision != snapshot.RecoveryAnchors.BackendWorkspaceRevision {
		return fmt.Errorf("recovery cut did not preserve pending Backend terminal state")
	}
	if err := writeJSON(filepath.Join(output, "authoritative-snapshot.json"), snapshot); err != nil {
		return err
	}
	return writeJSON(filepath.Join(output, "cas-manifest.json"), map[string]any{
		"schema_version":               "r0.3a-pagination-v3-cas-manifest@1",
		"runtime_incarnation":          snapshot.RuntimeIncarnation,
		"entries":                      snapshot.CAS,
		"manifest_digest":              snapshot.CASManifestDigest,
		"historical_evidence_modified": false,
	})
}

func writeJSON(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0600)
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
