// pattern: Imperative Shell
package kernel

import "context"

// ProductTaskJobStartCommand contains only the bounded, owner-approved project
// job fields. Company, Task, and WorkerSession identity always come from Binding.
type ProductTaskJobStartCommand struct {
	EnvironmentRevisionID string
	Kind                  string
	ServiceID             string
	ScriptPath            string
	Args                  []string
	RequestID             string
}

// ProductTaskJobLifecycle bridges the fake-only Worker tool surface to the
// existing Control lifecycle. Implementations must recheck the exact binding;
// absence of this adapter keeps the surface fail-closed.
type ProductTaskJobLifecycle interface {
	StartProductTaskJob(context.Context, Binding, ProductTaskJobStartCommand) (JobRunRecord, error)
	StopProductTaskJob(context.Context, Binding, string, string) (JobRunRecord, error)
}
