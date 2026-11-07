// pattern: Functional Core
package kernel

import "polis/internal/core"

type CapabilityRef struct {
	ID     string
	Status string
}

func validateEmployeeCapabilityBinding(employeeID, status string, skills, mcps []CapabilityRef) error {
	if !validEmployeeID(employeeID) || (status != "draft" && status != "approved" && status != "disabled" && status != "revoked") {
		return core.Malformed
	}
	if len(skills) > maxManifestListItems || len(mcps) > maxManifestListItems {
		return core.TooLarge
	}
	if err := validateBindingRefs(skills, true); err != nil {
		return err
	}
	if err := validateBindingRefs(mcps, false); err != nil {
		return err
	}
	if status == "approved" && len(skills) == 0 && len(mcps) == 0 {
		return core.Denied
	}
	return nil
}

func validateBindingRefs(refs []CapabilityRef, skills bool) error {
	seen := map[string]struct{}{}
	for _, ref := range refs {
		if !core.ValidID(ref.ID) {
			return core.Malformed
		}
		if _, ok := seen[ref.ID]; ok {
			return core.Malformed
		}
		seen[ref.ID] = struct{}{}
		if skills {
			if ref.Status != "approved" {
				return core.Denied
			}
			continue
		}
		if ref.Status == "revoked" || ref.Status == "" {
			return core.Denied
		}
	}
	return nil
}

func validEmployeeID(id string) bool {
	switch id {
	case "emp-planning", "emp-backend", "emp-frontend", "emp-review":
		return true
	default:
		return false
	}
}

func nextSkillCatalogStatus(current, action string) (string, error) {
	switch action {
	case "approve":
		if current == "candidate" || current == "approved" {
			return "approved", nil
		}
	case "revoke":
		if current == "candidate" || current == "approved" || current == "revoked" {
			return "revoked", nil
		}
	}
	return "", core.Denied
}
