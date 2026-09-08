// pattern: Imperative Shell
package codex

import (
	"context"
	"os"
	"path/filepath"
	"polis/internal/runner"
	"testing"
	"time"
)

func TestNativeProtocolWithoutInference(t *testing.T) {
	binary := os.Getenv("POLIS_CODEX_BINARY")
	if binary == "" {
		t.Skip("pinned native binary required")
	}
	root := t.TempDir()
	args, _, e := runner.NativeArgs(binary, filepath.Join(root, "home"), "")
	if e != nil {
		t.Fatal(e)
	}
	helperArgs := append([]string(nil), args...)
	helperArgs[len(helperArgs)-3] = "/codex-code-mode-host"
	helperArgs[len(helperArgs)-2] = "--help"
	helperArgs = helperArgs[:len(helperArgs)-1]
	if _, e = runner.Run(helperArgs, []string{"PATH=/usr/bin:/bin"}, 10*time.Second); e != nil {
		t.Fatalf("native helper cannot execute in sandbox: %v", e)
	}
	p, e := runner.Start("native-protocol", args, []string{"PATH=/usr/bin:/bin"})
	if e != nil {
		t.Fatal(e)
	}
	defer p.Stop()
	c, e := New(p, filepath.Join(root, "evidence"))
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if e = c.Initialize(ctx); e != nil {
		t.Fatal(e)
	}
	if _, e = c.StartThread(ctx, "medium"); e != nil {
		t.Fatal(e)
	}
	if _, e = c.StartThread(ctx, "high"); e != nil {
		t.Fatal(e)
	}
	// No turn/start occurs in this qualification test.
}
