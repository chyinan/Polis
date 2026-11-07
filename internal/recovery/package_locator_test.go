package recovery

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateRecoveryPackageRefUsesExplicitImmutableInputs(t *testing.T) {
	root := t.TempDir()
	dump := filepath.Join(root, "dump")
	casRoot := filepath.Join(root, "cas")
	evidence := filepath.Join(root, "evidence.json")
	packageManifest := filepath.Join(root, "recovery-package-complete.json")
	casManifest := filepath.Join(root, "cas-manifest.json")
	content := []byte("blob")
	d := sha256.Sum256(content)
	digest := hex.EncodeToString(d[:])
	company := "company"
	if err := os.MkdirAll(filepath.Join(casRoot, company), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(casRoot, company, digest), content, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dump, []byte("dump"), 0o600); err != nil {
		t.Fatal(err)
	}
	cas := map[string]any{"entries": []map[string]any{{"company_id": company, "content_sha256": digest, "size": int64(len(content))}}}
	casRaw, _ := json.Marshal(cas)
	if err := os.WriteFile(casManifest, casRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	casHash := sha256.Sum256(casRaw)
	closure := map[string]any{"status": "PASSED", "package_semantic_closure": "PASSED"}
	closureRaw, _ := json.Marshal(closure)
	if err := os.WriteFile(evidence, closureRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	dumpHash := sha256.Sum256([]byte("dump"))
	manifest := map[string]any{"status": "COMPLETE", "package_generation_id": "generation-1", "dump_path": dump, "dump_hash": hex.EncodeToString(dumpHash[:]), "cas_manifest_path": casManifest, "cas_manifest_hash": hex.EncodeToString(casHash[:])}
	manifestRaw, _ := json.Marshal(manifest)
	if err := os.WriteFile(packageManifest, manifestRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	manifestHash := sha256.Sum256(manifestRaw)
	report, err := ValidateRecoveryPackageRef(RecoveryPackageRef{PackageCompleteManifestPath: packageManifest, SemanticClosureEvidencePath: evidence, CASRoot: casRoot, ExpectedPackageGenerationID: "generation-1", ExpectedPackageManifestSHA256: hex.EncodeToString(manifestHash[:]), ExpectedDumpSHA256: hex.EncodeToString(dumpHash[:]), ExpectedCASManifestSHA256: hex.EncodeToString(casHash[:])})
	if err != nil || report.Status != "PASSED" || report.VerifiedBlobCount != 1 {
		t.Fatalf("report=%+v err=%v", report, err)
	}
	base := RecoveryPackageRef{PackageCompleteManifestPath: packageManifest, SemanticClosureEvidencePath: evidence, CASRoot: casRoot, ExpectedPackageGenerationID: "generation-1", ExpectedPackageManifestSHA256: hex.EncodeToString(manifestHash[:]), ExpectedDumpSHA256: hex.EncodeToString(dumpHash[:]), ExpectedCASManifestSHA256: hex.EncodeToString(casHash[:])}
	for name, mutate := range map[string]func(*RecoveryPackageRef){
		"wrong generation":       func(r *RecoveryPackageRef) { r.ExpectedPackageGenerationID = "other" },
		"manifest hash mismatch": func(r *RecoveryPackageRef) { r.ExpectedPackageManifestSHA256 = "bad" },
		"dump hash mismatch":     func(r *RecoveryPackageRef) { r.ExpectedDumpSHA256 = "bad" },
		"CAS hash mismatch":      func(r *RecoveryPackageRef) { r.ExpectedCASManifestSHA256 = "bad" },
		"semantic closure missing": func(r *RecoveryPackageRef) {
			r.SemanticClosureEvidencePath = filepath.Join(root, "missing-closure.json")
		},
	} {
		t.Run(name, func(t *testing.T) {
			candidate := base
			mutate(&candidate)
			if _, err := ValidateRecoveryPackageRef(candidate); err == nil {
				t.Fatal("invalid package reference accepted")
			}
		})
	}
}

func TestValidateRecoveryPackageRefDoesNotDiscoverNearbyPackage(t *testing.T) {
	_, err := ValidateRecoveryPackageRef(RecoveryPackageRef{PackageCompleteManifestPath: "", SemanticClosureEvidencePath: "", CASRoot: "", ExpectedPackageGenerationID: "nearby"})
	if err == nil {
		t.Fatal("missing explicit package ref was accepted")
	}
}
