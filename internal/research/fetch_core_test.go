// pattern: Functional Core
package research

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeFetchPlanBindsExactHTTPSOrigin(t *testing.T) {
	plan, err := NormalizeFetchPlan(FetchPlan{Origin: "https://Example.test:443", TargetURL: "https://example.test/docs", TimeoutMS: 5000, MaxBytes: 4096})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Origin != "https://example.test" || plan.TargetURL != "https://example.test/docs" {
		t.Fatalf("plan=%+v", plan)
	}
}

func TestNormalizeFetchPlanRejectsCrossOriginOrUnsafeInputs(t *testing.T) {
	base := FetchPlan{Origin: "https://example.test", TargetURL: "https://example.test/docs", TimeoutMS: 5000, MaxBytes: 4096}
	for name, mutate := range map[string]func(*FetchPlan){
		"http origin":  func(v *FetchPlan) { v.Origin = "http://example.test" },
		"cross origin": func(v *FetchPlan) { v.TargetURL = "https://other.test/docs" },
		"credentials":  func(v *FetchPlan) { v.TargetURL = "https://user:pass@example.test/docs" },
		"oversize":     func(v *FetchPlan) { v.MaxBytes = maxFetchBytes + 1 },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := base
			mutate(&candidate)
			if _, err := NormalizeFetchPlan(candidate); !errors.Is(err, ErrInvalidFetchPlan) {
				t.Fatalf("plan=%+v err=%v", candidate, err)
			}
		})
	}
}

func TestValidateFetchOutcomeRejectsRedirectAndOversize(t *testing.T) {
	plan, err := NormalizeFetchPlan(FetchPlan{Origin: "https://example.test", TargetURL: "https://example.test/docs", TimeoutMS: 5000, MaxBytes: 4096})
	if err != nil {
		t.Fatal(err)
	}
	body := []byte("fixture body")
	valid := FetchOutcome{State: "succeeded", ReasonCode: "research_fetch_succeeded", FinalURL: plan.TargetURL, StatusCode: 200, ContentType: "text/html", RedirectCount: 0, Body: body, BodySHA256: DigestBytes(body)}
	if err := ValidateFetchOutcome(plan, valid); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*FetchOutcome){
		"redirect":  func(v *FetchOutcome) { v.RedirectCount = 1 },
		"bad media": func(v *FetchOutcome) { v.ContentType = "application/octet-stream" },
		"oversize":  func(v *FetchOutcome) { v.Body = []byte(strings.Repeat("x", 4097)) },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := valid
			mutate(&candidate)
			if err := ValidateFetchOutcome(plan, candidate); !errors.Is(err, ErrFetchPolicyViolation) {
				t.Fatalf("outcome=%+v err=%v", candidate, err)
			}
		})
	}
}
