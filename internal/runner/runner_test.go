// pattern: Imperative Shell
package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	switch os.Getenv("POLIS_RUNNER_HELPER") {
	case "child":
		time.Sleep(5 * time.Minute)
		os.Exit(0)
	case "host-reconcile":
		time.Sleep(5 * time.Minute)
		os.Exit(0)
	case "parent":
		pidFile := os.Getenv("POLIS_RUNNER_CHILD_PID_FILE")
		child := exec.Command(os.Args[0])
		child.Env = []string{"POLIS_RUNNER_HELPER=child"}
		if err := child.Start(); err != nil {
			os.Exit(2)
		}
		if err := os.WriteFile(pidFile, []byte(strconv.Itoa(child.Process.Pid)), 0600); err != nil {
			os.Exit(3)
		}
		time.Sleep(100 * time.Millisecond)
		os.Exit(0)
	default:
		os.Exit(m.Run())
	}
}

func TestStopProofBindsSessionAndWaitsForExit(t *testing.T) {
	argv := []string{"/bin/sleep", "60"}
	if runtime.GOOS == "windows" {
		argv = []string{"cmd.exe", "/c", "ping -n 61 127.0.0.1 >NUL"}
	}
	p, e := Start("attempt-test", argv, nil)
	if e != nil {
		t.Fatal(e)
	}
	proof, e := p.Stop()
	if e != nil {
		t.Fatal(e)
	}
	if !proof.For("attempt-test") || proof.For("another-attempt") {
		t.Fatal("stop proof lacks session ownership")
	}
}

func TestVerifierRejectsExpiredProbeBeforeExecution(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	v := Verifier{Context: ctx, Scratch: t.TempDir()}
	if _, e := v.Check("package formatter", "single"); e != context.Canceled {
		t.Fatalf("expired probe was not stopped: %v", e)
	}
}

func TestNativeBundleRequiresCodeModeHost(t *testing.T) {
	root := t.TempDir()
	binary := filepath.Join(root, "codex")
	if e := os.WriteFile(binary, []byte("test"), 0700); e != nil {
		t.Fatal(e)
	}
	if _, _, e := NativeArgs(binary, filepath.Join(root, "home"), ""); e == nil {
		t.Fatal("incomplete native package admitted")
	}
}

func TestNativeArgsTransportPoliciesAreExplicitlyDistinct(t *testing.T) {
	root := t.TempDir()
	binary := filepath.Join(root, "codex")
	helper := filepath.Join(root, "codex-code-mode-host")
	if e := os.WriteFile(binary, []byte("binary"), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(helper, []byte("helper"), 0700); e != nil {
		t.Fatal(e)
	}
	_, disabledCapability, e := NativeArgsWithTransportPolicy(binary, filepath.Join(root, "disabled"), "", "", NativeTransportPolicyExplicitlyDisabled)
	if e != nil {
		t.Fatal(e)
	}
	defaultArgs, defaultCapability, e := NativeArgsWithTransportPolicy(binary, filepath.Join(root, "default"), "", "", NativeTransportPolicyNativeDefault)
	if e != nil {
		t.Fatal(e)
	}
	if disabledCapability == defaultCapability {
		t.Fatal("explicitly disabled and native default policies share a capability digest")
	}
	defaultLaunch := strings.Join(defaultArgs, " ")
	if strings.Contains(defaultLaunch, "--unshare-cgroup") || strings.Contains(defaultLaunch, "/sys/fs/cgroup") {
		t.Fatalf("legacy native launch profile changed unexpectedly: %s", defaultLaunch)
	}
	disabled, e := os.ReadFile(filepath.Join(root, "disabled", "config.toml"))
	if e != nil {
		t.Fatal(e)
	}
	nativeDefault, e := os.ReadFile(filepath.Join(root, "default", "config.toml"))
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(string(disabled), "supports_websockets = false") || strings.Contains(string(nativeDefault), "supports_websockets = false") || !strings.Contains(string(disabled), "responses_websockets = false") || strings.Contains(string(nativeDefault), "responses_websockets = false") {
		t.Fatalf("transport policy config mismatch: disabled=%s default=%s", disabled, nativeDefault)
	}
}
func TestIsolatedVerifierRejectsBaselineAndAcceptsValidNeighbor(t *testing.T) {
	goRoot := os.Getenv("POLIS_GO_ROOT")
	if goRoot == "" {
		t.Skip("POLIS_GO_ROOT required for sandbox test")
	}
	root := t.TempDir()
	v := Verifier{GoRoot: goRoot, Scratch: root}
	bad, e := v.Check("package formatter\nfunc Render(v float64) string{return \"0\"}", "single")
	if e != nil {
		t.Fatal(e)
	}
	if bad.Passed {
		t.Fatal("constant zero accepted")
	}
	bypass, e := v.Check("package formatter\nimport \"os\"\nfunc init(){os.Exit(0)}\nfunc Render(v float64)string{return \"wrong\"}", "single")
	if e != nil {
		t.Fatal(e)
	}
	if bypass.Passed {
		t.Fatal("test lifecycle bypass was accepted")
	}
	good, e := v.Check("package formatter\nimport(\"math\";\"strconv\")\nfunc Render(v float64)string{if v==0{if math.Signbit(v){return \"-0\"};return \"0\"};s:=strconv.FormatFloat(v,'f',-1,64);if v>0{return \"+\"+s};return s}", "single")
	if e != nil {
		t.Fatal(e)
	}
	if !good.Passed {
		t.Fatal(good.Output)
	}
	if _, e = os.Stat(filepath.Join(root, "escaped")); !os.IsNotExist(e) {
		t.Fatal("unexpected external file")
	}
}
