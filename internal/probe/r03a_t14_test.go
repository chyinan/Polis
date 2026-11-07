// pattern: Functional Core
package probe

import (
	"strings"
	"testing"

	"polis/internal/codex"
)

func TestCompareT14FactorsAllowsOnlyTransportPolicyAndDerivedCapability(t *testing.T) {
	old := t14TestFactors()
	next := old
	next.Transport = codex.TransportPolicyManifest{ProviderTransportPolicy: codex.ProviderTransportPolicyNativeDefault, WebSocketPolicy: codex.ProviderTransportPolicyNativeDefault}
	next.Combination.CapabilityDigest = strings.Repeat("b", 64)

	proof, err := CompareT14Factors(old, next)
	if err != nil || !proof.Passed {
		t.Fatalf("transport-only factor diff rejected: proof=%+v err=%v", proof, err)
	}
	if len(proof.ConfirmedDivergence) != 3 {
		t.Fatalf("confirmed divergence = %v", proof.ConfirmedDivergence)
	}
	if len(proof.NonTransportDivergence) != 0 {
		t.Fatalf("unexpected non-transport divergence = %v", proof.NonTransportDivergence)
	}
}

func TestCompareT14FactorsRejectsVersionDivergence(t *testing.T) {
	old := t14TestFactors()
	next := old
	next.Combination.CodexVersion = "0.151.0"

	proof, err := CompareT14Factors(old, next)
	if err == nil || proof.Passed {
		t.Fatalf("version divergence was accepted: proof=%+v err=%v", proof, err)
	}
}

func t14TestFactors() T14CanaryFactors {
	return T14CanaryFactors{
		Combination:      codex.ExecutionCombination{CodexVersion: "0.153.4", BinarySHA256: strings.Repeat("a", 64), Model: "gpt-5.6-luna", Effort: "medium", RuntimeProfile: "wsl-linux-amd64", SandboxClass: "read-only", ProxyConfigDigest: "proxy", AuthSourceClass: "mounted_codex_auth_file", CodeModeHostSHA256: strings.Repeat("c", 64), CapabilityDigest: strings.Repeat("a", 64), NativeProtocolDigest: strings.Repeat("d", 64)},
		AuthManifest:     codex.AuthFingerprintManifest{AuthSourceClass: "mounted_codex_auth_file", AuthIdentityFingerprintSchemaVersion: codex.AuthIdentityFingerprintSchemaVersion, AuthIdentityFingerprint: strings.Repeat("e", 64), AuthIdentityFingerprintStatus: "available", AuthCredentialRevisionFingerprintSchemaVersion: codex.AuthCredentialRevisionFingerprintSchemaVersion, AuthCredentialRevisionFingerprint: strings.Repeat("f", 64), AuthCredentialRevisionFingerprintStatus: "available"},
		ProxyEnvironment: map[string]string{"HTTP_PROXY": "absent", "HTTPS_PROXY": "absent", "ALL_PROXY": "absent", "NO_PROXY": "absent", "POLIS_NATIVE_PROXY": "absent"},
		CWD:              "/work", Home: "/home/codex", Invocation: "app-server --stdio", Sandbox: "read-only", WebSocketPolicy: "disabled", DynamicToolCount: 0, ToolSchemaBytes: 0, DeveloperInstructionDigest: "prompt", PromptDigest: "prompt", Transport: codex.TransportPolicyManifest{ProviderTransportPolicy: codex.ProviderTransportPolicyExplicitlyDisabled, WebSocketPolicy: codex.ProviderTransportPolicyExplicitlyDisabled},
	}
}
