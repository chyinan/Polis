// pattern: Functional Core
package gotestsupervisor

// An inactive template immediately after --no-block is not proof that the
// queued job ever started. This path requires a durable observed invocation.
func unitTerminal(properties map[string]string) bool {
	return properties["InvocationID"] != "" && (properties["ActiveState"] == "inactive" || properties["ActiveState"] == "failed")
}

// systemd may clear InvocationID before the controller's next poll after a
// short-lived unit exits. Only a successful terminal unit is eligible for the
// separate, new-directory result-file verification fallback.
func unitResultMayBeReady(properties map[string]string) bool {
	return properties["InvocationID"] == "" &&
		(properties["ActiveState"] == "inactive" || properties["ActiveState"] == "failed") &&
		properties["Result"] == "success" && properties["ExecMainStatus"] == "0"
}
