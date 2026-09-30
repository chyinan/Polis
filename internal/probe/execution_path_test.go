// pattern: Functional Core
package probe

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAuthorizationBindingFutureOutputDoesNotRequireLeaf(t *testing.T) {
	root := t.TempDir()
	ref := AuthorizationBindingPathRef(filepath.Join(root, "future", "frontend-binding.json"), "wsl", true)
	if ref.MustExist || !ref.CreateAllowed || ref.PathKind != ExecutionPathFile {
		t.Fatalf("unexpected future-output contract: %+v", ref)
	}
	diagnostic, err := ValidateExecutionPathRef(ref, true)
	if err != nil || diagnostic.Status != "PASS" || diagnostic.RequiredSemantics != "future_output_file_parent_accessible" {
		t.Fatalf("future output was rejected: diagnostic=%+v err=%v", diagnostic, err)
	}
}

func TestAuthorizationBindingExistingInputStillRequiresLeaf(t *testing.T) {
	ref := AuthorizationBindingPathRef("/tmp/polis-missing-binding.json", "wsl", false)
	diagnostic, err := ValidateExecutionPathRef(ref, true)
	if err == nil || diagnostic.ReasonCode != "authorization_binding_path_unavailable" || diagnostic.Status != "FAILED" {
		t.Fatalf("missing existing input was accepted: diagnostic=%+v err=%v", diagnostic, err)
	}
}

func TestAuthorizationBindingPathRejectsTraversalAndWrongOS(t *testing.T) {
	traversal := AuthorizationBindingPathRef("/mnt/d/Programs/Polis/../secret/binding.json", "wsl", true)
	if _, err := ValidateExecutionPathRef(traversal, false); err == nil {
		t.Fatal("traversal path was accepted")
	}
	wrongOS := AuthorizationBindingPathRef("/mnt/d/Programs/Polis/binding.json", "windows-native", true)
	diagnostic, err := ValidateExecutionPathRef(wrongOS, false)
	if err == nil || diagnostic.ReasonCode != "authorization_binding_path_invalid_representation" {
		t.Fatalf("wrong OS representation was accepted: diagnostic=%+v err=%v", diagnostic, err)
	}
}

func TestAuthorizationBindingPathHandlesSpacesAndTypeMismatch(t *testing.T) {
	root := t.TempDir()
	withSpaces := filepath.Join(root, "Polis recovery", "binding.json")
	diagnostic, err := ValidateExecutionPathRef(AuthorizationBindingPathRef(withSpaces, "wsl", true), true)
	if err != nil || diagnostic.Status != "PASS" {
		t.Fatalf("path with spaces was rejected: diagnostic=%+v err=%v", diagnostic, err)
	}
	if err := os.Mkdir(filepath.Join(root, "directory-binding"), 0o700); err != nil {
		t.Fatal(err)
	}
	diagnostic, err = ValidateExecutionPathRef(AuthorizationBindingPathRef(filepath.Join(root, "directory-binding"), "wsl", true), true)
	if err == nil || diagnostic.ReasonCode != "authorization_binding_path_type_mismatch" {
		t.Fatalf("file/directory mismatch was accepted: diagnostic=%+v err=%v", diagnostic, err)
	}
}

func TestAuthorizationBindingFrozenFailureRegression(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "not-yet-created", "authorization.json")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("fixture unexpectedly exists: %v", err)
	}
	ref := AuthorizationBindingPathRef(path, "wsl", true)
	if _, err := ValidateExecutionPathRef(ref, true); err != nil {
		t.Fatalf("hardened future-output validation did not replace old leaf-exists failure: %v", err)
	}
}
