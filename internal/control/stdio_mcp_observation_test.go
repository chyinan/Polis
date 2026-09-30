// pattern: Imperative Shell
package control

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"polis/internal/capabilitysource"
	"polis/internal/kernel"
)

type retryableMCPObserverFixture struct {
	closeCalls int
}

func (*retryableMCPObserverFixture) Observe(context.Context, string, ObserveStdioMCPRuntimeRequest) (kernel.StdioMCPRuntimeQualification, error) {
	return kernel.StdioMCPRuntimeQualification{}, nil
}

func (observer *retryableMCPObserverFixture) Close() error {
	observer.closeCalls++
	if observer.closeCalls == 1 {
		return errors.New("injected process-tree cleanup uncertainty")
	}
	return nil
}

func TestServiceCloseRetainsMCPObserverUntilCleanupSucceeds(t *testing.T) {
	service := NewService(nil, nil)
	observer := &retryableMCPObserverFixture{}
	service.SetStdioMCPRuntimeObserver(observer)

	if err := service.Close(); err == nil {
		t.Fatal("first observer cleanup failure was ignored")
	}
	if service.mcpRuntimeObserver != observer {
		t.Fatal("service discarded the observer after cleanup failed")
	}
	if err := service.Close(); err != nil {
		t.Fatalf("retry observer cleanup: %v", err)
	}
	if observer.closeCalls != 2 || service.mcpRuntimeObserver != nil {
		t.Fatalf("observer cleanup calls=%d retained=%t, want two calls then release", observer.closeCalls, service.mcpRuntimeObserver != nil)
	}
}

func TestMaterializeMCPRuntimePackageWritesDigestPinnedFiles(t *testing.T) {
	workspace := t.TempDir()
	content := []byte("fixed MCP executable bytes")
	digest := sha256.Sum256(content)
	files := []capabilitysource.StdioMCPBundleFile{{
		RelativePath: "runtime/node.exe", MediaType: "application/octet-stream", ByteSize: int64(len(content)),
		ContentSHA256: hex.EncodeToString(digest[:]), Content: content,
	}}
	packageRoot := filepath.Join(workspace, "mcp-observation-one")
	pinned, err := materializeMCPRuntimePackage(workspace, packageRoot, files)
	if err != nil {
		t.Fatalf("materialize package files: %v", err)
	}
	if len(pinned) != 1 || pinned[0].Path != filepath.Join(packageRoot, "runtime", "node.exe") || pinned[0].SHA256 != files[0].ContentSHA256 {
		t.Fatalf("pinned file inventory=%+v", pinned)
	}
	readback, err := os.ReadFile(pinned[0].Path)
	if err != nil || string(readback) != string(content) {
		t.Fatalf("materialized package readback=%q error=%v", readback, err)
	}
}

func TestMaterializeMCPRuntimePackageRejectsPathsOutsidePrivateRoot(t *testing.T) {
	workspace := t.TempDir()
	outside := filepath.Join(workspace, "outside.txt")
	if err := os.WriteFile(outside, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	content := []byte("bad path")
	digest := sha256.Sum256(content)
	files := []capabilitysource.StdioMCPBundleFile{{
		RelativePath: "../outside.txt", MediaType: "text/plain", ByteSize: int64(len(content)),
		ContentSHA256: hex.EncodeToString(digest[:]), Content: content,
	}}
	packageRoot := filepath.Join(workspace, "mcp-observation-two")
	if _, err := materializeMCPRuntimePackage(workspace, packageRoot, files); err == nil {
		t.Fatal("package traversal escaped its private observation root")
	}
	readback, err := os.ReadFile(outside)
	if err != nil || string(readback) != "keep" {
		t.Fatalf("outside sentinel changed: content=%q error=%v", readback, err)
	}
	if _, err = os.Lstat(packageRoot); !os.IsNotExist(err) {
		t.Fatalf("failed materialization left a workspace behind: %v", err)
	}
}
