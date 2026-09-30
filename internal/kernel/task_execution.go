// pattern: Functional Core
package kernel

import (
	"encoding/hex"

	"polis/internal/core"
	"polis/internal/taskvalidation"
)

// IsProductProviderExecutable identifies product employee work from persisted
// Task kind and assignee. Mission planning/control Tasks are never product work.
func (t Task) IsProductProviderExecutable() bool {
	return core.IsProductProviderExecutableTask(t.Kind, t.Owner)
}

// HasValidProductValidationBinding verifies the immutable public validator
// snapshot attached to this provider-executable Task.
func (t Task) HasValidProductValidationBinding() bool {
	binding := t.ValidationBinding
	return t.IsProductProviderExecutable() && binding != nil && binding.TaskID == t.ID && binding.MissionID == t.Mission &&
		taskvalidation.ValidateContract(&binding.Contract) == nil && binding.ConfigurationDigest != "" &&
		taskvalidation.ConfigurationDigest(*binding) == binding.ConfigurationDigest
}

// IsValidProductWorkspaceCAS checks the initial workspace identity captured by
// a provider WorkerSession before any product workspace write can occur.
func IsValidProductWorkspaceCAS(digest string, revision int64) bool {
	if len(digest) != 64 || revision <= 0 {
		return false
	}
	_, err := hex.DecodeString(digest)
	return err == nil
}

// SelectSingleProductProviderTask fails closed unless exactly one product
// provider-executable Task exists in a Mission.
func SelectSingleProductProviderTask(tasks []Task) (Task, error) {
	var selected Task
	found := false
	for _, task := range tasks {
		if !task.IsProductProviderExecutable() {
			continue
		}
		if found {
			return Task{}, core.Conflict
		}
		selected = task
		found = true
	}
	if !found {
		return Task{}, core.Denied
	}
	return selected, nil
}
