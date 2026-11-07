package taskvalidation

import "strings"

type Status string

const (
	StatusPass                 Status = "PASS"
	StatusFail                 Status = "FAIL"
	StatusNotConfigured        Status = "VALIDATION_NOT_CONFIGURED"
	StatusValidatorUnavailable Status = "VALIDATOR_UNAVAILABLE"
	StatusInfrastructureError  Status = "INFRASTRUCTURE_ERROR"
)

type Finding struct {
	Code         string `json:"code"`
	RequiredText string `json:"required_text,omitempty"`
	Feedback     string `json:"feedback"`
}

type Result struct {
	Status              Status    `json:"status"`
	ReasonCode          string    `json:"reason_code"`
	Feedback            string    `json:"feedback"`
	Findings            []Finding `json:"findings,omitempty"`
	TaskID              string    `json:"task_id,omitempty"`
	MissionID           string    `json:"mission_id,omitempty"`
	AcceptanceRevision  string    `json:"acceptance_revision,omitempty"`
	RunnerKind          string    `json:"runner_kind,omitempty"`
	RunnerRevision      string    `json:"runner_revision,omitempty"`
	ConfigurationDigest string    `json:"configuration_digest,omitempty"`
	WorkspaceDigest     string    `json:"workspace_digest,omitempty"`
	WorkspaceRevision   int64     `json:"workspace_revision,omitempty"`
	SessionID           string    `json:"session_id,omitempty"`
	Epoch               int64     `json:"epoch,omitempty"`
}

type validatorKey struct {
	acceptanceRevision string
	runnerKind         string
	runnerRevision     string
}

// Registry selects validators by the trusted, frozen Task binding. The
// current product registry deliberately contains one domain-generic validator.
type Registry struct {
	validators map[validatorKey]struct{}
}

func DefaultRegistry() Registry {
	return Registry{validators: map[validatorKey]struct{}{
		{acceptanceRevision: AcceptanceContractRevision, runnerKind: TextContainsAllRunnerKind, runnerRevision: TextContainsAllRunnerRevision}: {},
	}}
}

func (r Registry) Validate(binding *Binding, content string) Result {
	if binding == nil {
		return Result{
			Status:     StatusNotConfigured,
			ReasonCode: "validation_not_configured",
			Feedback:   "This Task has no public acceptance contract. Exploration is allowed, but a qualified checkpoint and artifact submission are not.",
		}
	}
	key := validatorKey{binding.AcceptanceRevision, binding.RunnerKind, binding.RunnerRevision}
	if _, available := r.validators[key]; !available {
		return resultFor(binding, StatusValidatorUnavailable, "validator_unavailable", "The Task's configured validator is unavailable; use a supported acceptance contract or restore its validator revision.")
	}
	if binding.TaskID == "" || binding.MissionID == "" || ValidateContract(&binding.Contract) != nil || binding.ConfigurationDigest == "" || ConfigurationDigest(*binding) != binding.ConfigurationDigest {
		return resultFor(binding, StatusInfrastructureError, "binding_integrity_error", "The persisted Task validation binding is incomplete or its configuration digest does not match; contact the controller.")
	}
	findings := make([]Finding, 0)
	for _, required := range binding.Contract.RequiredText {
		if !strings.Contains(content, required) {
			findings = append(findings, Finding{
				Code:         "required_text_missing",
				RequiredText: required,
				Feedback:     "Add this public acceptance text to the current Task workspace: " + required,
			})
		}
	}
	if len(findings) != 0 {
		return Result{
			Status:              StatusFail,
			ReasonCode:          "required_text_missing",
			Feedback:            "The current Task workspace is missing one or more required public acceptance criteria.",
			Findings:            findings,
			TaskID:              binding.TaskID,
			MissionID:           binding.MissionID,
			AcceptanceRevision:  binding.AcceptanceRevision,
			RunnerKind:          binding.RunnerKind,
			RunnerRevision:      binding.RunnerRevision,
			ConfigurationDigest: binding.ConfigurationDigest,
		}
	}
	return resultFor(binding, StatusPass, "all_required_text_present", "All public required-text criteria are present in the current Task workspace.")
}

func resultFor(binding *Binding, status Status, reason, feedback string) Result {
	return Result{
		Status:              status,
		ReasonCode:          reason,
		Feedback:            feedback,
		TaskID:              binding.TaskID,
		MissionID:           binding.MissionID,
		AcceptanceRevision:  binding.AcceptanceRevision,
		RunnerKind:          binding.RunnerKind,
		RunnerRevision:      binding.RunnerRevision,
		ConfigurationDigest: binding.ConfigurationDigest,
	}
}
