package kernel

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeCASFixture(t *testing.T) (string, CASRequiredBlob) {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".pagination-runtime"), 0o700); err != nil {
		t.Fatal(err)
	}
	company := "company-1"
	content := []byte("sample-cas-content")
	h := sha256.Sum256(content)
	digest := hex.EncodeToString(h[:])
	companyDir := filepath.Join(root, company)
	if err := os.Mkdir(companyDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(companyDir, digest), content, 0o600); err != nil {
		t.Fatal(err)
	}
	return root, CASRequiredBlob{CompanyID: company, ContentSHA256: digest, Size: int64(len(content))}
}

func TestValidateCASSourceBindingAcceptsDirectorySentinelAndExactBlob(t *testing.T) {
	root, required := makeCASFixture(t)
	report, err := ValidateCASSourceBinding(root, []CASRequiredBlob{required})
	if err != nil {
		t.Fatalf("valid CAS rejected: %v", err)
	}
	if report.SentinelKind != "directory" || report.RequiredBlobCount != 1 || report.PresentBlobCount != 1 || report.DigestVerifiedCount != 1 || report.InventoryDigest == "" {
		t.Fatalf("unexpected CAS report: %+v", report)
	}
	binding := RuntimeCASBinding{CanonicalRoot: report.CanonicalRoot, LayoutRevision: report.LayoutRevision, CompanyNamespace: required.CompanyID, RequiredBlobInventoryDigest: report.InventoryDigest, RequiredBlobCount: report.RequiredBlobCount}
	if err := binding.ValidateReport(report); err != nil {
		t.Fatalf("runtime binding rejected exact report: %v", err)
	}
}

func TestValidateCASSourceBindingRejectsLayoutAndBlobFailures(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(string, CASRequiredBlob) []CASRequiredBlob
		want   string
	}{
		{name: "sentinel regular file", mutate: func(root string, required CASRequiredBlob) []CASRequiredBlob {
			os.Remove(filepath.Join(root, ".pagination-runtime"))
			_ = os.WriteFile(filepath.Join(root, ".pagination-runtime"), []byte("wrong"), 0o600)
			return []CASRequiredBlob{required}
		}, want: "cas_layout_sentinel_wrong_type"},
		{name: "sentinel missing", mutate: func(root string, required CASRequiredBlob) []CASRequiredBlob {
			os.RemoveAll(filepath.Join(root, ".pagination-runtime"))
			return []CASRequiredBlob{required}
		}, want: "cas_layout_sentinel_missing"},
		{name: "blob missing", mutate: func(root string, required CASRequiredBlob) []CASRequiredBlob {
			os.Remove(filepath.Join(root, required.CompanyID, required.ContentSHA256))
			return []CASRequiredBlob{required}
		}, want: "cas_blob_missing"},
		{name: "digest mismatch", mutate: func(root string, required CASRequiredBlob) []CASRequiredBlob {
			_ = os.WriteFile(filepath.Join(root, required.CompanyID, required.ContentSHA256), []byte(strings.Repeat("x", int(required.Size))), 0o600)
			return []CASRequiredBlob{required}
		}, want: "cas_blob_digest_mismatch"},
		{name: "company namespace missing", mutate: func(root string, required CASRequiredBlob) []CASRequiredBlob {
			os.RemoveAll(filepath.Join(root, required.CompanyID))
			return []CASRequiredBlob{required}
		}, want: "cas_company_namespace_missing"},
		{name: "path escape", mutate: func(root string, required CASRequiredBlob) []CASRequiredBlob {
			required.CompanyID = "../outside"
			return []CASRequiredBlob{required}
		}, want: "cas_path_escape"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root, required := makeCASFixture(t)
			_, err := ValidateCASSourceBinding(root, tc.mutate(root, required))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v want reason=%s", err, tc.want)
			}
		})
	}
}

func TestValidateCASSourceBindingRejectsMissingOrNonDirectoryRoot(t *testing.T) {
	_, err := ValidateCASSourceBinding(filepath.Join(t.TempDir(), "missing"), nil)
	if err == nil || !strings.Contains(err.Error(), "cas_root_not_found") {
		t.Fatalf("missing root error=%v", err)
	}
	file := filepath.Join(t.TempDir(), "root-file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = ValidateCASSourceBinding(file, nil)
	if err == nil || !strings.Contains(err.Error(), "cas_root_not_directory") {
		t.Fatalf("file root error=%v", err)
	}
}
