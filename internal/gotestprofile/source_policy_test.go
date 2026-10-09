// pattern: Functional Core
package gotestprofile

import (
	"polis/internal/fixture"
	"testing"
)

func TestFixtureSourcePolicyProtectsTrustedRunner(t *testing.T) {
	if !validateFixtureSourcePolicy("formatter.go", []byte(fixture.Source)) || !validateFixtureSourcePolicy("frozen_test.go", []byte(fixture.Tests("full"))) {
		t.Fatal("trusted fixture rejected")
	}
	for _, s := range []string{
		"package formatter; import \"os\"; func Render(v float64) string { os.Exit(0);return \"\" }",
		"package formatter; import \"testing\"; func TestMain(m *testing.M){}",
		"package formatter; import \"testing\"; func TestExtra(t *testing.T){testing.Main(nil,nil,nil,nil)}",
		"package formatter;func init(){}",
		"package formatter;var bypass=func()int{return 0}()",
		"package formatter;func Render(v float64)string{println(\"PASS\");return \"\"}",
		"package formatter;func Render(v float64)string{go func(){}();return \"\"}",
		"package formatter\n//go:linkname bypass os.Exit\nfunc bypass()",
	} {
		if validateFixtureSourcePolicy("extra_test.go", []byte(s)) {
			t.Fatalf("runner override accepted: %s", s)
		}
	}
}
