// pattern: Functional Core
package codex

import (
	"strings"
	"testing"
)

func TestCanonicalManifestV7BindsExactToolSurface(t *testing.T) {
	base := CanonicalManifestV6{
		FingerprintSchemaVersion: CanonicalManifestFingerprintSchemaVersionV6,
		Combination:              ExecutionCombination{CodexVersion: "0.153.4", BinarySHA256: strings.Repeat("a", 64), Model: "gpt-5.6-luna", Effort: "medium", RuntimeProfile: "windows-native", SandboxClass: "read-only", ProxyConfigDigest: strings.Repeat("b", 64), AuthSourceClass: "controlled_diagnostic_auth_material", CodeModeHostSHA256: strings.Repeat("c", 64), CapabilityDigest: strings.Repeat("d", 64), NativeProtocolDigest: strings.Repeat("e", 64)},
		Auth:                     validTransportPolicyAuth(),
		Transport:                TransportPolicyManifest{ProviderTransportPolicy: ProviderTransportPolicyNativeDefault, WebSocketPolicy: ProviderTransportPolicyNativeDefault},
		Runtime:                  RuntimeExecutionEnvelope{Platform: "windows", Architecture: "amd64", RuntimeProfile: "windows-native", FilesystemIsolation: "diagnostic_home_only", ProcessIsolation: "windows_process_handle", NetworkPolicy: RuntimeNetworkPolicyNativeOSNetwork, CWDRole: "diagnostic_workspace", LaunchMechanism: "CreateProcess", StdioMode: "redirected_standard_pipes"},
		CodexHomeProfile:         CodexHomeProfileWindowsDiagnostic, EffectiveConfigDigest: strings.Repeat("f", 64), EffectiveTransportConfigDigest: strings.Repeat("1", 64),
	}
	surface := ToolSurfaceManifest{SurfaceID: "peer_backend", ToolCount: 1, AggregateManifestDigest: strings.Repeat("2", 64), AggregateSchemaDigest: strings.Repeat("3", 64), AggregateSchemaBytes: 10, ThreadStartPayloadDigest: strings.Repeat("5", 64), ThreadStartPayloadBytes: 100, Tools: []ToolSurfaceEntry{{Name: "polis_work_current", SchemaDigest: strings.Repeat("4", 64), SchemaBytes: 10, BindingIdentity: "PeerEmployeeTools.call->Handover", AuthorizationClass: "peer_backend", RegistrationOrdinal: 1}}, BusinessWritePolicy: "diagnostic_denied"}
	if err := surface.Validate(); err != nil {
		t.Fatal(err)
	}
	manifest := CanonicalManifestV7{FingerprintSchemaVersion: CanonicalManifestFingerprintSchemaVersionV7, Base: base, ToolSurface: surface}
	if err := manifest.Validate(); err != nil {
		t.Fatal(err)
	}
	fingerprint := base.Combination.CurrentFingerprintV7(manifest)
	if fingerprint.FingerprintSchemaVersion != CanonicalManifestFingerprintSchemaVersionV7 || fingerprint.CanonicalManifestDigest == "" {
		t.Fatalf("invalid V7 fingerprint: %+v", fingerprint)
	}
	if fingerprint.CanonicalManifestDigest == base.Combination.CurrentFingerprintV6(base.Auth, base.Transport, base.Runtime, base.CodexHomeProfile, base.EffectiveConfigDigest, base.EffectiveTransportConfigDigest).CanonicalManifestDigest {
		t.Fatal("tool surface did not affect fingerprint")
	}
}

func TestToolSurfaceRejectsDuplicateRegistrationOrdinal(t *testing.T) {
	surface := ToolSurfaceManifest{SurfaceID: "peer_backend", ToolCount: 2, AggregateManifestDigest: strings.Repeat("a", 64), AggregateSchemaDigest: strings.Repeat("b", 64), AggregateSchemaBytes: 20, ThreadStartPayloadDigest: strings.Repeat("e", 64), ThreadStartPayloadBytes: 100, Tools: []ToolSurfaceEntry{{Name: "polis_a", SchemaDigest: strings.Repeat("c", 64), SchemaBytes: 10, BindingIdentity: "binding", AuthorizationClass: "peer_backend", RegistrationOrdinal: 1}, {Name: "polis_b", SchemaDigest: strings.Repeat("d", 64), SchemaBytes: 10, BindingIdentity: "binding", AuthorizationClass: "peer_backend", RegistrationOrdinal: 1}}, BusinessWritePolicy: "diagnostic_denied"}
	if err := surface.Validate(); err == nil {
		t.Fatal("duplicate registration ordinal was accepted")
	}
}

func TestPeerFrontendInteractionSchemasUseReferencesOnly(t *testing.T) {
	var apply, checkpoint map[string]any
	for _, raw := range PeerFrontendTools() {
		tool := raw.(map[string]any)
		switch tool["name"] {
		case "polis_collab_apply":
			apply = tool["inputSchema"].(map[string]any)
		case "polis_work_checkpoint":
			checkpoint = tool["inputSchema"].(map[string]any)
		}
	}
	applyProperties := apply["properties"].(map[string]any)
	if _, present := applyProperties["content"]; present {
		t.Fatal("collab_apply schema still exposes candidate content")
	}
	for _, name := range []string{"obligation_id", "contract_revision_id", "workspace_revision", "evidence_refs"} {
		if _, present := applyProperties[name]; !present {
			t.Fatalf("collab_apply schema omitted %s", name)
		}
	}
	checkpointProperties := checkpoint["properties"].(map[string]any)
	if _, present := checkpointProperties["evidence"]; present {
		t.Fatal("checkpoint schema still exposes free-text evidence")
	}
	if _, present := checkpointProperties["evidence_refs"]; !present {
		t.Fatal("checkpoint schema omitted evidence_refs")
	}
}

func TestPeerFrontendCollabApplyDescriptionPublishesEvidenceTypes(t *testing.T) {
	for _, raw := range PeerFrontendTools() {
		tool := raw.(map[string]any)
		if tool["name"] != "polis_collab_apply" {
			continue
		}
		description := tool["description"].(string)
		for _, phrase := range []string{"workspace.replace", "workspace.check", "successful collab.apply", "progress-checkpoint receipts"} {
			if !strings.Contains(description, phrase) {
				t.Fatalf("collab_apply description omitted public evidence rule %q: %s", phrase, description)
			}
		}
		return
	}
	t.Fatal("collab_apply tool was not registered")
}

func TestPeerBackendCollabSendUsesExplicitEmployeeAndTaskTargets(t *testing.T) {
	for _, raw := range PeerBackendTools() {
		tool := raw.(map[string]any)
		if tool["name"] != "polis_collab_send" {
			continue
		}
		schema := tool["inputSchema"].(map[string]any)
		properties := schema["properties"].(map[string]any)
		for _, name := range []string{"to_employee_id", "to_task_id", "contract_revision_id", "body", "actionable"} {
			if _, ok := properties[name]; !ok {
				t.Fatalf("collab.send schema omitted explicit target field %s", name)
			}
		}
		if _, ok := properties["to_task"]; ok {
			t.Fatal("collab.send schema still exposes ambiguous to_task")
		}
		return
	}
	t.Fatal("collab.send tool was not registered")
}
