// pattern: Functional Core
package kernel

import (
	"errors"
	"testing"

	"polis/internal/core"
)

func TestCapabilityCatalogCompanyStateRejectsArchivedCompany(t *testing.T) {
	if err := validateCapabilityCatalogCompanyState("archived"); !errors.Is(err, core.Denied) {
		t.Fatalf("archived state error=%v, want %s", err, core.Denied)
	}
}

func TestCapabilityCatalogCompanyStateAllowsActiveCompany(t *testing.T) {
	if err := validateCapabilityCatalogCompanyState("active"); err != nil {
		t.Fatalf("active state error=%v", err)
	}
}
