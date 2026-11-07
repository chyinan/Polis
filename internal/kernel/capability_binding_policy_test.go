package kernel

import (
	"errors"
	"testing"

	"polis/internal/core"
)

func TestValidateEmployeeCapabilityBindingRequiresApprovedSkills(t *testing.T) {
	approved := []CapabilityRef{{ID: "skill-1", Status: "approved"}}
	if err := validateEmployeeCapabilityBinding("emp-backend", "draft", approved, nil); err != nil {
		t.Fatal(err)
	}
	if err := validateEmployeeCapabilityBinding("emp-backend", "approved", nil, []CapabilityRef{{ID: "mcp-1", Status: "unverified"}}); err != nil {
		t.Fatal(err)
	}
	if err := validateEmployeeCapabilityBinding("emp-backend", "approved", []CapabilityRef{{ID: "skill-1", Status: "candidate"}}, nil); !errors.Is(err, core.Denied) {
		t.Fatalf("candidate skill err = %v", err)
	}
	if err := validateEmployeeCapabilityBinding("emp-backend", "approved", nil, nil); !errors.Is(err, core.Denied) {
		t.Fatalf("empty approved err = %v", err)
	}
	if err := validateEmployeeCapabilityBinding("emp-backend", "draft", nil, []CapabilityRef{{ID: "mcp-1", Status: "revoked"}}); !errors.Is(err, core.Denied) {
		t.Fatalf("revoked mcp err = %v", err)
	}
	if err := validateEmployeeCapabilityBinding("someone", "draft", approved, nil); !errors.Is(err, core.Malformed) {
		t.Fatalf("employee err = %v", err)
	}
}

func TestNextSkillCatalogStatusIsIdempotentAndDoesNotReviveRevoked(t *testing.T) {
	next, err := nextSkillCatalogStatus("candidate", "approve")
	if err != nil || next != "approved" {
		t.Fatalf("approve = %s %v", next, err)
	}
	next, err = nextSkillCatalogStatus("approved", "approve")
	if err != nil || next != "approved" {
		t.Fatalf("repeat = %s %v", next, err)
	}
	if _, err = nextSkillCatalogStatus("revoked", "approve"); !errors.Is(err, core.Denied) {
		t.Fatalf("revive err = %v", err)
	}
	next, err = nextSkillCatalogStatus("approved", "revoke")
	if err != nil || next != "revoked" {
		t.Fatalf("revoke = %s %v", next, err)
	}
}
