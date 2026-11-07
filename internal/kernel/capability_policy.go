// pattern: Functional Core
package kernel

import "polis/internal/core"

func validateCapabilityCatalogCompanyState(state string) error {
	switch state {
	case "active":
		return nil
	case "archived":
		return core.Denied
	default:
		return core.OutOfScope
	}
}
