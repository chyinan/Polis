// pattern: Imperative Shell
package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestStopProofBindsSessionAndWaitsForExit(t *testing.T) {
	p, e := Start("attempt-test", []string{"/bin/sleep", "60"}, nil)
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
