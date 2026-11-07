// pattern: Functional Core
package codex

const (
	AuthorizationAllowedReasonCode             = "authorization_binding_exact"
	MissingExecutionFingerprintReasonCode      = "execution_fingerprint_missing"
	ExecutionFingerprintMismatchReasonCode     = "execution_fingerprint_mismatch"
	QualificationStaleReasonCode               = "qualification_stale"
	EmployeeIDMismatchReasonCode               = "employee_id_mismatch"
	ProblemKeyMismatchReasonCode               = "problem_key_mismatch"
	PurposeMismatchReasonCode                  = "purpose_mismatch"
	ModelMismatchReasonCode                    = "model_mismatch"
	ProfileMismatchReasonCode                  = "profile_mismatch"
	EffortMismatchReasonCode                   = "effort_mismatch"
	AllowanceLimitsMismatchReasonCode          = "allowance_limits_mismatch"
	AuthorizationFieldsMissingReasonCode       = "authorization_fields_missing"
	ToolCallLimitTooLargeReasonCode            = "tool_call_limit_exceeds_runtime_cap"
	TransportPolicyMissingReasonCode           = "transport_policy_binding_missing"
	TransportPolicyMismatchReasonCode          = "transport_policy_binding_mismatch"
	DatabaseBindingMissingReasonCode           = "database_binding_missing"
	DatabaseBindingMismatchReasonCode          = "database_binding_mismatch"
	FrontendConsumptionStackMissingReasonCode  = "frontend_consumption_stack_missing"
	FrontendConsumptionStackMismatchReasonCode = "frontend_consumption_stack_mismatch"
)

type AllowanceLimits struct {
	Medium        int `json:"medium_limit"`
	High          int `json:"high_limit"`
	Concurrency   int `json:"concurrency"`
	ToolCallLimit int `json:"tool_call_limit"`
}

type BusinessAuthorizationBinding struct {
	ExecutionFingerprint                string                  `json:"execution_fingerprint"`
	EmployeeID                          string                  `json:"employee_id"`
	ProblemKey                          string                  `json:"problem_key"`
	Purpose                             string                  `json:"purpose"`
	Model                               string                  `json:"model"`
	Profile                             string                  `json:"profile"`
	Effort                              string                  `json:"effort"`
	Limits                              AllowanceLimits         `json:"allowance_limits"`
	TransportPolicyRevision             string                  `json:"transport_policy_revision,omitempty"`
	TransportPolicy                     TransportPolicySnapshot `json:"transport_policy,omitempty"`
	DatabaseBindingFingerprint          string                  `json:"database_binding_fingerprint,omitempty"`
	DatabaseAccessStrategyRevision      string                  `json:"database_access_strategy_revision,omitempty"`
	FrontendConsumptionContractRevision string                  `json:"frontend_consumption_contract_revision,omitempty"`
	FrontendBindingContractRevision     string                  `json:"frontend_binding_contract_revision,omitempty"`
	FrontendBehaviorVerifierRevision    string                  `json:"frontend_behavior_verifier_revision,omitempty"`
}

type BusinessExecutionContext struct {
	ExecutionFingerprint                string
	EmployeeID                          string
	ProblemKey                          string
	Purpose                             string
	Model                               string
	Profile                             string
	Effort                              string
	Limits                              AllowanceLimits
	TransportPolicyRevision             string
	TransportPolicy                     TransportPolicySnapshot
	DatabaseBindingFingerprint          string
	DatabaseAccessStrategyRevision      string
	FrontendConsumptionContractRevision string
	FrontendBindingContractRevision     string
	FrontendBehaviorVerifierRevision    string
	QualificationStale                  bool
}

type AuthorizationDecision struct {
	Allowed    bool   `json:"allowed"`
	ReasonCode string `json:"reason_code"`
}

func BusinessAuthorizationBindingFromContext(context BusinessExecutionContext) BusinessAuthorizationBinding {
	return BusinessAuthorizationBinding{
		ExecutionFingerprint:                context.ExecutionFingerprint,
		EmployeeID:                          context.EmployeeID,
		ProblemKey:                          context.ProblemKey,
		Purpose:                             context.Purpose,
		Model:                               context.Model,
		Profile:                             context.Profile,
		Effort:                              context.Effort,
		Limits:                              context.Limits,
		TransportPolicyRevision:             context.TransportPolicyRevision,
		TransportPolicy:                     context.TransportPolicy,
		DatabaseBindingFingerprint:          context.DatabaseBindingFingerprint,
		DatabaseAccessStrategyRevision:      context.DatabaseAccessStrategyRevision,
		FrontendConsumptionContractRevision: context.FrontendConsumptionContractRevision,
		FrontendBindingContractRevision:     context.FrontendBindingContractRevision,
		FrontendBehaviorVerifierRevision:    context.FrontendBehaviorVerifierRevision,
	}
}

func AuthorizeBusinessExecution(binding BusinessAuthorizationBinding, context BusinessExecutionContext) AuthorizationDecision {
	if context.QualificationStale {
		return deniedAuthorization(QualificationStaleReasonCode)
	}
	if binding.ExecutionFingerprint == "" || context.ExecutionFingerprint == "" {
		return deniedAuthorization(MissingExecutionFingerprintReasonCode)
	}
	if binding.EmployeeID == "" || context.EmployeeID == "" || binding.ProblemKey == "" || context.ProblemKey == "" || binding.Purpose == "" || context.Purpose == "" || binding.Model == "" || context.Model == "" || binding.Profile == "" || context.Profile == "" || binding.Effort == "" || context.Effort == "" || binding.Limits.Medium < 1 || context.Limits.Medium < 1 || binding.Limits.High < 0 || context.Limits.High < 0 || binding.Limits.Concurrency < 1 || context.Limits.Concurrency < 1 || binding.Limits.ToolCallLimit < 1 || context.Limits.ToolCallLimit < 1 {
		return deniedAuthorization(AuthorizationFieldsMissingReasonCode)
	}
	if binding.ExecutionFingerprint != context.ExecutionFingerprint {
		return deniedAuthorization(ExecutionFingerprintMismatchReasonCode)
	}
	if binding.EmployeeID != context.EmployeeID {
		return deniedAuthorization(EmployeeIDMismatchReasonCode)
	}
	if binding.ProblemKey != context.ProblemKey {
		return deniedAuthorization(ProblemKeyMismatchReasonCode)
	}
	if binding.Purpose != context.Purpose {
		return deniedAuthorization(PurposeMismatchReasonCode)
	}
	if binding.Model != context.Model {
		return deniedAuthorization(ModelMismatchReasonCode)
	}
	if binding.Profile != context.Profile {
		return deniedAuthorization(ProfileMismatchReasonCode)
	}
	if binding.Effort != context.Effort {
		return deniedAuthorization(EffortMismatchReasonCode)
	}
	if binding.Limits != context.Limits {
		return deniedAuthorization(AllowanceLimitsMismatchReasonCode)
	}
	if binding.Limits.ToolCallLimit > RuntimeToolCallSafetyCap || context.Limits.ToolCallLimit > RuntimeToolCallSafetyCap {
		return deniedAuthorization(ToolCallLimitTooLargeReasonCode)
	}
	if binding.TransportPolicyRevision != "" || context.TransportPolicyRevision != "" || binding.TransportPolicy.Revision != "" || context.TransportPolicy.Revision != "" {
		if binding.TransportPolicyRevision == "" || context.TransportPolicyRevision == "" || binding.TransportPolicy.Revision == "" || context.TransportPolicy.Revision == "" {
			return deniedAuthorization(TransportPolicyMissingReasonCode)
		}
		if binding.TransportPolicyRevision != context.TransportPolicyRevision || binding.TransportPolicyRevision != binding.TransportPolicy.Revision || context.TransportPolicyRevision != context.TransportPolicy.Revision || binding.TransportPolicy != context.TransportPolicy {
			return deniedAuthorization(TransportPolicyMismatchReasonCode)
		}
	}
	if binding.DatabaseBindingFingerprint != "" || context.DatabaseBindingFingerprint != "" || binding.DatabaseAccessStrategyRevision != "" || context.DatabaseAccessStrategyRevision != "" {
		if binding.DatabaseBindingFingerprint == "" || context.DatabaseBindingFingerprint == "" || binding.DatabaseAccessStrategyRevision == "" || context.DatabaseAccessStrategyRevision == "" {
			return deniedAuthorization(DatabaseBindingMissingReasonCode)
		}
		if binding.DatabaseBindingFingerprint != context.DatabaseBindingFingerprint || binding.DatabaseAccessStrategyRevision != context.DatabaseAccessStrategyRevision {
			return deniedAuthorization(DatabaseBindingMismatchReasonCode)
		}
	}
	if binding.FrontendConsumptionContractRevision != "" || context.FrontendConsumptionContractRevision != "" || binding.FrontendBindingContractRevision != "" || context.FrontendBindingContractRevision != "" || binding.FrontendBehaviorVerifierRevision != "" || context.FrontendBehaviorVerifierRevision != "" {
		if binding.FrontendConsumptionContractRevision == "" || context.FrontendConsumptionContractRevision == "" || binding.FrontendBindingContractRevision == "" || context.FrontendBindingContractRevision == "" || binding.FrontendBehaviorVerifierRevision == "" || context.FrontendBehaviorVerifierRevision == "" {
			return deniedAuthorization(FrontendConsumptionStackMissingReasonCode)
		}
		if binding.FrontendConsumptionContractRevision != context.FrontendConsumptionContractRevision || binding.FrontendBindingContractRevision != context.FrontendBindingContractRevision || binding.FrontendBehaviorVerifierRevision != context.FrontendBehaviorVerifierRevision {
			return deniedAuthorization(FrontendConsumptionStackMismatchReasonCode)
		}
	}
	return AuthorizationDecision{Allowed: true, ReasonCode: AuthorizationAllowedReasonCode}
}

func deniedAuthorization(reasonCode string) AuthorizationDecision {
	return AuthorizationDecision{Allowed: false, ReasonCode: reasonCode}
}
