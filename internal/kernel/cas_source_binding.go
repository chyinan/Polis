// pattern: Functional Core
package kernel

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const CASLayoutRevision = "r03a-cas-layout@1"

type RuntimeCASBinding struct {
	CanonicalRoot               string `json:"canonical_root"`
	LayoutRevision              string `json:"layout_revision"`
	CompanyNamespace            string `json:"company_namespace"`
	RequiredBlobInventoryDigest string `json:"required_blob_inventory_digest"`
	RequiredBlobCount           int    `json:"required_blob_count"`
}

func (b RuntimeCASBinding) ValidateSyntax() error {
	if b.CanonicalRoot == "" || b.LayoutRevision == "" || b.CompanyNamespace == "" || b.RequiredBlobInventoryDigest == "" || b.RequiredBlobCount <= 0 {
		return errors.New("runtime CAS binding is incomplete")
	}
	if b.LayoutRevision != CASLayoutRevision || !validCASDigest(b.RequiredBlobInventoryDigest) {
		return errors.New("runtime CAS binding revision or inventory digest is invalid")
	}
	return nil
}

func (b RuntimeCASBinding) ValidateReport(report CASSourceBindingReport) error {
	if err := b.ValidateSyntax(); err != nil {
		return err
	}
	if b.CanonicalRoot == "" || b.LayoutRevision == "" || b.CompanyNamespace == "" || b.RequiredBlobInventoryDigest == "" || b.RequiredBlobCount <= 0 {
		return errors.New("runtime CAS binding is incomplete")
	}
	if b.LayoutRevision != report.LayoutRevision || b.CanonicalRoot != report.CanonicalRoot || b.CompanyNamespace != report.CompanyNamespace || b.RequiredBlobCount != report.RequiredBlobCount || b.RequiredBlobInventoryDigest != report.InventoryDigest {
		return fmt.Errorf("runtime CAS binding mismatch: root=%t layout=%t namespace=%t inventory=%t count=%t", b.CanonicalRoot == report.CanonicalRoot, b.LayoutRevision == report.LayoutRevision, b.CompanyNamespace == report.CompanyNamespace, b.RequiredBlobInventoryDigest == report.InventoryDigest, b.RequiredBlobCount == report.RequiredBlobCount)
	}
	return nil
}

type CASRequiredBlob struct {
	CompanyID     string `json:"company_id"`
	ContentSHA256 string `json:"content_sha256"`
	Size          int64  `json:"size"`
}

type CASSourceBindingReport struct {
	CanonicalRoot       string `json:"canonical_root"`
	SentinelKind        string `json:"sentinel_kind"`
	LayoutRevision      string `json:"layout_revision"`
	CompanyNamespace    string `json:"company_namespace"`
	RequiredBlobCount   int    `json:"required_blob_count"`
	PresentBlobCount    int    `json:"present_blob_count"`
	DigestVerifiedCount int    `json:"digest_verified_count"`
	InventoryDigest     string `json:"inventory_digest"`
}

type CASSourceBindingError struct {
	ReasonCode string
	Path       string
}

func (e *CASSourceBindingError) Error() string {
	if e.Path == "" {
		return e.ReasonCode
	}
	return fmt.Sprintf("%s: %s", e.ReasonCode, e.Path)
}

func ValidateCASSourceBinding(root string, required []CASRequiredBlob) (CASSourceBindingReport, error) {
	var report CASSourceBindingReport
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return report, &CASSourceBindingError{ReasonCode: "cas_root_not_found", Path: root}
		}
		return report, err
	}
	info, err := os.Stat(canonical)
	if err != nil {
		return report, err
	}
	if !info.IsDir() {
		return report, &CASSourceBindingError{ReasonCode: "cas_root_not_directory", Path: canonical}
	}
	sentinel := filepath.Join(canonical, ".pagination-runtime")
	sentinelInfo, err := os.Lstat(sentinel)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return report, &CASSourceBindingError{ReasonCode: "cas_layout_sentinel_missing", Path: sentinel}
		}
		return report, err
	}
	if sentinelInfo.Mode()&os.ModeSymlink != 0 || !sentinelInfo.IsDir() {
		return report, &CASSourceBindingError{ReasonCode: "cas_layout_sentinel_wrong_type", Path: sentinel}
	}
	if len(required) == 0 {
		return report, &CASSourceBindingError{ReasonCode: "cas_required_blob_set_empty"}
	}

	report = CASSourceBindingReport{CanonicalRoot: canonical, SentinelKind: "directory", LayoutRevision: CASLayoutRevision, RequiredBlobCount: len(required)}
	if len(required) > 0 {
		report.CompanyNamespace = required[0].CompanyID
	}
	inventory := make([]string, 0, len(required))
	for _, expected := range required {
		if expected.CompanyID != report.CompanyNamespace {
			return report, &CASSourceBindingError{ReasonCode: "cas_company_namespace_mismatch", Path: expected.CompanyID}
		}
		blobPath, reason := casBlobPath(canonical, expected)
		if reason != "" {
			return report, &CASSourceBindingError{ReasonCode: reason, Path: blobPath}
		}
		companyPath := filepath.Dir(blobPath)
		companyInfo, err := os.Lstat(companyPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return report, &CASSourceBindingError{ReasonCode: "cas_company_namespace_missing", Path: companyPath}
			}
			return report, err
		}
		if companyInfo.Mode()&os.ModeSymlink != 0 || !companyInfo.IsDir() {
			return report, &CASSourceBindingError{ReasonCode: "cas_company_namespace_missing", Path: companyPath}
		}
		blobInfo, err := os.Lstat(blobPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return report, &CASSourceBindingError{ReasonCode: "cas_blob_missing", Path: blobPath}
			}
			return report, err
		}
		if blobInfo.Mode()&os.ModeSymlink != 0 || !blobInfo.Mode().IsRegular() {
			return report, &CASSourceBindingError{ReasonCode: "cas_blob_not_regular_file", Path: blobPath}
		}
		if blobInfo.Size() != expected.Size {
			return report, &CASSourceBindingError{ReasonCode: "cas_blob_size_mismatch", Path: blobPath}
		}
		actual, err := sha256File(blobPath)
		if err != nil {
			return report, err
		}
		report.PresentBlobCount++
		if actual != expected.ContentSHA256 {
			return report, &CASSourceBindingError{ReasonCode: "cas_blob_digest_mismatch", Path: blobPath}
		}
		report.DigestVerifiedCount++
		inventory = append(inventory, expected.CompanyID+"/"+expected.ContentSHA256+fmt.Sprintf("/%d", expected.Size))
	}
	sort.Strings(inventory)
	h := sha256.Sum256([]byte(strings.Join(inventory, "\n")))
	report.InventoryDigest = hex.EncodeToString(h[:])
	return report, nil
}

func casBlobPath(canonical string, expected CASRequiredBlob) (string, string) {
	if !validCASDigest(expected.ContentSHA256) {
		return filepath.Join(canonical, expected.CompanyID, expected.ContentSHA256), "cas_path_escape"
	}
	joined := filepath.Join(canonical, expected.CompanyID, expected.ContentSHA256)
	rel, err := filepath.Rel(canonical, joined)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return joined, "cas_path_escape"
	}
	return joined, ""
}

func validCASDigest(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
