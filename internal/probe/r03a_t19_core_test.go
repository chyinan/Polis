// pattern: Functional Core
package probe

import "testing"

func TestCompareT19ClassifiesRuntimeDifferencesWithoutUnexpectedFactors(t *testing.T) {
	linux := t19TestSide("linux")
	windows := t19TestSide("windows")
	diff := CompareT19Factors(linux, windows)
	if len(diff.UnexpectedlyDifferent) != 0 || len(diff.NotComparable) != 0 {
		t.Fatalf("unexpected T19 diff = %+v", diff)
	}
	if diff.Classifications["model"] != T19ConfirmedSame || diff.Classifications["effort"] != T19ConfirmedSame || diff.Classifications["auth_identity"] != T19ConfirmedSame || diff.Classifications["effective_config_digest"] != T19ConfirmedSame {
		t.Fatalf("controlled-same classifications = %+v", diff.Classifications)
	}
	if diff.Classifications["platform"] != T19RuntimeDerivedDifferent || diff.Classifications["filesystem_isolation"] != T19RuntimeDerivedDifferent || diff.Classifications["network_policy"] != T19RuntimeDerivedDifferent {
		t.Fatalf("runtime classifications = %+v", diff.Classifications)
	}
	if !diff.Eligible {
		t.Fatalf("controlled T19 sides were not eligible: %+v", diff)
	}
}

func TestCompareT19MarksChangedControlledFactorUnexpected(t *testing.T) {
	linux := t19TestSide("linux")
	windows := t19TestSide("windows")
	windows.Effort = "xhigh"
	diff := CompareT19Factors(linux, windows)
	if diff.Eligible || diff.Classifications["effort"] != T19UnexpectedlyDifferent {
		t.Fatalf("changed effort was not rejected: %+v", diff)
	}
}

func t19TestSide(platform string) T19NormalizedSide {
	side := T19NormalizedSide{Platform: platform, Architecture: "amd64", CodexVersion: "0.153.4", BinarySHA256: "binary-" + platform, CodeModeHostSHA256: "host-" + platform, Invocation: "app-server --stdio", Model: "gpt-5.6-luna", Effort: "medium", EffectiveConfigDigest: "config", EffectiveTransportConfigDigest: "transport", AuthSourceClass: "controlled_diagnostic_auth_material", AuthIdentityFingerprint: "identity", AuthCredentialRevisionFingerprint: "revision", ProxyPolicy: "no_injected_proxy", ProviderTransportPolicy: "native_default", DynamicToolCount: 0, SandboxProfile: "read-only", FilesystemIsolation: "fs-" + platform, ProcessIsolation: "process-" + platform, NetworkPolicy: "network-" + platform, CWDRole: "diagnostic_workspace", PromptDigest: "prompt", DeveloperInstructionDigest: "developer", NativeProtocolSchemaDigest: "protocol", NativeProtocolCompatibility: "passed", CapabilityDigest: "cap-" + platform, CodexHomeProfile: "home-" + platform, StdioInputLineEnding: "LF", StdioOutputLineEnding: "LF", PipeSemantics: "pipes", LaunchMechanism: "launch-" + platform}
	return side
}
