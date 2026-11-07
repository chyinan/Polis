// pattern: Functional Core
package probe

const (
	T19ConfirmedSame           = "confirmed_same"
	T19RuntimeDerivedDifferent = "runtime-derived-different"
	T19UnexpectedlyDifferent   = "unexpectedly-different"
	T19NotComparable           = "not-comparable"
	T19NotRecorded             = "not-recorded"
)

type T19NormalizedSide struct {
	Platform                          string
	Architecture                      string
	CodexVersion                      string
	BinarySHA256                      string
	CodeModeHostSHA256                string
	Invocation                        string
	Model                             string
	Effort                            string
	EffectiveConfigDigest             string
	EffectiveTransportConfigDigest    string
	AuthSourceClass                   string
	AuthIdentityFingerprint           string
	AuthCredentialRevisionFingerprint string
	ProxyPolicy                       string
	ProviderTransportPolicy           string
	DynamicToolCount                  int
	SandboxProfile                    string
	FilesystemIsolation               string
	ProcessIsolation                  string
	NetworkPolicy                     string
	CWDRole                           string
	PromptDigest                      string
	DeveloperInstructionDigest        string
	NativeProtocolSchemaDigest        string
	NativeProtocolCompatibility       string
	CapabilityDigest                  string
	CodexHomeProfile                  string
	StdioInputLineEnding              string
	StdioOutputLineEnding             string
	PipeSemantics                     string
	LaunchMechanism                   string
}

type T19FactorDiff struct {
	Field          string `json:"field"`
	Classification string `json:"classification"`
	A              string `json:"a"`
	B              string `json:"b"`
}

type T19FactorComparison struct {
	Classifications         map[string]string `json:"classifications"`
	ControlledSame          []T19FactorDiff   `json:"controlled_same"`
	RuntimeDerivedDifferent []T19FactorDiff   `json:"runtime_derived_different"`
	UnexpectedlyDifferent   []T19FactorDiff   `json:"unexpectedly_different"`
	NotComparable           []T19FactorDiff   `json:"not_comparable"`
	NotRecorded             []T19FactorDiff   `json:"not_recorded"`
	Eligible                bool              `json:"eligible"`
}

func CompareT19Factors(a, b T19NormalizedSide) T19FactorComparison {
	result := T19FactorComparison{Classifications: map[string]string{}}
	requiredSame := map[string][2]string{
		"architecture":                      {a.Architecture, b.Architecture},
		"codex_version":                     {a.CodexVersion, b.CodexVersion},
		"invocation":                        {a.Invocation, b.Invocation},
		"model":                             {a.Model, b.Model},
		"effort":                            {a.Effort, b.Effort},
		"effective_config_digest":           {a.EffectiveConfigDigest, b.EffectiveConfigDigest},
		"effective_transport_config_digest": {a.EffectiveTransportConfigDigest, b.EffectiveTransportConfigDigest},
		"auth_source_class":                 {a.AuthSourceClass, b.AuthSourceClass},
		"auth_identity":                     {a.AuthIdentityFingerprint, b.AuthIdentityFingerprint},
		"auth_credential_revision":          {a.AuthCredentialRevisionFingerprint, b.AuthCredentialRevisionFingerprint},
		"proxy_policy":                      {a.ProxyPolicy, b.ProxyPolicy},
		"provider_transport_policy":         {a.ProviderTransportPolicy, b.ProviderTransportPolicy},
		"dynamic_tool_count":                {intT19String(a.DynamicToolCount), intT19String(b.DynamicToolCount)},
		"sandbox_profile":                   {a.SandboxProfile, b.SandboxProfile},
		"cwd_role":                          {a.CWDRole, b.CWDRole},
		"prompt_digest":                     {a.PromptDigest, b.PromptDigest},
		"developer_instruction_digest":      {a.DeveloperInstructionDigest, b.DeveloperInstructionDigest},
		"native_protocol_schema_digest":     {a.NativeProtocolSchemaDigest, b.NativeProtocolSchemaDigest},
		"native_protocol_compatibility":     {a.NativeProtocolCompatibility, b.NativeProtocolCompatibility},
	}
	for field, values := range requiredSame {
		appendT19Diff(&result, field, values[0], values[1], T19ConfirmedSame, true)
	}
	runtimeDerived := map[string][2]string{
		"platform":                 {a.Platform, b.Platform},
		"binary_sha256":            {a.BinarySHA256, b.BinarySHA256},
		"code_mode_host_sha256":    {a.CodeModeHostSHA256, b.CodeModeHostSHA256},
		"codex_home_profile":       {a.CodexHomeProfile, b.CodexHomeProfile},
		"filesystem_isolation":     {a.FilesystemIsolation, b.FilesystemIsolation},
		"process_isolation":        {a.ProcessIsolation, b.ProcessIsolation},
		"network_policy":           {a.NetworkPolicy, b.NetworkPolicy},
		"capability_digest":        {a.CapabilityDigest, b.CapabilityDigest},
		"stdio_input_line_ending":  {a.StdioInputLineEnding, b.StdioInputLineEnding},
		"stdio_output_line_ending": {a.StdioOutputLineEnding, b.StdioOutputLineEnding},
		"pipe_semantics":           {a.PipeSemantics, b.PipeSemantics},
		"launch_mechanism":         {a.LaunchMechanism, b.LaunchMechanism},
	}
	for field, values := range runtimeDerived {
		classification := T19RuntimeDerivedDifferent
		if values[0] == values[1] {
			classification = T19ConfirmedSame
		}
		appendT19Diff(&result, field, values[0], values[1], classification, false)
	}
	result.Eligible = len(result.UnexpectedlyDifferent) == 0 && len(result.NotComparable) == 0 && len(result.NotRecorded) == 0
	return result
}

func appendT19Diff(result *T19FactorComparison, field, a, b, classification string, required bool) {
	if required && a != b && a != "" && b != "" && a != "unknown" && b != "unknown" {
		classification = T19UnexpectedlyDifferent
	}
	if a == "" || b == "" || a == "unknown" || b == "unknown" {
		classification = T19NotComparable
	}
	if a == T19NotRecorded || b == T19NotRecorded {
		classification = T19NotRecorded
	}
	diff := T19FactorDiff{Field: field, Classification: classification, A: a, B: b}
	result.Classifications[field] = classification
	if classification == T19ConfirmedSame {
		result.ControlledSame = append(result.ControlledSame, diff)
		return
	}
	if classification == T19RuntimeDerivedDifferent && !required {
		result.RuntimeDerivedDifferent = append(result.RuntimeDerivedDifferent, diff)
		return
	}
	if classification == T19UnexpectedlyDifferent || (required && a != b && classification == T19UnexpectedlyDifferent) {
		result.UnexpectedlyDifferent = append(result.UnexpectedlyDifferent, diff)
		return
	}
	if classification == T19NotRecorded {
		result.NotRecorded = append(result.NotRecorded, diff)
		return
	}
	result.NotComparable = append(result.NotComparable, diff)
}

func intT19String(value int) string {
	if value == 0 {
		return "0"
	}
	return "nonzero"
}
