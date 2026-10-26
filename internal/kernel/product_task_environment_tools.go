// pattern: Imperative Shell
package kernel

import "context"

// ProductTaskEnvironmentLifecycle bridges a current Worker request to the
// qualified Control preparation executor. Binding identity is supplied by the
// trusted Worker adapter, never by tool arguments.
type ProductTaskEnvironmentLifecycle interface {
	EnsureProductTaskEnvironment(context.Context, Binding, string, string) (EnvironmentPreparationRun, error)
}
