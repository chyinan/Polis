// pattern: Functional Core
package codex

import (
	"strings"
	"testing"
	"time"
)

func TestCanonicalManifestV5IncludesEffectiveConfigAndHomeProfile(t *testing.T) {
	combination := ExecutionCombination{CodexVersion: "0.153.4", BinarySHA256: "binary", Model: "gpt-5.6-luna", Effort: "medium", RuntimeProfile: "wsl-linux-amd64", SandboxClass: "read-only", ProxyConfigDigest: "proxy", AuthSourceClass: "mounted_codex_auth_file", CodeModeHostSHA256: "host", CapabilityDigest: "cap", NativeProtocolDigest: "native"}
	auth := validTransportPolicyAuth()
	transport := TransportPolicyManifest{ProviderTransportPolicy: ProviderTransportPolicyNativeDefault, WebSocketPolicy: ProviderTransportPolicyNativeDefault}
	shared := combination.CanonicalManifestV5Digest(auth, transport, NetworkNamespacePolicySharedHostNetwork, CodexHomeProfilePolisIsolated, strings.Repeat("a", 64), strings.Repeat("b", 64))
	working := combination.CanonicalManifestV5Digest(auth, transport, NetworkNamespacePolicySharedHostNetwork, CodexHomeProfileWindowsWorking, strings.Repeat("a", 64), strings.Repeat("b", 64))
	if shared == working {
		t.Fatal("home profile did not affect canonical-manifest-v5")
	}
	manifest := CanonicalManifestV5{FingerprintSchemaVersion: CanonicalManifestFingerprintSchemaVersionV5, Combination: combination, Auth: auth, Transport: transport, NetworkNamespacePolicy: NetworkNamespacePolicySharedHostNetwork, CodexHomeProfile: CodexHomeProfilePolisIsolated, EffectiveConfigDigest: strings.Repeat("a", 64), EffectiveTransportConfigDigest: strings.Repeat("b", 64)}
	if err := manifest.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestEvaluateBusinessGateV5StalesOnEffectiveConfigChange(t *testing.T) {
	combination := ExecutionCombination{CodexVersion: "0.153.4", BinarySHA256: "binary", Model: "gpt-5.6-luna", Effort: "medium", RuntimeProfile: "wsl-linux-amd64", SandboxClass: "read-only", ProxyConfigDigest: "proxy", AuthSourceClass: "mounted_codex_auth_file", CodeModeHostSHA256: "host", CapabilityDigest: "cap", NativeProtocolDigest: "native"}
	auth := validTransportPolicyAuth()
	transport := TransportPolicyManifest{ProviderTransportPolicy: ProviderTransportPolicyNativeDefault, WebSocketPolicy: ProviderTransportPolicyNativeDefault}
	record := QualificationRecordV5{Layer: QualificationL1BaseTransport, Status: QualificationQualified, SchedulingDecision: "qualified", EvidenceResult: EvidencePassed, FingerprintSchemaVersion: CanonicalManifestFingerprintSchemaVersionV5, CanonicalManifestDigest: combination.CanonicalManifestV5Digest(auth, transport, NetworkNamespacePolicySharedHostNetwork, CodexHomeProfilePolisIsolated, strings.Repeat("a", 64), strings.Repeat("b", 64)), Combination: combination, Auth: auth, Transport: transport, NetworkNamespacePolicy: NetworkNamespacePolicySharedHostNetwork, CodexHomeProfile: CodexHomeProfilePolisIsolated, EffectiveConfigDigest: strings.Repeat("a", 64), EffectiveTransportConfigDigest: strings.Repeat("b", 64), CreatedAt: time.Unix(1, 0)}
	decision := EvaluateBusinessGateV5(time.Unix(2, 0), combination, auth, transport, NetworkNamespacePolicySharedHostNetwork, CodexHomeProfilePolisIsolated, strings.Repeat("c", 64), strings.Repeat("b", 64), []QualificationRecordV5{record})
	if decision.Status != QualificationStale || decision.Reason != EffectiveConfigChangedReasonCode {
		t.Fatalf("effective config decision = %+v", decision)
	}
}
