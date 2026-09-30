// pattern: Functional Core
package intake

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareGitSnapshotPinsCommitAndDisclosesOmittedInputs(t *testing.T) {
	commitID := strings.Repeat("a", 40)
	treeID := strings.Repeat("b", 40)
	snapshot, err := PrepareGitSnapshot("project", strings.Repeat("c", 64), commitID, treeID, "not_included_by_policy", []DirectoryInputFile{
		{RelativePath: "project/README.md", MediaType: "text/markdown", Content: []byte("committed readme\n")},
	}, []GitSnapshotExclusion{
		{RelativePath: "project/vendor-lib", Reason: "submodule_content_not_fetched"},
		{RelativePath: "project/assets/large.bin", Reason: "git_lfs_payload_not_fetched"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Upload.SourceKind != "git_snapshot" || snapshot.Upload.State != StatePartial || snapshot.Upload.DisplayName != "project@"+commitID[:12] {
		t.Fatalf("Git snapshot metadata = %+v", snapshot.Upload)
	}
	if err = VerifyPreparedUpload(snapshot.Upload, snapshot.Archive); err != nil {
		t.Fatalf("canonical Git snapshot did not verify: %v", err)
	}
	files, err := ExtractVerifiedInputArchive("git_snapshot", snapshot.Archive)
	if err != nil {
		t.Fatal(err)
	}
	var sourceNote GitSnapshotSourceNote
	for _, file := range files {
		if file.RelativePath == "project/.polis-git-source.json" {
			if err = json.Unmarshal(file.Content, &sourceNote); err != nil {
				t.Fatal(err)
			}
		}
	}
	if sourceNote.CommitID != commitID || sourceNote.WorkingTreeStatus != "not_included_by_policy" || sourceNote.SubmodulesNotFetched != 1 || sourceNote.LFSPayloadsNotFetched != 1 || !strings.Contains(sourceNote.WorkingTreePolicy, "separate directory snapshot") {
		t.Fatalf("worker source note omitted commit or incomplete-source status: %+v", sourceNote)
	}
	if len(sourceNote.OmissionExamples) != 2 || sourceNote.OmissionExamples[0].RelativePath != "project/assets/large.bin" || sourceNote.OmissionExamples[0].Reason != "git_lfs_payload_not_fetched" {
		t.Fatalf("worker source note omitted concrete missing-path examples: %+v", sourceNote.OmissionExamples)
	}
	if _, err = PrepareGitSnapshot("project", strings.Repeat("c", 64), commitID, treeID, "not_included_by_policy", []DirectoryInputFile{
		{RelativePath: "project/.polis-git-source.json", MediaType: "application/json", Content: []byte("collision")},
	}, nil); gitSnapshotReason(err) != "git_snapshot_reserved_path" {
		t.Fatalf("reserved metadata path collision error = %v", err)
	}
}

func TestImportGitCommitUsesOnlyTheSelectedCommitWithoutReadingWorktreeFilters(t *testing.T) {
	repo := t.TempDir()
	runGitTest(t, repo, "init", "-q")
	runGitTest(t, repo, "config", "user.name", "Polis Fixture")
	runGitTest(t, repo, "config", "user.email", "polis-fixture@example.invalid")
	if err := os.WriteFile(filepath.Join(repo, ".gitattributes"), []byte("*.md filter=polis-test-clean\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("selected commit sentinel\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGitTest(t, repo, "add", ".gitattributes", "README.md")
	runGitTest(t, repo, "commit", "-q", "-m", "fixture")
	commitID := strings.TrimSpace(string(runGitTest(t, repo, "rev-parse", "HEAD")))
	filterMarker := filepath.Join(t.TempDir(), "clean-filter-ran")
	runGitTest(t, repo, "config", "filter.polis-test-clean.clean", "touch "+filterMarker)
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("uncommitted replacement sentinel\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "untracked.md"), []byte("untracked sentinel\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// A configured remote is deliberately unreachable. The local fixed-commit importer must not contact it.
	runGitTest(t, repo, "remote", "add", "origin", "https://127.0.0.1:1/not-used.git")

	snapshot, err := ImportGitCommit(context.Background(), repo, commitID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filterMarker); err == nil {
		t.Fatal("local Git clean filter ran during read-only import")
	}
	if snapshot.Upload.SourceKind != "git_snapshot" || snapshot.Upload.State != StatePartial {
		t.Fatalf("dirty local import status = %+v", snapshot.Upload)
	}
	files, err := ExtractVerifiedInputArchive("git_snapshot", snapshot.Archive)
	if err != nil {
		t.Fatal(err)
	}
	contents := make(map[string][]byte, len(files))
	for _, file := range files {
		contents[file.RelativePath] = file.Content
	}
	root := filepath.Base(repo)
	readme := contents[root+"/README.md"]
	if !bytes.Contains(readme, []byte("selected commit sentinel")) || bytes.Contains(readme, []byte("uncommitted replacement sentinel")) {
		t.Fatalf("snapshot did not preserve exactly the selected commit: %q", readme)
	}
	if _, exists := contents[root+"/untracked.md"]; exists {
		t.Fatal("untracked worktree content leaked into the selected commit snapshot")
	}
	var note GitSnapshotSourceNote
	if err = json.Unmarshal(contents[root+"/.polis-git-source.json"], &note); err != nil {
		t.Fatal(err)
	}
	if note.WorkingTreeStatus != "not_included_by_policy" || note.CommitID != commitID || note.NetworkFetch != "disabled" {
		t.Fatalf("Git source note = %+v", note)
	}
}

func TestImportGitCommitRejectsNonFullCommitSelectors(t *testing.T) {
	if _, err := ImportGitCommit(context.Background(), t.TempDir(), "HEAD"); gitSnapshotReason(err) != "git_commit_selector_invalid" {
		t.Fatalf("branch selector error = %v", err)
	}
}

func runGitTest(t *testing.T, directory string, args ...string) []byte {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", directory}, args...)...)
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v: %s", strings.Join(args, " "), err, output)
	}
	return output
}

func gitSnapshotReason(err error) string {
	if err == nil {
		return ""
	}
	var uploadErr *UploadError
	if errors.As(err, &uploadErr) {
		return uploadErr.ReasonCode
	}
	return err.Error()
}
