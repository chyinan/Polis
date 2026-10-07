// pattern: Imperative Shell
package browser

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPlaywrightRunnerUsesAnEmptyPrivateProfileAndControlledCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the disposable shell helper uses POSIX syntax")
	}
	root := t.TempDir()
	t.Setenv("OPENAI_API_KEY", "must-not-leak")
	script := filepath.Join(root, "fake-playwright.sh")
	content := `#!/bin/sh
if [ -n "$OPENAI_API_KEY" ]; then exit 7; fi
cat >/dev/null
printf '%s' '{"protocol":"polis-browser-runner@1","state":"succeeded","reason_code":"browser_run_succeeded","final_url":"https://example.test/app","title":"fixture","text":"rendered","request_count":1,"blocked_request_count":0,"response_bytes":128,"download_count":0,"websocket_count":0}'
`
	if err := os.WriteFile(script, []byte(content), 0700); err != nil {
		t.Fatal(err)
	}
	runner := PlaywrightRunner{PythonPath: "/bin/sh", ScriptPath: script, ProfileRoot: root}
	outcome, err := runner.Run(context.Background(), BrowserRunPlan{TargetOrigin: "https://example.test", TimeoutMS: 5000, MaxRequests: 8, MaxResponseBytes: 4096})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Text != "rendered" || outcome.FinalURL != "https://example.test/app" {
		t.Fatalf("outcome=%+v", outcome)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "fake-playwright.sh" {
		t.Fatalf("profile cleanup left unexpected entries=%v", entries)
	}
}

func TestPlaywrightRunnerRejectsMissingProfileRoot(t *testing.T) {
	if _, err := (PlaywrightRunner{PythonPath: "/bin/sh", ScriptPath: "/tmp/missing-playwright-script", ProfileRoot: "/tmp/missing-profile-root"}).Run(context.Background(), BrowserRunPlan{TargetOrigin: "https://example.test", TimeoutMS: 5000, MaxRequests: 8, MaxResponseBytes: 4096}); err == nil {
		t.Fatal("runner accepted missing profile root")
	}
}

func TestPlaywrightRunnerWindowsChromeLoopbackTLSFixture(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("requires the authorized Windows Chrome qualification host")
	}
	pythonPath := os.Getenv("POLIS_TEST_PYTHON_PATH")
	pythonPackageRoot := os.Getenv("POLIS_TEST_PYTHON_PACKAGE_ROOT")
	browserPath := os.Getenv("POLIS_TEST_BROWSER_PATH")
	scriptPath := os.Getenv("POLIS_TEST_BROWSER_SCRIPT")
	if pythonPath == "" || pythonPackageRoot == "" || browserPath == "" || scriptPath == "" {
		t.Skip("POLIS_TEST_PYTHON_PATH, POLIS_TEST_PYTHON_PACKAGE_ROOT, POLIS_TEST_BROWSER_PATH and POLIS_TEST_BROWSER_SCRIPT are required")
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(writer, "<!doctype html><title>Polis runner fixture</title><main>runner fixture rendered</main>")
	}))
	defer server.Close()
	runner := PlaywrightRunner{PythonPath: pythonPath, PythonPackageRoot: pythonPackageRoot, ScriptPath: scriptPath, ProfileRoot: t.TempDir(), BrowserExecutable: browserPath}
	outcome, err := runner.Run(context.Background(), BrowserRunPlan{TargetOrigin: server.URL, TimeoutMS: 30000, MaxRequests: 32, MaxResponseBytes: 1 << 20, TestOnlyAllowInsecureTLS: true})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Title != "Polis runner fixture" || !strings.Contains(outcome.Text, "runner fixture rendered") || outcome.DownloadCount != 0 || outcome.WebSocketCount != 0 {
		t.Fatalf("outcome=%+v", outcome)
	}
}
