// pattern: Functional Core
package gotestsupervisor

// An inactive template immediately after --no-block is not proof that the
// queued job ever started. Collection requires an observed invocation.
func unitTerminal(properties map[string]string) bool {
	return properties["InvocationID"] != "" && (properties["ActiveState"] == "inactive" || properties["ActiveState"] == "failed")
}
