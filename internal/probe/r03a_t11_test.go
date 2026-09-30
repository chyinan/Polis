// pattern: Functional Core
package probe

import (
	"reflect"
	"testing"

	"polis/internal/codex"
)

func TestNormalizeT11NativeArgsRemovesOnlyVersionSpecificPaths(t *testing.T) {
	old := []string{"bwrap", "--bind", "/old-home", "/home/codex", "--ro-bind", "/old-binary", "/codex", "--ro-bind", "/old-helper", "/codex-code-mode-host", "/codex", "app-server", "--stdio"}
	next := []string{"bwrap", "--bind", "/new-home", "/home/codex", "--ro-bind", "/new-binary", "/codex", "--ro-bind", "/new-helper", "/codex-code-mode-host", "/codex", "app-server", "--stdio"}

	oldNormalized := normalizeT11NativeArgs(old, "/old-home", "/old-binary", "/old-helper")
	nextNormalized := normalizeT11NativeArgs(next, "/new-home", "/new-binary", "/new-helper")
	if !reflect.DeepEqual(oldNormalized, nextNormalized) {
		t.Fatalf("normalized args differ: old=%v next=%v", oldNormalized, nextNormalized)
	}
}

func TestSummarizeT11ProtocolCapturesTransportMilestones(t *testing.T) {
	raw := []byte(
		`{"time":"2026-09-10T00:00:00Z","direction":"send","data":{"method":"initialize"}}` + "\n" +
			`{"time":"2026-09-10T00:00:00.010Z","direction":"receive","data":{"id":1,"result":{"userAgent":"polis/0.153.4","codexHome":"/home/codex"}}}` + "\n" + `{"time":"2026-09-10T00:00:00.011Z","direction":"send","data":{"method":"initialized"}}` + "\n" + `{"time":"2026-09-10T00:00:00.020Z","direction":"send","data":{"method":"thread/start"}}` + "\n" + `{"time":"2026-09-10T00:00:00.030Z","direction":"receive","data":{"id":2,"result":{"thread":{"id":"thread-1"}}}}` + "\n" + `{"time":"2026-09-10T00:00:00.031Z","direction":"receive","data":{"method":"thread/started","params":{"thread":{"id":"thread-1"}}}}` + "\n" + `{"time":"2026-09-10T00:00:00.040Z","direction":"send","data":{"method":"turn/start"}}` + "\n" + `{"time":"2026-09-10T00:00:00.050Z","direction":"receive","data":{"method":"turn/started","params":{"turn":{"id":"turn-1"}}}}` + "\n" + `{"time":"2026-09-10T00:00:00.060Z","direction":"receive","data":{"method":"item/started","params":{"item":{"type":"userMessage"}}}}` + "\n" + `{"time":"2026-09-10T00:00:00.070Z","direction":"receive","data":{"method":"error","params":{"willRetry":true,"error":{"codexErrorInfo":{"responseStreamDisconnected":{}}},"threadId":"thread-1","turnId":"turn-1"}}}` + "\n" + `{"time":"2026-09-10T00:00:00.080Z","direction":"receive","data":{"method":"item/agentMessage/delta","params":{"delta":"POLIS_TRANSPORT_CANARY_OK"}}}` + "\n" + `{"time":"2026-09-10T00:00:00.090Z","direction":"receive","data":{"method":"thread/tokenUsage/updated","params":{"tokenUsage":{"total":{"totalTokens":12}}}}}` + "\n" + `{"time":"2026-09-10T00:00:00.100Z","direction":"receive","data":{"method":"turn/completed","params":{"turn":{"status":"completed"}}}}` + "\n",
	)

	trace, err := SummarizeT11Protocol(raw, "POLIS_TRANSPORT_CANARY_OK")
	if err != nil {
		t.Fatal(err)
	}
	if !trace.InitializeSent || !trace.InitializeReceived || !trace.ThreadStartSent || !trace.ThreadStarted || !trace.TurnStartSent || !trace.TurnStarted || !trace.UserMessageStarted || !trace.FirstValidOutput || !trace.TurnCompleted {
		t.Fatalf("missing lifecycle milestone: %+v", trace)
	}
	if trace.ReconnectCount != 1 || len(trace.RecoveryTimestamps) != 1 || trace.FirstDisconnectDeltaMS != 20 || trace.RecoveryDeltaMS != 10 || trace.AssistantOutput != "POLIS_TRANSPORT_CANARY_OK" || trace.SentinelMatch != "passed" {
		t.Fatalf("incorrect output/reconnect summary: %+v", trace)
	}
}

func TestSummarizeT11ProtocolIgnoresStderrEvidenceRecords(t *testing.T) {
	raw := []byte(
		`{"time":"2026-09-10T00:00:00Z","direction":"receive","data":{"method":"turn/started","params":{}}}` + "\n" +
			`{"time":"2026-09-10T00:00:01Z","direction":"stderr","data":"diagnostic text"}` + "\n",
	)
	trace, err := SummarizeT11Protocol(raw, "POLIS_TRANSPORT_CANARY_OK")
	if err != nil {
		t.Fatalf("stderr evidence should be ignored: %v", err)
	}
	if !trace.TurnStarted {
		t.Fatal("turn/started was lost while ignoring stderr")
	}
}

func TestCompareT11FactorsAcceptsOnlyVersionAndDerivedChanges(t *testing.T) {
	old := t11TestFactors()
	next := old
	next.Combination.CodexVersion = "0.153.4"
	next.Combination.BinarySHA256 = "new-binary"
	next.Combination.CodeModeHostSHA256 = "new-code-mode-host"
	next.Combination.CapabilityDigest = "new-capability"
	next.Combination.NativeProtocolDigest = "new-native-protocol"

	proof, err := CompareT11Factors(old, next)
	if err != nil {
		t.Fatalf("version-only preflight rejected: %v", err)
	}
	if !proof.Passed {
		t.Fatalf("version-only preflight did not pass: %+v", proof)
	}
	if proof.OldT9Fingerprint != old.Combination.Fingerprint() {
		t.Fatalf("old T9 fingerprint = %q, want %q", proof.OldT9Fingerprint, old.Combination.Fingerprint())
	}
	if proof.NewFingerprint.FingerprintSchemaVersion != codex.CanonicalManifestFingerprintSchemaVersion {
		t.Fatalf("new fingerprint schema = %q", proof.NewFingerprint.FingerprintSchemaVersion)
	}
	if len(proof.ConfirmedDivergence) != 5 {
		t.Fatalf("confirmed divergence = %v, want version plus four derived fields", proof.ConfirmedDivergence)
	}
}

func TestCompareT11FactorsRejectsNonVersionDivergence(t *testing.T) {
	old := t11TestFactors()
	next := old
	next.Combination.CodexVersion = "0.153.4"
	next.Combination.BinarySHA256 = "new-binary"
	next.Combination.CodeModeHostSHA256 = "new-code-mode-host"
	next.Combination.CapabilityDigest = "new-capability"
	next.Combination.NativeProtocolDigest = "new-native-protocol"
	next.Combination.Model = "different-model"
	next.ProxyEnvironment = map[string]string{}
	for key, value := range old.ProxyEnvironment {
		next.ProxyEnvironment[key] = value
	}
	next.ProxyEnvironment["HTTP_PROXY"] = "unexpected"

	proof, err := CompareT11Factors(old, next)
	if err == nil {
		t.Fatalf("expected preflight_failed, proof=%+v", proof)
	}
	if proof.Passed {
		t.Fatalf("non-version divergence passed: %+v", proof)
	}
}

func TestCompareT11FactorsSeparatesCredentialRevisionFromIdentity(t *testing.T) {
	old := t11TestFactors()
	old.AuthManifest = codex.AuthFingerprintManifest{
		AuthSourceClass: "mounted_codex_auth_file", AuthIdentityFingerprintSchemaVersion: codex.AuthIdentityFingerprintSchemaVersion, AuthIdentityFingerprint: "unavailable", AuthIdentityFingerprintStatus: "unavailable", AuthCredentialRevisionFingerprintSchemaVersion: codex.AuthCredentialRevisionFingerprintSchemaVersion, AuthCredentialRevisionFingerprint: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", AuthCredentialRevisionFingerprintStatus: "available",
	}
	next := old
	next.AuthManifest.AuthCredentialRevisionFingerprint = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	proof, err := CompareT11Factors(old, next)
	if err == nil {
		t.Fatalf("expected credential revision divergence, proof=%+v", proof)
	}
	if proof.AuthComparison.Identity != codex.AuthComparisonNotReconstructable || proof.AuthComparison.CredentialRevision != codex.AuthComparisonDifferent || proof.AuthComparison.ReasonCode != codex.AuthCredentialRevisionChangedReasonCode {
		t.Fatalf("auth comparison = %+v", proof.AuthComparison)
	}
}

func t11TestFactors() T11CanaryFactors {
	return T11CanaryFactors{
		Combination: codex.ExecutionCombination{
			CodexVersion:         "0.151.0",
			BinarySHA256:         "old-binary",
			Model:                "gpt-5.6-luna",
			Effort:               "medium",
			RuntimeProfile:       "wsl-linux-amd64",
			SandboxClass:         "read-only",
			ProxyConfigDigest:    "proxy-none",
			AuthSourceClass:      "local_codex_auth_file",
			CodeModeHostSHA256:   "old-code-mode-host",
			CapabilityDigest:     "old-capability",
			NativeProtocolDigest: "old-native-protocol",
		},
		AuthManifest: codex.AuthFingerprintManifest{
			AuthSourceClass: "mounted_codex_auth_file", AuthIdentityFingerprintSchemaVersion: codex.AuthIdentityFingerprintSchemaVersion, AuthIdentityFingerprint: "1111111111111111111111111111111111111111111111111111111111111111", AuthIdentityFingerprintStatus: "available", AuthCredentialRevisionFingerprintSchemaVersion: codex.AuthCredentialRevisionFingerprintSchemaVersion, AuthCredentialRevisionFingerprint: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", AuthCredentialRevisionFingerprintStatus: "available",
		},
		ProxyEnvironment: map[string]string{
			"HTTP_PROXY":         "absent",
			"HTTPS_PROXY":        "absent",
			"ALL_PROXY":          "absent",
			"NO_PROXY":           "absent",
			"POLIS_NATIVE_PROXY": "absent",
		},
		CWD:                        "/work",
		Home:                       "/home/codex",
		Invocation:                 "app-server --stdio",
		Sandbox:                    "read-only",
		WebSocketPolicy:            "disabled",
		DynamicToolCount:           0,
		ToolSchemaBytes:            0,
		DeveloperInstructionDigest: "same-developer",
		PromptDigest:               "same-prompt",
	}
}
