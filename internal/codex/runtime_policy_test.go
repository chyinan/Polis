// pattern: Functional Core
package codex

import (
	"strings"
	"testing"
)

func TestCanonicalManifestV6IncludesRuntimeEnvelope(t *testing.T) {
	combination := ExecutionCombination{CodexVersion: "0.153.4", BinarySHA256: "binary", Model: "gpt-5.6-luna", Effort: "medium", RuntimeProfile: "wsl-linux-amd64", SandboxClass: "read-only", ProxyConfigDigest: "proxy", AuthSourceClass: "mounted_codex_auth_file", CodeModeHostSHA256: "host", CapabilityDigest: "cap", NativeProtocolDigest: "native"}
	auth := validTransportPolicyAuth()
	transport := TransportPolicyManifest{ProviderTransportPolicy: ProviderTransportPolicyNativeDefault, WebSocketPolicy: ProviderTransportPolicyNativeDefault}
	wsl := RuntimeExecutionEnvelope{Platform: "linux", Architecture: "amd64", RuntimeProfile: "wsl-linux-bwrap", FilesystemIsolation: "bwrap_read_only_guest", ProcessIsolation: "bwrap_process_group", NetworkPolicy: RuntimeNetworkPolicySharedHostNetwork, CWDRole: "diagnostic_workspace", LaunchMechanism: "bwrap_exec", StdioMode: "pipes"}
	windows := wsl
	windows.Platform, windows.RuntimeProfile, windows.NetworkPolicy, windows.FilesystemIsolation, windows.ProcessIsolation, windows.LaunchMechanism = "windows", "windows-native", RuntimeNetworkPolicyNativeOSNetwork, "diagnostic_home_only", "windows_process_handle", "CreateProcess"
	if combination.CanonicalManifestV6Digest(auth, transport, wsl, CodexHomeProfilePolisIsolated, strings.Repeat("a", 64), strings.Repeat("b", 64)) == combination.CanonicalManifestV6Digest(auth, transport, windows, CodexHomeProfileWindowsWorking, strings.Repeat("a", 64), strings.Repeat("b", 64)) {
		t.Fatal("runtime envelope did not affect canonical-manifest-v6")
	}
	manifest := CanonicalManifestV6{FingerprintSchemaVersion: CanonicalManifestFingerprintSchemaVersionV6, Combination: combination, Auth: auth, Transport: transport, Runtime: wsl, CodexHomeProfile: CodexHomeProfilePolisIsolated, EffectiveConfigDigest: strings.Repeat("a", 64), EffectiveTransportConfigDigest: strings.Repeat("b", 64)}
	if err := manifest.Validate(); err != nil {
		t.Fatal(err)
	}
}
