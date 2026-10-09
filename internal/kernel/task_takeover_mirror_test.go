// pattern: Functional Core
package kernel

import "testing"

func TestTakeoverLegacyMirrorRequiresExactSingleFileIdentity(t *testing.T) {
	for _, path := range []string{"workspace.txt", "formatter.go"} {
		entry := WorkspaceTreeEntry{RelativePath: path, Digest: repeatDigest('a'), SourceRevision: 2}
		if !taskTakeoverWorkspaceTreeMatchesLegacy([]WorkspaceTreeEntry{entry}, entry.Digest, 2) {
			t.Fatal("formal legacy mirror rejected", path)
		}
		if taskTakeoverWorkspaceTreeMatchesLegacy([]WorkspaceTreeEntry{entry}, repeatDigest('b'), 2) || taskTakeoverWorkspaceTreeMatchesLegacy([]WorkspaceTreeEntry{entry}, entry.Digest, 3) || taskTakeoverWorkspaceTreeMatchesLegacy([]WorkspaceTreeEntry{entry, entry}, entry.Digest, 2) {
			t.Fatal("mismatched legacy mirror accepted", path)
		}
		entry.RelativePath = "arbitrary.go"
		if taskTakeoverWorkspaceTreeMatchesLegacy([]WorkspaceTreeEntry{entry}, entry.Digest, 2) {
			t.Fatal("unapproved mirror path accepted")
		}
	}
}
