package kernel

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestScanSkillLibraryReadsManifestsAndSkipsScripts(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "skills", "review-notes")
	if err := os.MkdirAll(filepath.Join(dir, "scripts"), 0o700); err != nil {
		t.Fatal(err)
	}
	document := []byte("---\nname: review-notes\ndescription: checklist\n---\nDo not run scripts.\n")
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), document, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scripts", "run.sh"), []byte("echo should-not-run\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	mcp := filepath.Join(root, "mcp")
	if err := os.MkdirAll(mcp, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mcp, "docs.yaml"), []byte("name: docs\ntransport: stdio\ncommand: echo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	linkExpected := true
	link := filepath.Join(root, "skills", "linked-notes")
	if err := os.Symlink(dir, link); err != nil {
		if runtime.GOOS != "windows" {
			t.Fatal(err)
		}
		linkExpected = false
	}

	scan, err := ScanSkillLibrary(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(scan.Skills) != 1 || scan.Skills[0].SourceRef != "library://skills/review-notes/SKILL.md" || !scan.Skills[0].ScriptsPresent {
		t.Fatalf("skills = %+v", scan.Skills)
	}
	if string(scan.Skills[0].Document) != string(document) {
		t.Fatal("skill bytes changed")
	}
	if len(scan.MCP) != 1 || scan.MCP[0].SourceRef != "library://mcp/docs.yaml" {
		t.Fatalf("mcp = %+v", scan.MCP)
	}
	foundLink := false
	for _, rejected := range scan.Rejected {
		if rejected.SourceRef == "library://skills/linked-notes/SKILL.md" && rejected.Reason == "symlink" {
			foundLink = true
		}
		if rejected.Reason == "should-not-run" {
			t.Fatal("script body leaked into the scan result")
		}
	}
	if linkExpected && !foundLink {
		t.Fatalf("rejected = %+v", scan.Rejected)
	}
}
