// pattern: Functional Core
package recovery

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type RecoveryPackageRef struct {
	PackageCompleteManifestPath   string `json:"package_complete_manifest_path"`
	SemanticClosureEvidencePath   string `json:"semantic_closure_evidence_path"`
	CASRoot                       string `json:"cas_root"`
	RuntimeRoot                   string `json:"runtime_root,omitempty"`
	PackageEvidenceRoot           string `json:"package_evidence_root,omitempty"`
	ExpectedPackageGenerationID   string `json:"expected_package_generation_id"`
	ExpectedPackageManifestSHA256 string `json:"expected_package_manifest_sha256"`
	ExpectedDumpSHA256            string `json:"expected_dump_sha256"`
	ExpectedCASManifestSHA256     string `json:"expected_cas_manifest_sha256"`
}

type RecoveryPackageValidationReport struct {
	Status                string `json:"status"`
	PackageGenerationID   string `json:"package_generation_id"`
	PackageManifestPath   string `json:"package_manifest_path"`
	PackageManifestSHA256 string `json:"package_manifest_sha256"`
	DumpPath              string `json:"dump_path"`
	DumpSHA256            string `json:"dump_sha256"`
	CASManifestPath       string `json:"cas_manifest_path"`
	CASManifestSHA256     string `json:"cas_manifest_sha256"`
	CASRoot               string `json:"cas_root"`
	RequiredBlobCount     int    `json:"required_blob_count"`
	VerifiedBlobCount     int    `json:"verified_blob_count"`
	SemanticClosureStatus string `json:"semantic_closure_status"`
	PackageMutated        bool   `json:"package_mutated"`
	CASMutated            bool   `json:"cas_mutated"`
}

type recoveryPackageManifest struct {
	Status              string `json:"status"`
	PackageGenerationID string `json:"package_generation_id"`
	DumpPath            string `json:"dump_path"`
	DumpHash            string `json:"dump_hash"`
	CASManifestPath     string `json:"cas_manifest_path"`
	CASManifestHash     string `json:"cas_manifest_hash"`
}

type recoveryCASManifest struct {
	Entries []recoveryCASManifestEntry `json:"entries"`
}

type recoveryCASManifestEntry struct {
	CompanyID string `json:"company_id"`
	Digest    string `json:"content_sha256"`
	Size      int64  `json:"size"`
}

var ErrRecoveryPackageInvalid = errors.New("recovery package reference invalid")

func ValidateRecoveryPackageRef(ref RecoveryPackageRef) (RecoveryPackageValidationReport, error) {
	var report RecoveryPackageValidationReport
	if !absolutePath(ref.PackageCompleteManifestPath) || !absolutePath(ref.SemanticClosureEvidencePath) || !absolutePath(ref.CASRoot) || ref.ExpectedPackageGenerationID == "" || ref.ExpectedPackageManifestSHA256 == "" || ref.ExpectedDumpSHA256 == "" || ref.ExpectedCASManifestSHA256 == "" {
		return report, fmt.Errorf("%w: explicit absolute package paths and expected hashes are required", ErrRecoveryPackageInvalid)
	}
	manifestRaw, err := os.ReadFile(ref.PackageCompleteManifestPath)
	if err != nil {
		return report, fmt.Errorf("%w: package manifest unavailable: %v", ErrRecoveryPackageInvalid, err)
	}
	manifestHash := sha256.Sum256(manifestRaw)
	manifestSHA := hex.EncodeToString(manifestHash[:])
	if manifestSHA != ref.ExpectedPackageManifestSHA256 {
		return report, fmt.Errorf("%w: package manifest hash mismatch", ErrRecoveryPackageInvalid)
	}
	var manifest recoveryPackageManifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil || manifest.Status != "COMPLETE" || manifest.PackageGenerationID != ref.ExpectedPackageGenerationID {
		return report, fmt.Errorf("%w: package generation/status mismatch", ErrRecoveryPackageInvalid)
	}
	dumpSHA, err := sha256Path(manifest.DumpPath)
	if err != nil || dumpSHA != ref.ExpectedDumpSHA256 || manifest.DumpHash != ref.ExpectedDumpSHA256 {
		return report, fmt.Errorf("%w: dump hash mismatch", ErrRecoveryPackageInvalid)
	}
	casRaw, err := os.ReadFile(manifest.CASManifestPath)
	if err != nil {
		return report, fmt.Errorf("%w: CAS manifest unavailable: %v", ErrRecoveryPackageInvalid, err)
	}
	casHash := sha256.Sum256(casRaw)
	casSHA := hex.EncodeToString(casHash[:])
	if casSHA != ref.ExpectedCASManifestSHA256 || manifest.CASManifestHash != ref.ExpectedCASManifestSHA256 {
		return report, fmt.Errorf("%w: CAS manifest hash mismatch", ErrRecoveryPackageInvalid)
	}
	var casManifest recoveryCASManifest
	if err := json.Unmarshal(casRaw, &casManifest); err != nil {
		return report, fmt.Errorf("%w: CAS manifest invalid", ErrRecoveryPackageInvalid)
	}
	verified, err := verifyRecoveryPackageCAS(ref.CASRoot, casManifest.Entries)
	if err != nil {
		return report, err
	}
	closureRaw, err := os.ReadFile(ref.SemanticClosureEvidencePath)
	if err != nil {
		return report, fmt.Errorf("%w: semantic closure evidence unavailable: %v", ErrRecoveryPackageInvalid, err)
	}
	var closure map[string]any
	if err := json.Unmarshal(closureRaw, &closure); err != nil || closure["status"] != "PASSED" || closure["package_semantic_closure"] != "PASSED" {
		return report, fmt.Errorf("%w: semantic closure is not passed", ErrRecoveryPackageInvalid)
	}
	report = RecoveryPackageValidationReport{Status: "PASSED", PackageGenerationID: manifest.PackageGenerationID, PackageManifestPath: ref.PackageCompleteManifestPath, PackageManifestSHA256: manifestSHA, DumpPath: manifest.DumpPath, DumpSHA256: dumpSHA, CASManifestPath: manifest.CASManifestPath, CASManifestSHA256: casSHA, CASRoot: ref.CASRoot, RequiredBlobCount: len(casManifest.Entries), VerifiedBlobCount: verified, SemanticClosureStatus: "PASSED", PackageMutated: false, CASMutated: false}
	return report, nil
}

func absolutePath(path string) bool {
	return filepath.IsAbs(path) || (len(path) >= 3 && path[1] == ':' && (path[2] == '\\' || path[2] == '/'))
}

func sha256Path(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:]), nil
}

func verifyRecoveryPackageCAS(root string, entries []recoveryCASManifestEntry) (int, error) {
	count := 0
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		return 0, fmt.Errorf("%w: CAS root unavailable: %v", ErrRecoveryPackageInvalid, err)
	}
	for _, entry := range entries {
		path := filepath.Join(canonical, entry.CompanyID, entry.Digest)
		rel, err := filepath.Rel(canonical, path)
		if err != nil || rel == ".." || len(rel) >= 3 && rel[:3] == ".."+string(filepath.Separator) {
			return count, fmt.Errorf("%w: CAS path escapes root", ErrRecoveryPackageInvalid)
		}
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() || info.Size() != entry.Size {
			return count, fmt.Errorf("%w: CAS blob missing or size mismatch", ErrRecoveryPackageInvalid)
		}
		actual, err := sha256Path(path)
		if err != nil || actual != entry.Digest {
			return count, fmt.Errorf("%w: CAS blob digest mismatch", ErrRecoveryPackageInvalid)
		}
		count++
	}
	return count, nil
}
