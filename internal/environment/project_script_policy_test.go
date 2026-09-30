// pattern: Functional Core
package environment

import (
	"strings"
	"testing"
)

func TestNormalizeNodeProjectScriptPathAcceptsOnlyCanonicalProjectScripts(t *testing.T) {
	for _, value := range []string{"scripts/build.mjs", "src/check.cjs", "test/run.js"} {
		got, err := NormalizeNodeProjectScriptPath(value)
		if err != nil || got != value {
			t.Fatalf("NormalizeNodeProjectScriptPath(%q) = %q, %v", value, got, err)
		}
	}
	for _, value := range []string{"", ".", "../build.js", "scripts/../build.js", "/scripts/build.js", "C:/scripts/build.js", `scripts\build.js`, "scripts/build.cmd", "scripts/./build.js", "scripts/build.js\x00"} {
		if _, err := NormalizeNodeProjectScriptPath(value); err == nil {
			t.Errorf("unsafe or unsupported script path %q was accepted", value)
		}
	}
}

func TestValidateNodeProjectScriptArgsBoundsCountSizeAndNUL(t *testing.T) {
	if err := ValidateNodeProjectScriptArgs([]string{"--mode", "test"}); err != nil {
		t.Fatalf("valid bounded script args rejected: %v", err)
	}
	tooMany := make([]string, MaxNodeProjectScriptArgs+1)
	if err := ValidateNodeProjectScriptArgs(tooMany); err == nil {
		t.Fatal("too many script arguments were accepted")
	}
	if err := ValidateNodeProjectScriptArgs([]string{strings.Repeat("x", MaxNodeProjectScriptArgBytes+1)}); err == nil {
		t.Fatal("oversized script argument was accepted")
	}
	tooLarge := make([]string, 9)
	for index := range tooLarge {
		tooLarge[index] = strings.Repeat("x", MaxNodeProjectScriptArgBytes)
	}
	if err := ValidateNodeProjectScriptArgs(tooLarge); err == nil {
		t.Fatal("arguments over the aggregate command bound were accepted")
	}
	if err := ValidateNodeProjectScriptArgs([]string{"bad\x00arg"}); err == nil {
		t.Fatal("NUL in script argument was accepted")
	}
}
