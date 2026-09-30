// pattern: Imperative Shell
package kernel

import (
	"context"
	"os"
	"testing"

	"polis/internal/organization"
)

func TestCompanyOrganizationRoundTrip(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required: run scripts/test.sh")
	}
	ctx := context.Background()
	k, err := Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()

	draft := organization.CompanyDraft{
		ID:            "r06-organization-roundtrip",
		Name:          "Dogfood Lab",
		WorkspaceRoot: "C:/workspace/dogfood",
		Roster:        organization.DefaultRoster(),
	}
	if _, err := k.TXCreateCompanyWithOrganization(ctx, draft); err != nil {
		t.Fatalf("TXCreateCompanyWithOrganization: %v", err)
	}
	details, err := k.CompanyDetails(ctx, draft.ID)
	if err != nil {
		t.Fatalf("CompanyDetails: %v", err)
	}
	if details.Name != draft.Name || details.WorkspaceRoot != draft.WorkspaceRoot || details.State != "active" {
		t.Fatalf("unexpected company details: %+v", details)
	}
	if len(details.Roster) != 4 || details.Roster[1].ModelProfile != "deterministic/fake" {
		t.Fatalf("unexpected company roster: %+v", details.Roster)
	}
}
