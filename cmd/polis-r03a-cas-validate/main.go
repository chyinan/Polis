package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"polis/internal/kernel"
	"sort"
)

type snapshot struct {
	CAS []kernel.CASRequiredBlob `json:"cas"`
}

func main() {
	root := flag.String("root", "", "canonical CAS root")
	anchorSnapshot := flag.String("anchor-snapshot", "", "production recovery anchor snapshot containing the required CAS set")
	output := flag.String("output", "", "validation evidence output")
	flag.Parse()
	if *root == "" || *anchorSnapshot == "" || *output == "" {
		fail("root, anchor-snapshot and output are required")
	}
	raw, err := os.ReadFile(*anchorSnapshot)
	if err != nil {
		fail(err.Error())
	}
	var source snapshot
	if err := json.Unmarshal(raw, &source); err != nil {
		fail(err.Error())
	}
	report, err := kernel.ValidateCASSourceBinding(*root, source.CAS)
	if err != nil {
		fail(err.Error())
	}
	companies := map[string]bool{}
	for _, entry := range source.CAS {
		companies[entry.CompanyID] = true
	}
	namespaces := make([]string, 0, len(companies))
	for company := range companies {
		namespaces = append(namespaces, company)
	}
	sort.Strings(namespaces)
	reportValue := map[string]any{"status": "PASSED", "configured_root": *root, "canonical_root": report.CanonicalRoot, "execution_environment": "wsl", "sentinel_kind": report.SentinelKind, "layout_revision": report.LayoutRevision, "company_namespaces": namespaces, "required_blob_count": report.RequiredBlobCount, "present_blob_count": report.PresentBlobCount, "digest_verified_count": report.DigestVerifiedCount, "inventory_digest": report.InventoryDigest, "source": "production_peer_recovery_anchor_snapshot", "historical_evidence_modified": false}
	out, err := json.MarshalIndent(reportValue, "", "  ")
	if err != nil {
		fail(err.Error())
	}
	if err := os.WriteFile(*output, append(out, '\n'), 0o600); err != nil {
		fail(err.Error())
	}
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
