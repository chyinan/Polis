// pattern: Imperative Shell

package probe

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadR03AT2BlobDurabilityQualificationAcceptsFrozenNestedEvidence(t *testing.T) {
	report, err := LoadR03AT2BlobDurabilityQualification(frozenBlobQualificationPath())
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "PASSED" || report.LoaderSchemaVersion == "" || report.SourceQualificationContentDigest == "" || report.NormalizedContractDigest == "" || report.WindowsSmokeStatus != "PASSED" || report.WindowsParentSyncOutcome != "explicitly_unsupported" {
		t.Fatalf("nested qualification was not normalized and bound: %+v", report)
	}
}

func TestLoadR03AT2BlobDurabilityQualificationRejectsInvalidNestedEvidence(t *testing.T) {
	base := loadFrozenBlobQualificationDocument(t)
	cases := map[string]func(map[string]any){
		"missing durability contract": func(document map[string]any) { delete(document, "durability_contract") },
		"missing Windows smoke":       func(document map[string]any) { delete(document, "windows_native_filesystem_smoke") },
		"status not passed":           func(document map[string]any) { document["status"] = "FAILED" },
		"file sync not required": func(document map[string]any) {
			document["durability_contract"].(map[string]any)["file_content_sync"] = "optional"
		},
		"atomic finalize not required": func(document map[string]any) {
			document["durability_contract"].(map[string]any)["atomic_finalize"] = "best_effort"
		},
		"Windows parent sync claimed Unix equivalent": func(document map[string]any) {
			document["durability_contract"].(map[string]any)["windows_parent_directory_sync"] = "strong_file_sync_atomic_rename_parent_directory_fsync"
		},
		"Windows smoke failed": func(document map[string]any) {
			document["windows_native_filesystem_smoke"].(map[string]any)["status"] = "FAILED"
		},
		"hash incorrect": func(document map[string]any) {
			document["windows_native_filesystem_smoke"].(map[string]any)["content_hash_correct"] = false
		},
		"stage residue present": func(document map[string]any) {
			document["windows_native_filesystem_smoke"].(map[string]any)["stage_files"] = 1
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			document := cloneJSONDocument(t, base)
			mutate(document)
			path := writeBlobQualificationFixture(t, document)
			if _, err := LoadR03AT2BlobDurabilityQualification(path); err == nil {
				t.Fatal("invalid nested qualification was accepted")
			}
		})
	}
}

func TestCompareR03AT2BlobDurabilityQualificationRejectsSourceDigestDrift(t *testing.T) {
	report, err := LoadR03AT2BlobDurabilityQualification(frozenBlobQualificationPath())
	if err != nil {
		t.Fatal(err)
	}
	drifted := report
	drifted.SourceQualificationContentDigest = "drifted"
	if err := CompareR03AT2BlobDurabilityQualification(report, drifted); !errors.Is(err, ErrR03AT2BlobQualificationStale) {
		t.Fatalf("source digest drift was not classified stale: %v", err)
	}
}

func loadFrozenBlobQualificationDocument(t *testing.T) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(frozenBlobQualificationPath())
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	return document
}

func frozenBlobQualificationPath() string {
	return filepath.Join("..", "..", "evidence", "development", "r0.3a-blob-durability", "qualification.json")
}

func cloneJSONDocument(t *testing.T, document map[string]any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	var clone map[string]any
	if err := json.Unmarshal(raw, &clone); err != nil {
		t.Fatal(err)
	}
	return clone
}

func writeBlobQualificationFixture(t *testing.T, document map[string]any) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "qualification.json")
	raw, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
