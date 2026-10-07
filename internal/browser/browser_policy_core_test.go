// pattern: Functional Core
package browser

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeBrowserRunPlanCanonicalizesHTTPSOrigin(t *testing.T) {
	plan, err := NormalizeBrowserRunPlan(BrowserRunPlan{TargetOrigin: "https://Example.test:443/", TimeoutMS: 5000, MaxRequests: 32, MaxResponseBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	if plan.TargetOrigin != "https://example.test" {
		t.Fatalf("target origin=%q", plan.TargetOrigin)
	}
}

func TestNormalizeBrowserRunPlanRejectsNonOriginTargets(t *testing.T) {
	base := BrowserRunPlan{TargetOrigin: "https://example.test", TimeoutMS: 5000, MaxRequests: 32, MaxResponseBytes: 1 << 20}
	for _, target := range []string{"http://example.test", "https://user:pass@example.test", "https://example.test/path", "https://example.test?x=1", "https://example.test#fragment"} {
		base.TargetOrigin = target
		if _, err := NormalizeBrowserRunPlan(base); !errors.Is(err, ErrInvalidBrowserRunPlan) {
			t.Fatalf("target=%q err=%v, want invalid plan", target, err)
		}
	}
}

func TestNormalizeBrowserRunPlanOnlyAllowsInsecureTLSForLoopbackFixtures(t *testing.T) {
	plan := BrowserRunPlan{TargetOrigin: "https://example.test", TimeoutMS: 5000, MaxRequests: 32, MaxResponseBytes: 1 << 20, TestOnlyAllowInsecureTLS: true}
	if _, err := NormalizeBrowserRunPlan(plan); !errors.Is(err, ErrInvalidBrowserRunPlan) {
		t.Fatal("insecure TLS was allowed for a non-loopback origin")
	}
	plan.TargetOrigin = "https://127.0.0.1:45169"
	if _, err := NormalizeBrowserRunPlan(plan); err != nil {
		t.Fatal(err)
	}
}

func TestValidateBrowserRunOutcomeBindsFinalOriginAndEgressControls(t *testing.T) {
	plan, err := NormalizeBrowserRunPlan(BrowserRunPlan{TargetOrigin: "https://example.test", TimeoutMS: 5000, MaxRequests: 32, MaxResponseBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	valid := BrowserRunOutcome{
		Protocol:            BrowserRunnerProtocol,
		State:               "succeeded",
		ReasonCode:          "browser_run_succeeded",
		FinalURL:            "https://example.test/app",
		Title:               "fixture",
		Text:                "rendered",
		RequestCount:        4,
		BlockedRequestCount: 1,
		ResponseBytes:       2048,
	}
	if err := ValidateBrowserRunOutcome(plan, valid); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*BrowserRunOutcome){
		"cross-origin final url": func(v *BrowserRunOutcome) { v.FinalURL = "https://other.test/" },
		"download":               func(v *BrowserRunOutcome) { v.DownloadCount = 1 },
		"websocket":              func(v *BrowserRunOutcome) { v.WebSocketCount = 1 },
		"request bound":          func(v *BrowserRunOutcome) { v.RequestCount = 33 },
		"response bound":         func(v *BrowserRunOutcome) { v.ResponseBytes = 1<<20 + 1 },
		"text bound":             func(v *BrowserRunOutcome) { v.Text = strings.Repeat("x", maxBrowserTextBytes+1) },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := valid
			mutate(&candidate)
			if err := ValidateBrowserRunOutcome(plan, candidate); !errors.Is(err, ErrBrowserPolicyViolation) {
				t.Fatalf("outcome err=%v, want policy violation", err)
			}
		})
	}
}
