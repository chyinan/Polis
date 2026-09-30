// pattern: Functional Core
package control

import (
	"context"
	"os"
	"testing"

	"polis/internal/kernel"
	"polis/internal/organization"
)

func TestValidateCreateCompanyRequestRequiresRequestIDAndRoster(t *testing.T) {
	draft := organization.CompanyDraft{ID: "company", Name: "Company", WorkspaceRoot: ".", Roster: organization.DefaultRoster()}
	if err := validateCreateCompanyRequest(CreateCompanyRequest{CompanyDraft: draft, RequestID: "request-1"}); err != nil {
		t.Fatalf("validateCreateCompanyRequest: %v", err)
	}
	if err := validateCreateCompanyRequest(CreateCompanyRequest{CompanyDraft: draft, RequestID: ""}); err == nil {
		t.Fatal("validateCreateCompanyRequest unexpectedly accepted missing request id")
	}
}

func TestRuntimeSettingsUpdatePersistsDesiredNonSecretConfiguration(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	company := "r06-runtime-settings"
	if _, err = k.TXCreateCompany(ctx, company); err != nil {
		t.Fatal(err)
	}
	service := NewService(k, &recordingWorkerAdapter{})
	receipt, err := service.UpdateRuntimeSettings(ctx, company, UpdateRuntimeSettingsRequest{Provider: "codex", Model: "gpt-5.6-luna", Effort: "medium", Profile: "gpt-5.6-luna/medium", RequestID: "runtime-settings-1"})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.ResultingState != "restart_required" {
		t.Fatalf("resulting state = %q, want restart_required", receipt.ResultingState)
	}
	settings, err := service.GetRuntimeSettings(ctx, company)
	if err != nil {
		t.Fatal(err)
	}
	if settings.Provider != "codex" || settings.Model != "gpt-5.6-luna" || settings.RuntimeReadiness != "restart_required" {
		t.Fatalf("unexpected runtime settings: %+v", settings)
	}
}
