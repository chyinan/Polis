// pattern: Functional Core
package core

// TaskKind is the typed form of the persisted tasks.kind discriminator.
type TaskKind string

const (
	TaskKindBootstrapPlan TaskKind = "bootstrap_plan"
	TaskKindCompute       TaskKind = "compute"
	TaskKindCompat        TaskKind = "compat"
	TaskKindReview        TaskKind = "review"
	TaskKindPeerBackend   TaskKind = "peer_backend"
	TaskKindPeerFrontend  TaskKind = "peer_frontend"
	TaskKindPeerReview    TaskKind = "peer_review"

	EmployeePlanningID = "emp-planning"
	EmployeeBackendID  = "emp-backend"
	EmployeeFrontendID = "emp-frontend"
	EmployeeReviewID   = "emp-review"
)

// IsProductProviderExecutableTask classifies work for the current generic
// product provider adapter. Other task kinds belong to control, deterministic,
// or historical peer flows and are not authorized by the product adapter.
func IsProductProviderExecutableTask(kind TaskKind, ownerEmployeeID string) bool {
	return kind == TaskKindCompat && ownerEmployeeID == EmployeeBackendID
}
