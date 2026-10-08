// pattern: Functional Core
package kernel

import (
	"strings"
	"testing"

	"polis/internal/intake"
)

func TestWorkspaceTreeGitImportStripsRepositoryRootAndSourceMetadata(t *testing.T) {
	snapshot, err := intake.PrepareGitSnapshot("fixture", strings.Repeat("a", 64), strings.Repeat("b", 40), strings.Repeat("c", 40), "not_included_by_policy", []intake.DirectoryInputFile{
		{RelativePath: "fixture/go.mod", MediaType: "text/plain", Content: []byte("module fixture\n\ngo 1.26\n")},
		{RelativePath: "fixture/formatter.go", MediaType: "text/plain", Content: []byte("package formatter\n")},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, files, err := workspaceTreeFilesFromGitSnapshot(snapshot.Archive)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("unexpected workspace paths: %+v", files)
	}
	if files[0].RelativePath != "formatter.go" || files[1].RelativePath != "go.mod" {
		t.Fatalf("repository root or source note was not normalized and sorted: %+v", files)
	}
}

func TestWorkspaceTreeGitImportRejectsOmissionsAndUnsupportedFiles(t *testing.T) {
	tests := []struct {
		name       string
		files      []intake.DirectoryInputFile
		exclusions []intake.GitSnapshotExclusion
	}{
		{name: "binary file", files: []intake.DirectoryInputFile{{RelativePath: "fixture/data.bin", MediaType: "application/octet-stream", Content: []byte{0, 1, 2}}}},
		{name: "source omission", files: []intake.DirectoryInputFile{{RelativePath: "fixture/main.go", MediaType: "text/plain", Content: []byte("package main\n")}}, exclusions: []intake.GitSnapshotExclusion{{RelativePath: "fixture/link", Reason: "symlink_not_followed"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			snapshot, err := intake.PrepareGitSnapshot("fixture", strings.Repeat("a", 64), strings.Repeat("b", 40), strings.Repeat("c", 40), "not_included_by_policy", test.files, test.exclusions)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err = workspaceTreeFilesFromGitSnapshot(snapshot.Archive); err == nil {
				t.Fatal("incomplete or unsupported Git commit entered WorkspaceTree")
			}
		})
	}
}
