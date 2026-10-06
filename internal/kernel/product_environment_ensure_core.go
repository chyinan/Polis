// pattern: Functional Core
package kernel

import "polis/internal/core"

func validateProductTaskEnvironmentEnsureScope(binding Binding, revisionMissionID, currentMissionID, currentTaskID string) error {
	if binding.scope.company == "" || binding.task == "" || binding.session == "" || revisionMissionID == "" || currentMissionID == "" || revisionMissionID != currentMissionID || currentTaskID != binding.task {
		return core.OutOfScope
	}
	return nil
}
