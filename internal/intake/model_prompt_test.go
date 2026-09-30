// pattern: Functional Core
package intake

import (
	"strings"
	"testing"
)

func TestPrepareModelInputContextIncludesBoundTextAndExcludesOtherRepresentations(t *testing.T) {
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
		t.Fatalf("model input payload digest = %q", prepared.PayloadDigest)
	}
}

func TestPrepareModelInputContextRejectsDigestDriftAndBoundsLargeText(t *testing.T) {
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

func TestPrepareModelInputContextIncludesValidatedCSVSource(t *testing.T) {
	content := []byte("name,value\nalpha,7\n")
	manifest, digest, err := PrepareModelInputManifest("company-1", "mission-1", "task-csv", []MissionInputReference{{
		InputID: "csv-input", Revision: 1, RequestID: "csv-request", SourceKind: "upload", DisplayName: "metrics.csv", MediaType: "text/csv", ByteSize: int64(len(content)), ContentDigest: digestForTest(string(content)), State: StateUsable,
	}})
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := PrepareModelInputContext(manifest, digest, map[string][]byte{"csv-input": content})
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.Inputs) != 1 || prepared.Inputs[0].Text != string(content) || !strings.Contains(prepared.PromptSection, "metrics.csv") {
		t.Fatalf("validated CSV was not included as a bounded Worker input: %+v", prepared)
	}
}

func TestPrepareModelInputContextExtractsSafeZIPTextAndReportsUnsupportedFiles(t *testing.T) {
	archive := zipForTest(t, map[string]string{
		"repo/README.md": "ZIP input reaches the worker",
		"repo/data.bin":  "opaque bytes",
	})
	preparedUpload, err := PrepareUpload("source.zip", "application/zip", archive)
	if err != nil {
		t.Fatal(err)
	}
	manifest, digest, err := PrepareModelInputManifest("company-1", "mission-1", "task-zip", []MissionInputReference{{
		InputID: "zip-input", Revision: 1, RequestID: "zip-request", SourceKind: preparedUpload.SourceKind, DisplayName: preparedUpload.DisplayName,
		MediaType: preparedUpload.MediaType, ByteSize: preparedUpload.ByteSize, ContentDigest: preparedUpload.ContentDigest, State: preparedUpload.State,
	}})
	if err != nil {
		t.Fatal(err)
	}
	context, err := PrepareModelInputContext(manifest, digest, map[string][]byte{"zip-input": archive})
	if err != nil {
		t.Fatal(err)
	}
	if len(context.Inputs) != 1 || context.Inputs[0].RelativePath != "repo/README.md" || !strings.Contains(context.Inputs[0].Text, "reaches the worker") {
		t.Fatalf("ZIP text was not delivered with its path: %+v", context.Inputs)
	}
	if len(context.Excluded) != 1 || context.Excluded[0].RelativePath != "repo/data.bin" || context.Excluded[0].Reason != "representation_not_supported" {
		t.Fatalf("unsupported ZIP entry was not disclosed: %+v", context.Excluded)
	}
}

func TestPrepareModelInputContextDeliversPinnedGitCommitAndSourceStatus(t *testing.T) {
	commitID := strings.Repeat("d", 40)
	snapshot, err := PrepareGitSnapshot("repo", strings.Repeat("e", 64), commitID, strings.Repeat("f", 40), "not_included_by_policy", []DirectoryInputFile{
		{RelativePath: "repo/README.md", MediaType: "text/markdown", Content: []byte("POLIS_GIT_COMMIT_SENTINEL_2026\n")},
	}, []GitSnapshotExclusion{{RelativePath: "repo/vendor", Reason: "submodule_content_not_fetched"}})
	if err != nil {
		t.Fatal(err)
	}
	manifest, manifestDigest, err := PrepareModelInputManifest("company-git", "mission-git", "task-git", []MissionInputReference{{
		InputID: "git-input", Revision: 1, RequestID: "git-import", SourceKind: snapshot.Upload.SourceKind,
		DisplayName: snapshot.Upload.DisplayName, MediaType: snapshot.Upload.MediaType, ByteSize: snapshot.Upload.ByteSize,
		ContentDigest: snapshot.Upload.ContentDigest, State: snapshot.Upload.State,
	}})
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := PrepareModelInputContext(manifest, manifestDigest, map[string][]byte{"git-input": snapshot.Archive})
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.Inputs) != 2 || !strings.Contains(prepared.PromptSection, "POLIS_GIT_COMMIT_SENTINEL_2026") || !strings.Contains(prepared.PromptSection, commitID) || !strings.Contains(prepared.PromptSection, "not_included_by_policy") || !strings.Contains(prepared.PromptSection, "separate directory snapshot") || !strings.Contains(prepared.PromptSection, "submodule_content_not_fetched") {
		t.Fatalf("Git source and its provenance were not delivered together: inputs=%+v prompt=%s", prepared.Inputs, prepared.PromptSection)
	}
	if strings.Contains(prepared.PromptSection, "vendor source bytes") {
		t.Fatalf("excluded submodule content unexpectedly entered the Worker prompt: %s", prepared.PromptSection)
	}
}
