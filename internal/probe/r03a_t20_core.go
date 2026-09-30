// pattern: Functional Core
package probe

const (
	T20Passed                  = "PASSED"
	T20Inconclusive            = "INCONCLUSIVE"
	T20L1Qualified             = "qualified"
	T20L1Unqualified           = "unqualified_for_business_execution"
	T20NoOutputReconnect       = "no_output_reconnect"
	T20StructuredProviderError = "structured_provider_error"
	T20StdioFailure            = "stdio_failure"
	T20AuthFailure             = "auth_failure"
)

type T20OutcomeInput struct {
	TurnStarted         bool
	FirstValidOutput    bool
	TurnCompleted       bool
	ReconnectCount      int
	UnresolvedTransport bool
	FailureMode         string
}

type T20Outcome struct {
	Result          string `json:"result"`
	L1Qualification string `json:"l1_qualification"`
	FailureMode     string `json:"failure_mode"`
	RuntimeFinding  string `json:"runtime_envelope_causal_evidence"`
}

func ClassifyT20Outcome(input T20OutcomeInput) T20Outcome {
	if input.TurnStarted && input.FirstValidOutput && input.TurnCompleted && !input.UnresolvedTransport {
		return T20Outcome{
			Result:          T20Passed,
			L1Qualification: T20L1Qualified,
			FailureMode:     "none",
			RuntimeFinding:  "STRONG_DIAGNOSTIC_EVIDENCE",
		}
	}

	failureMode := input.FailureMode
	if failureMode == "" {
		failureMode = T20StdioFailure
	}
	return T20Outcome{
		Result:          T20Inconclusive,
		L1Qualification: T20L1Unqualified,
		FailureMode:     failureMode,
		RuntimeFinding:  "NOT_ESTABLISHED",
	}
}

type T20ControlledFactors struct {
	CodexVersion                      string
	BinarySHA256                      string
	CodeModeHostSHA256                string
	Model                             string
	Effort                            string
	EffectiveConfigDigest             string
	EffectiveTransportConfigDigest    string
	AuthSourceClass                   string
	AuthIdentityFingerprint           string
	AuthCredentialRevisionFingerprint string
	ProxyPolicy                       string
	ProviderTransportPolicy           string
	DynamicToolCount                  string
	SandboxProfile                    string
	RuntimeProfile                    string
	CodexHomeProfile                  string
	CWDRole                           string
	PromptDigest                      string
	DeveloperInstructionDigest        string
	NativeProtocolSchemaDigest        string
	CapabilityDigest                  string
}

type T20FactorDiff struct {
	Field          string `json:"field"`
	Classification string `json:"classification"`
	Expected       string `json:"expected"`
	Actual         string `json:"actual"`
}

type T20FactorComparison struct {
	Passed  bool            `json:"passed"`
	Changed []T20FactorDiff `json:"changed"`
}

func CompareT20ControlledFactors(expected, actual T20ControlledFactors) T20FactorComparison {
	values := []struct {
		field    string
		expected string
		actual   string
	}{
		{"codex_version", expected.CodexVersion, actual.CodexVersion},
		{"binary_sha256", expected.BinarySHA256, actual.BinarySHA256},
		{"code_mode_host_sha256", expected.CodeModeHostSHA256, actual.CodeModeHostSHA256},
		{"model", expected.Model, actual.Model},
		{"effort", expected.Effort, actual.Effort},
		{"effective_config_digest", expected.EffectiveConfigDigest, actual.EffectiveConfigDigest},
		{"effective_transport_config_digest", expected.EffectiveTransportConfigDigest, actual.EffectiveTransportConfigDigest},
		{"auth_source_class", expected.AuthSourceClass, actual.AuthSourceClass},
		{"auth_identity_fingerprint", expected.AuthIdentityFingerprint, actual.AuthIdentityFingerprint},
		{"auth_credential_revision_fingerprint", expected.AuthCredentialRevisionFingerprint, actual.AuthCredentialRevisionFingerprint},
		{"proxy_policy", expected.ProxyPolicy, actual.ProxyPolicy},
		{"provider_transport_policy", expected.ProviderTransportPolicy, actual.ProviderTransportPolicy},
		{"dynamic_tool_count", expected.DynamicToolCount, actual.DynamicToolCount},
		{"sandbox_profile", expected.SandboxProfile, actual.SandboxProfile},
		{"runtime_profile", expected.RuntimeProfile, actual.RuntimeProfile},
		{"codex_home_profile", expected.CodexHomeProfile, actual.CodexHomeProfile},
		{"cwd_role", expected.CWDRole, actual.CWDRole},
		{"prompt_digest", expected.PromptDigest, actual.PromptDigest},
		{"developer_instruction_digest", expected.DeveloperInstructionDigest, actual.DeveloperInstructionDigest},
		{"native_protocol_schema_digest", expected.NativeProtocolSchemaDigest, actual.NativeProtocolSchemaDigest},
		{"capability_digest", expected.CapabilityDigest, actual.CapabilityDigest},
	}
	result := T20FactorComparison{Passed: true}
	for _, value := range values {
		if value.expected == value.actual {
			continue
		}
		result.Passed = false
		result.Changed = append(result.Changed, T20FactorDiff{Field: value.field, Classification: "unexpectedly-different", Expected: value.expected, Actual: value.actual})
	}
	return result
}
