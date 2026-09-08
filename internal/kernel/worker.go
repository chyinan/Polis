// pattern: Imperative Shell
package kernel

type Workspace struct {
	Digest, Content string
	Revision        int64
}
type HandoverBundle struct {
	EmployeeID  string
	Task        Task
	Workspace   Workspace
	Obligations []Obligation
	Checkpoints []Checkpoint
	CompanySeq  int64
	Contract    string
}
type Checkpoint struct {
	Summary   string   `json:"summary"`
	Facts     []string `json:"facts"`
	Decisions []string `json:"decisions"`
	Rejected  []string `json:"rejected"`
	Evidence  []string `json:"evidence"`
}

func (b Binding) SessionID() string { return b.session }
