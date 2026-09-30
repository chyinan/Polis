// pattern: Functional Core
package probe

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ExecutionPathKind string

const (
	ExecutionPathFile      ExecutionPathKind = "file"
	ExecutionPathDirectory ExecutionPathKind = "directory"
)

type ExecutionPathRef struct {
	LogicalRole       string            `json:"logical_role"`
	ProducerOS        string            `json:"producer_os"`
	ConsumerOS        string            `json:"consumer_os"`
	RepresentationOS  string            `json:"representation_os"`
	PathKind          ExecutionPathKind `json:"path_kind"`
	AccessMode        string            `json:"access_mode"`
	MustExist         bool              `json:"must_exist"`
	ParentMustExist   bool              `json:"parent_must_exist"`
	CreateAllowed     bool              `json:"create_allowed"`
	Path              string            `json:"path"`
	WindowsPath       string            `json:"windows_path,omitempty"`
	WSLPath           string            `json:"wsl_path,omitempty"`
	CanonicalIdentity string            `json:"canonical_identity,omitempty"`
}

type ExecutionPathDiagnostic struct {
	LogicalRole       string            `json:"logical_role"`
	ConsumerOS        string            `json:"consumer_os"`
	RepresentationOS  string            `json:"representation_os"`
	PathKind          ExecutionPathKind `json:"path_kind"`
	RequiredSemantics string            `json:"required_semantics"`
	Status            string            `json:"status"`
	ReasonCode        string            `json:"reason_code,omitempty"`
	ActionableSummary string            `json:"actionable_summary,omitempty"`
}

func AuthorizationBindingPathRef(path, pathEncoding string, futureOutput bool) ExecutionPathRef {
	consumer := "wsl"
	if pathEncoding == "windows-native" {
		consumer = "windows"
	}
	if futureOutput {
		consumer = "windows"
	}
	ref := ExecutionPathRef{
		LogicalRole:      "authorization_binding_path",
		ProducerOS:       "windows",
		ConsumerOS:       consumer,
		RepresentationOS: pathEncoding,
		PathKind:         ExecutionPathFile,
		AccessMode:       "read_write",
		MustExist:        !futureOutput,
		ParentMustExist:  !futureOutput,
		CreateAllowed:    futureOutput,
		Path:             path,
	}
	if futureOutput {
		ref.CanonicalIdentity = canonicalExecutionPath(path)
	}
	return ref
}

func ValidateExecutionPathRef(ref ExecutionPathRef, checkAccess bool) (ExecutionPathDiagnostic, error) {
	diagnostic := ExecutionPathDiagnostic{
		LogicalRole:       ref.LogicalRole,
		ConsumerOS:        ref.ConsumerOS,
		RepresentationOS:  ref.RepresentationOS,
		PathKind:          ref.PathKind,
		RequiredSemantics: executionPathSemantics(ref),
		Status:            "PASS",
	}
	if ref.LogicalRole == "" || ref.Path == "" || ref.PathKind == "" || ref.ConsumerOS == "" || ref.RepresentationOS == "" {
		return pathFailure(diagnostic, "authorization_binding_path_invalid", "typed path reference is incomplete")
	}
	if !validExecutionPathRepresentation(ref.Path, ref.RepresentationOS) {
		return pathFailure(diagnostic, "authorization_binding_path_invalid_representation", "path representation does not match the consumer OS")
	}
	if hasTraversalSegment(ref.Path) {
		return pathFailure(diagnostic, "authorization_binding_path_traversal", "path contains a traversal segment")
	}
	if !checkAccess {
		return diagnostic, nil
	}

	info, statErr := os.Stat(ref.Path)
	if ref.MustExist {
		if statErr != nil {
			return pathFailure(diagnostic, "authorization_binding_path_unavailable", "required existing input file is unavailable")
		}
		if ref.PathKind == ExecutionPathFile && info.IsDir() {
			return pathFailure(diagnostic, "authorization_binding_path_type_mismatch", "expected a regular file")
		}
		if ref.PathKind == ExecutionPathDirectory && !info.IsDir() {
			return pathFailure(diagnostic, "authorization_binding_path_type_mismatch", "expected a directory")
		}
		return diagnostic, nil
	}
	if statErr == nil {
		if ref.PathKind == ExecutionPathFile && info.IsDir() {
			return pathFailure(diagnostic, "authorization_binding_path_type_mismatch", "future output path is a directory")
		}
		if ref.PathKind == ExecutionPathDirectory && !info.IsDir() {
			return pathFailure(diagnostic, "authorization_binding_path_type_mismatch", "future output path is not a directory")
		}
		return diagnostic, nil
	}
	if !errors.Is(statErr, os.ErrNotExist) || !ref.CreateAllowed {
		return pathFailure(diagnostic, "authorization_binding_path_unavailable", "path cannot be addressed by the consumer")
	}
	parent, err := nearestExistingDirectory(filepath.Dir(ref.Path))
	if err != nil {
		return pathFailure(diagnostic, "authorization_binding_parent_unavailable", "future output parent is neither present nor safely creatable")
	}
	if !parentAllowsCreate(parent) {
		return pathFailure(diagnostic, "authorization_binding_parent_inaccessible", "future output parent is not writable under the current access policy")
	}
	return diagnostic, nil
}

func executionPathSemantics(ref ExecutionPathRef) string {
	if ref.MustExist {
		return "existing_input_file"
	}
	return "future_output_file_parent_accessible"
}

func pathFailure(diagnostic ExecutionPathDiagnostic, reason, summary string) (ExecutionPathDiagnostic, error) {
	diagnostic.Status = "FAILED"
	diagnostic.ReasonCode = reason
	diagnostic.ActionableSummary = summary
	return diagnostic, fmt.Errorf("%s: %s", reason, summary)
}

func validExecutionPathRepresentation(path, consumer string) bool {
	if consumer == "windows" || consumer == "windows-native" {
		return isWindowsAbsolute(path)
	}
	return isWSLAbsolute(path)
}

func hasTraversalSegment(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(strings.ReplaceAll(path, "\\", "/")), "/") {
		if part == ".." {
			return true
		}
	}
	return false
}

func nearestExistingDirectory(path string) (string, error) {
	current := filepath.Clean(path)
	for {
		info, err := os.Stat(current)
		if err == nil {
			if info.IsDir() {
				return current, nil
			}
			return "", errors.New("existing ancestor is not a directory")
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		next := filepath.Dir(current)
		if next == current {
			return "", err
		}
		current = next
	}
}

func parentAllowsCreate(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false
	}
	// On Unix this catches the common read-only parent case without creating a
	// probe file. Windows ACL enforcement remains the consumer's final check.
	return info.Mode().Perm()&0222 != 0 || info.Mode().Perm() == 0
}

func canonicalExecutionPath(path string) string {
	return strings.ToLower(filepath.ToSlash(filepath.Clean(strings.ReplaceAll(path, "\\", "/"))))
}
