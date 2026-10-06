// pattern: Functional Core
package kernel

import "polis/internal/core"

func validateProductTaskJobReadScope(binding Binding, record JobRunRecord, activeTaskID, activeSessionID string) error {
	if binding.scope.company == "" || binding.task == "" || binding.session == "" || record.CompanyID != binding.scope.company || record.TaskID != binding.task || record.SessionID != binding.session || activeTaskID != binding.task || activeSessionID != binding.session {
		return core.OutOfScope
	}
	return nil
}
