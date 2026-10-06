// pattern: Functional Core
package kernel

import (
	"errors"
	"strings"
	"testing"

	"polis/internal/core"
)

func TestCanonicalBrowserRunOriginAllowsOnlyHTTPSOrigins(t *testing.T) {
	got, err := canonicalBrowserRunOrigin("https://Example.test/")
	if err != nil || got != "https://example.test" {
		t.Fatalf("canonical origin=%q err=%v, want https://example.test", got, err)
	}
	got, err = canonicalBrowserRunOrigin("https://[2001:DB8::1]:8443")
	if err != nil || got != "https://[2001:db8::1]:8443" {
		t.Fatalf("IPv6 canonical origin=%q err=%v", got, err)
	}
	for _, raw := range []string{
		"http://example.test",
		"https://user:password@example.test",
		"https://example.test/app",
		"https://example.test/?next=https://other.test",
		"https://example.test#fragment",
	} {
		if _, err := canonicalBrowserRunOrigin(raw); !errors.Is(err, core.Denied) {
			t.Fatalf("origin %q returned %v, want %v", raw, err, core.Denied)
		}
	}
}

func TestNormalizeBrowserRunRequestBoundsMetadataAndPlan(t *testing.T) {
	valid := BrowserRunRequest{
		ServiceJobID: "service-job", ServiceGeneration: 2, TargetOrigin: "https://example.test/",
		PlanSHA256: strings.Repeat("a", 64), BrowserBuild: "chromium-1", ExecutionEnvironment: "windows-managed-profile",
		InputRevision: "input-1", ViewportWidth: 1280, ViewportHeight: 720, Locale: "zh-CN", Timezone: "Asia/Shanghai",
	}
	got, err := normalizeBrowserRunRequest(valid)
	if err != nil || got.TargetOrigin != "https://example.test" {
		t.Fatalf("valid BrowserRun request=%+v err=%v", got, err)
	}
	for name, mutate := range map[string]func(*BrowserRunRequest){
		"bad plan":               func(input *BrowserRunRequest) { input.PlanSHA256 = "not-a-digest" },
		"missing input revision": func(input *BrowserRunRequest) { input.InputRevision = "" },
		"oversized viewport":     func(input *BrowserRunRequest) { input.ViewportWidth = 4097 },
		"missing timezone":       func(input *BrowserRunRequest) { input.Timezone = "" },
	} {
		t.Run(name, func(t *testing.T) {
			input := valid
			mutate(&input)
			if _, err := normalizeBrowserRunRequest(input); !errors.Is(err, core.Malformed) {
				t.Fatalf("normalize error=%v, want %v", err, core.Malformed)
			}
		})
	}
}
