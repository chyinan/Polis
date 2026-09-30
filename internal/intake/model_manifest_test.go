// pattern: Functional Core
package intake

import (
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"strings"
	"testing"
)

func TestPrepareModelInputManifestSelectsLatestRevisionAndSortsCandidates(t *testing.T) {
	inputs := []MissionInputReference{
		{InputID: "z-input", Revision: 1, RequestID: "z-1", SourceKind: "upload", DisplayName: "z.md", MediaType: "text/markdown", ByteSize: 5, ContentDigest: digestForTest("z-old"), State: StateUsable},
		{InputID: "a-input", Revision: 1, RequestID: "a-1", SourceKind: "upload", DisplayName: "a.md", MediaType: "text/markdown", ByteSize: 5, ContentDigest: digestForTest("a"), State: StateUsable},
		{InputID: "z-input", Revision: 2, RequestID: "z-2", SourceKind: "upload", DisplayName: "z.md", MediaType: "text/markdown", ByteSize: 8, ContentDigest: digestForTest("z-new"), State: StatePartial},
	}
	manifest, digest, err := PrepareModelInputManifest("company-1", "mission-1", "task-1", inputs)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != ModelInputManifestSchema || manifest.DeliveryStatus != "not_delivered" || len(manifest.CandidateInputs) != 1 || len(manifest.ExcludedInputs) != 1 {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	if manifest.CandidateInputs[0].InputID != "a-input" || manifest.ExcludedInputs[0].InputID != "z-input" || manifest.ExcludedInputs[0].Revision != 2 {
		t.Fatalf("manifest did not choose latest revisions deterministically: %+v", manifest)
	}
	reversed, reversedDigest, err := PrepareModelInputManifest("company-1", "mission-1", "task-1", []MissionInputReference{inputs[2], inputs[0], inputs[1]})
	if err != nil {
		t.Fatal(err)
	}
	if digest != reversedDigest || !reflect.DeepEqual(manifest, reversed) {
		t.Fatal("manifest changed when input metadata order changed")
	}
	if err = VerifyModelInputManifest(manifest, digest); err != nil {
		t.Fatalf("manifest digest failed verification: %v", err)
	}
}

func TestPrepareModelInputManifestRejectsConflictingOrMalformedRevisions(t *testing.T) {
	base := MissionInputReference{InputID: "input-1", Revision: 1, RequestID: "request-1", SourceKind: "upload", DisplayName: "goal.md", MediaType: "text/markdown", ByteSize: 8, ContentDigest: digestForTest("goal"), State: StateUsable}
	for name, inputs := range map[string][]MissionInputReference{
		"same-revision-conflict": {base, base},
		"digest-invalid":         {{InputID: "input-1", Revision: 1, RequestID: "request-1", SourceKind: "upload", DisplayName: "goal.md", MediaType: "text/markdown", ByteSize: 8, ContentDigest: "bad", State: StateUsable}},
		"source-invalid":         {{InputID: "input-1", Revision: 1, RequestID: "request-1", SourceKind: "mcp", DisplayName: "goal.md", MediaType: "text/markdown", ByteSize: 8, ContentDigest: digestForTest("goal"), State: StateUsable}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := PrepareModelInputManifest("company-1", "mission-1", "task-1", inputs); err == nil {
				t.Fatal("accepted malformed input revisions")
			}
		})
	}
}

func TestVerifyModelInputManifestRejectsDigestDrift(t *testing.T) {
	manifest, digest, err := PrepareModelInputManifest("company-1", "mission-1", "task-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	manifest.DeliveryStatus = "delivered"
	if err = VerifyModelInputManifest(manifest, digest); err == nil {
		t.Fatal("accepted manifest with changed delivery status and stale digest")
	}
}

func TestPrepareModelInputContextDeliversOnlyBoundTextWithTrustAndDigestMarkers(t *testing.T) {
	text := []byte("POLIS_BOUND_INPUT_SENTINEL_2026\n")
	directory, err := PrepareDirectorySnapshot([]DirectoryInputFile{
		{RelativePath: "project/README.md", MediaType: "text/markdown", Content: []byte("directory source text\n")},
		{RelativePath: "project/readme.bin", MediaType: "application/octet-stream", Content: []byte("opaque")},
	})
	if err != nil {
		t.Fatal(err)
	}
	manifest, digest, err := PrepareModelInputManifest("company-1", "mission-1", "task-1", []MissionInputReference{
		{InputID: "text-input", Revision: 2, RequestID: "text-request", SourceKind: "upload", DisplayName: "goal.md", MediaType: "text/markdown", ByteSize: int64(len(text)), ContentDigest: digestForTest(string(text)), State: StateUsable},
		{InputID: "directory-input", Revision: 1, RequestID: "directory-request", SourceKind: "directory_snapshot", DisplayName: directory.RootName, MediaType: directory.Upload.MediaType, ByteSize: directory.Upload.ByteSize, ContentDigest: directory.Upload.ContentDigest, State: directory.Upload.State},
	})
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := PrepareModelInputContext(manifest, digest, map[string][]byte{"text-input": text, "directory-input": directory.Archive})
	if err != nil {
		t.Fatal(err)
	}
	directTextIncluded, directoryTextIncluded := false, false
	for _, item := range prepared.Inputs {
		if item.Reference.InputID == "text-input" && item.Reference.Revision == 2 && item.Text == string(text) {
			directTextIncluded = true
		}
		if item.Reference.InputID == "directory-input" && item.RelativePath == "project/README.md" && item.Text == "directory source text\n" {
			directoryTextIncluded = true
		}
	}
	if len(prepared.Inputs) != 2 || !directTextIncluded || !directoryTextIncluded || len(prepared.Excluded) != 1 || prepared.Excluded[0].InputID != "directory-input" || prepared.Excluded[0].RelativePath != "project/readme.bin" {
		t.Fatalf("model input context selection = %+v", prepared)
	}
	for _, required := range []string{digest, "untrusted user input", "POLIS_BOUND_INPUT_SENTINEL_2026", "project/README.md", "directory source text", "directory-input/project/readme.bin (representation_not_supported)"} {
		if !strings.Contains(prepared.PromptSection, required) {
			t.Fatalf("model input prompt omitted %q: %s", required, prepared.PromptSection)
		}
	}
	if len(prepared.PayloadDigest) != 64 {
		t.Fatalf("model input payload digest is invalid: %q", prepared.PayloadDigest)
	}
}

func TestPrepareModelInputContextRejectsContentDriftAndBoundsLargeText(t *testing.T) {
	content := []byte("verified")
	manifest, digest, err := PrepareModelInputManifest("company-1", "mission-1", "task-1", []MissionInputReference{{
		InputID: "text-input", Revision: 1, RequestID: "text-request", SourceKind: "upload", DisplayName: "goal.txt", MediaType: "text/plain", ByteSize: int64(len(content)), ContentDigest: digestForTest(string(content)), State: StateUsable,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = PrepareModelInputContext(manifest, digest, map[string][]byte{"text-input": []byte("drifted")}); err == nil {
		t.Fatal("accepted content that does not match the frozen digest")
	}
	large := []byte(strings.Repeat("x", MaxModelInputFileBytes+1))
	largeManifest, largeDigest, err := PrepareModelInputManifest("company-1", "mission-1", "task-2", []MissionInputReference{{
		InputID: "large-input", Revision: 1, RequestID: "large-request", SourceKind: "upload", DisplayName: "large.txt", MediaType: "text/plain", ByteSize: int64(len(large)), ContentDigest: digestForTest(string(large)), State: StateUsable,
	}})
	if err != nil {
		t.Fatal(err)
	}
	bounded, err := PrepareModelInputContext(largeManifest, largeDigest, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(bounded.Inputs) != 0 || len(bounded.Excluded) != 1 || bounded.Excluded[0].Reason != "context_limit" {
		t.Fatalf("oversized input was not excluded from bounded context: %+v", bounded)
	}
}

func digestForTest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
