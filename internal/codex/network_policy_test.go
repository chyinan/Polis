// pattern: Functional Core
package codex

import (
	"strings"
	"testing"
	"time"
)

func TestCanonicalManifestV4IncludesNetworkNamespacePolicy(t *testing.T) {
	combination := ExecutionCombination{CodexVersion: "0.153.4", BinarySHA256: "binary", Model: "gpt-5.6-luna", Effort: "medium", RuntimeProfile: "wsl-linux-amd64", SandboxClass: "read-only", ProxyConfigDigest: "proxy", AuthSourceClass: "mounted_codex_auth_file", CodeModeHostSHA256: "host", CapabilityDigest: "cap", NativeProtocolDigest: "native"}
	auth := validTransportPolicyAuth()
	transport := TransportPolicyManifest{ProviderTransportPolicy: ProviderTransportPolicyNativeDefault, WebSocketPolicy: ProviderTransportPolicyNativeDefault}
	shared := combination.CanonicalManifestV4Digest(auth, transport, NetworkNamespacePolicySharedHostNetwork)
	isolated := combination.CanonicalManifestV4Digest(auth, transport, NetworkNamespacePolicyIsolatedNetworkNamespace)
	if shared == isolated {
		t.Fatal("network namespace policy did not affect canonical-manifest-v4")
	}
	manifest := CanonicalManifestV4{FingerprintSchemaVersion: CanonicalManifestFingerprintSchemaVersionV4, Combination: combination, Auth: auth, Transport: transport, NetworkNamespacePolicy: NetworkNamespacePolicySharedHostNetwork}
	if err := manifest.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestEvaluateBusinessGateV4StalesOnNetworkNamespacePolicyChange(t *testing.T) {
	combination := ExecutionCombination{CodexVersion: "0.153.4", BinarySHA256: "binary", Model: "gpt-5.6-luna", Effort: "medium", RuntimeProfile: "wsl-linux-amd64", SandboxClass: "read-only", ProxyConfigDigest: "proxy", AuthSourceClass: "mounted_codex_auth_file", CodeModeHostSHA256: "host", CapabilityDigest: "cap", NativeProtocolDigest: "native"}
	auth := validTransportPolicyAuth()
	transport := TransportPolicyManifest{ProviderTransportPolicy: ProviderTransportPolicyNativeDefault, WebSocketPolicy: ProviderTransportPolicyNativeDefault}
	record := QualificationRecordV4{Layer: QualificationL1BaseTransport, Status: QualificationQualified, SchedulingDecision: "qualified", EvidenceResult: EvidencePassed, FingerprintSchemaVersion: CanonicalManifestFingerprintSchemaVersionV4, CanonicalManifestDigest: combination.CanonicalManifestV4Digest(auth, transport, NetworkNamespacePolicySharedHostNetwork), Combination: combination, Auth: auth, Transport: transport, NetworkNamespacePolicy: NetworkNamespacePolicySharedHostNetwork, CreatedAt: time.Unix(1, 0)}
	decision := EvaluateBusinessGateV4(time.Unix(2, 0), combination, auth, transport, NetworkNamespacePolicyIsolatedNetworkNamespace, []QualificationRecordV4{record})
	if decision.Status != QualificationStale || decision.Reason != NetworkNamespacePolicyChangedReasonCode {
		t.Fatalf("network policy decision = %+v", decision)
	}
}

func TestCanonicalManifestV4RejectsUnknownNetworkNamespacePolicy(t *testing.T) {
	combination := ExecutionCombination{CodexVersion: "0.153.4", BinarySHA256: strings.Repeat("a", 64), Model: "gpt-5.6-luna", Effort: "medium", RuntimeProfile: "wsl-linux-amd64", SandboxClass: "read-only", ProxyConfigDigest: strings.Repeat("b", 64), AuthSourceClass: "mounted_codex_auth_file", CodeModeHostSHA256: strings.Repeat("c", 64), CapabilityDigest: strings.Repeat("d", 64), NativeProtocolDigest: strings.Repeat("e", 64)}
	manifest := CanonicalManifestV4{FingerprintSchemaVersion: CanonicalManifestFingerprintSchemaVersionV4, Combination: combination, Auth: validTransportPolicyAuth(), Transport: TransportPolicyManifest{ProviderTransportPolicy: ProviderTransportPolicyNativeDefault, WebSocketPolicy: ProviderTransportPolicyNativeDefault}, NetworkNamespacePolicy: "unknown"}
	if err := manifest.Validate(); err == nil {
		t.Fatal("unknown network namespace policy was accepted")
	}
}
