// pattern: Imperative Shell
package probe

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"polis/internal/codex"
)

func TestRequireBusinessAuthorizationAllowsExactBinding(t *testing.T) {
	dir := t.TempDir()
	context := businessAuthorizationShellContext()
	path := filepath.Join(dir, "authorization.json")
	writeAuthorizationFixture(t, path, codex.BusinessAuthorizationBindingFromContext(context))
	if err := RequireBusinessAuthorization(path, context); err != nil {
		t.Fatal(err)
	}
}

func TestRequireBusinessAuthorizationRejectsHistoricalT21BEmptyFingerprint(t *testing.T) {
	dir := t.TempDir()
	context := businessAuthorizationShellContext()
	binding := codex.BusinessAuthorizationBindingFromContext(context)
	binding.ExecutionFingerprint = ""
	path := filepath.Join(dir, "t21b-empty-fingerprint.json")
	writeAuthorizationFixture(t, path, binding)
	if err := RequireBusinessAuthorization(path, context); err == nil {
		t.Fatal("historical T21B empty fingerprint was accepted")
	}
}

func TestR03AT2AuthorizationContextBindsWorkerStartup(t *testing.T) {
	context := r03aT2BusinessAuthorizationContext(R03AT2Config{Config: Config{Model: "gpt-5.6-luna", MediumLimit: 1, HighLimit: 0, ToolCallLimit: 17}, ProblemKey: "problem", RunPurpose: "purpose", ExecutionFingerprint: "execution"})
	if context.ExecutionFingerprint != "execution" || context.EmployeeID != "emp-backend" || context.ProblemKey != "problem" || context.Purpose != "purpose" || context.Model != "gpt-5.6-luna" || context.Profile != "gpt-5.6-luna/medium" || context.Effort != "medium" || context.Limits != (codex.AllowanceLimits{Medium: 1, High: 0, Concurrency: 1, ToolCallLimit: 17}) {
		t.Fatalf("worker-start authorization context drifted: %+v", context)
	}
}

func businessAuthorizationShellContext() codex.BusinessExecutionContext {
	return codex.BusinessExecutionContext{ExecutionFingerprint: "execution", EmployeeID: "emp-backend", ProblemKey: "problem", Purpose: "backend purpose", Model: "gpt-5.6-luna", Profile: "gpt-5.6-luna/medium", Effort: "medium", Limits: codex.AllowanceLimits{Medium: 1, High: 0, Concurrency: 1, ToolCallLimit: 17}}
}

func writeAuthorizationFixture(t *testing.T, path string, binding codex.BusinessAuthorizationBinding) {
	t.Helper()
	raw, err := json.Marshal(binding)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}
