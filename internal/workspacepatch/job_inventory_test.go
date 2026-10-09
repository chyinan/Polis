// pattern: Functional Core
package workspacepatch

import (
	"polis/internal/gotestprofile"
	"testing"
)

func TestPatchInventoryDigestMatchesActualGoJobBinding(t *testing.T) {
	content := []byte("package formatter\nfunc Render() string {return \"old\"}\n")
	sha := digestBytes(content)
	files := []gotestprofile.SnapshotFile{{RelativePath: "formatter.go", Digest: sha, Bytes: int64(len(content)), ContentType: "text/utf-8", EntryType: "regular", Content: content}}
	want, _, err := gotestprofile.ValidateFixtureSnapshotFiles(files)
	if err != nil {
		t.Fatal(err)
	}
	got := fileReferencesDigest([]FileReference{{RelativePath: "formatter.go", SHA256: sha, Bytes: int64(len(content)), FileRevision: 1, SourceRevision: 1, ContentType: "text/utf-8"}})
	if got != want {
		t.Fatalf("patch inventory=%s but actual frozen Go binding inventory=%s", got, want)
	}
}

func FuzzGoInventoryDigestIgnoresOrdering(f *testing.F) {
	f.Add([]byte("first"), []byte("second"))
	f.Add([]byte{}, []byte{0, 1, 255})
	f.Fuzz(func(t *testing.T, left, right []byte) {
		if len(left) > 4096 {
			left = left[:4096]
		}
		if len(right) > 4096 {
			right = right[:4096]
		}
		files := []gotestprofile.SnapshotFile{{RelativePath: "formatter.go", Digest: digestBytes(left), Bytes: int64(len(left))}, {RelativePath: "extra_test.go", Digest: digestBytes(right), Bytes: int64(len(right))}}
		reversed := []gotestprofile.SnapshotFile{files[1], files[0]}
		if gotestprofile.InventoryDigest(files) != gotestprofile.InventoryDigest(reversed) {
			t.Fatal("immutable inventory depends on input ordering")
		}
	})
}
