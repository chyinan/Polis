// pattern: Functional Core
package workspacepath

import "testing"

func TestValidRelativePathAcceptsCanonicalFilePaths(t *testing.T) {
	for _, path := range []string{"formatter.go", "internal/core/model.go", "a-b/c_d.txt"} {
		if !ValidRelativePath(path) {
			t.Errorf("ValidRelativePath(%q) = false, want true", path)
		}
	}
}

func TestValidRelativePathRejectsEscapesAndAmbiguousNames(t *testing.T) {
	for _, path := range []string{"", "/absolute.go", "../escape.go", "a/../b", "a//b", "a/.", "a\\b", "a%2fb", "a:b", " a.go", "a.go\n"} {
		if ValidRelativePath(path) {
			t.Errorf("ValidRelativePath(%q) = true, want false", path)
		}
	}
}
