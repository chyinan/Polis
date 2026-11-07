// pattern: Functional Core
package probe

import (
	"reflect"
	"strings"
	"sync"
	"testing"

	"polis/internal/codex"
)

func TestDeriveT14AArtifactsBindsPolicyCapabilityManifestFingerprintAndLaunch(t *testing.T) {
	disabled := t14ATestConfig(codex.ProviderTransportPolicyExplicitlyDisabled)
	nativeDefault := t14ATestConfig(codex.ProviderTransportPolicyNativeDefault)
	disabledArtifacts, err := DeriveT14AArtifacts(disabled)
	if err != nil {
		t.Fatal(err)
	}
	nativeDefaultArtifacts, err := DeriveT14AArtifacts(nativeDefault)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateT14ABinding(disabled, disabledArtifacts); err != nil {
		t.Fatalf("disabled binding rejected: %v", err)
	}
	if err := ValidateT14ABinding(nativeDefault, nativeDefaultArtifacts); err != nil {
		t.Fatalf("native-default binding rejected: %v", err)
	}
	if disabledArtifacts.CapabilityDigest == nativeDefaultArtifacts.CapabilityDigest || disabledArtifacts.ManifestDigest == nativeDefaultArtifacts.ManifestDigest || disabledArtifacts.Fingerprint.CanonicalManifestDigest == nativeDefaultArtifacts.Fingerprint.CanonicalManifestDigest || disabledArtifacts.LaunchConfigDigest == nativeDefaultArtifacts.LaunchConfigDigest {
		t.Fatal("transport policy did not change all policy-derived artifacts")
	}
	if disabledArtifacts.ConfigDigest == nativeDefaultArtifacts.ConfigDigest {
		t.Fatal("effective execution config digest ignored policy")
	}
	if disabledArtifacts.Manifest.Transport.WebSocketPolicy != codex.ProviderTransportPolicyExplicitlyDisabled || nativeDefaultArtifacts.Manifest.Transport.WebSocketPolicy != codex.ProviderTransportPolicyNativeDefault {
		t.Fatalf("manifest policy mismatch: disabled=%+v default=%+v", disabledArtifacts.Manifest.Transport, nativeDefaultArtifacts.Manifest.Transport)
	}
}

func TestValidateT14ABindingRejectsStaleOrTamperedDerivedFields(t *testing.T) {
	config := t14ATestConfig(codex.ProviderTransportPolicyNativeDefault)
	artifacts, err := DeriveT14AArtifacts(config)
	if err != nil {
		t.Fatal(err)
	}
	mutations := map[string]func(*T14ADerivedArtifacts){
		"capability": func(a *T14ADerivedArtifacts) { a.CapabilityDigest = strings.Repeat("0", 64) },
		"manifest policy": func(a *T14ADerivedArtifacts) {
			a.Manifest.Transport.WebSocketPolicy = codex.ProviderTransportPolicyExplicitlyDisabled
		},
		"launch args":          func(a *T14ADerivedArtifacts) { a.LaunchArgs = append(a.LaunchArgs, "unexpected") },
		"launch config":        func(a *T14ADerivedArtifacts) { a.LaunchConfig = append(a.LaunchConfig, 'x') },
		"source config digest": func(a *T14ADerivedArtifacts) { a.ConfigDigest = strings.Repeat("1", 64) },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			copy := artifacts
			copy.LaunchArgs = append([]string(nil), artifacts.LaunchArgs...)
			copy.LaunchEnvironment = append([]string(nil), artifacts.LaunchEnvironment...)
			copy.LaunchConfig = append([]byte(nil), artifacts.LaunchConfig...)
			mutate(&copy)
			if err := ValidateT14ABinding(config, copy); err == nil {
				t.Fatal("tampered binding was accepted")
			}
		})
	}
}

func TestT14AUnrelatedPromptDoesNotChangeTransportCapability(t *testing.T) {
	base := t14ATestConfig(codex.ProviderTransportPolicyNativeDefault)
	changed := base
	changed.PromptDigest = strings.Repeat("9", 64)
	baseArtifacts, err := DeriveT14AArtifacts(base)
	if err != nil {
		t.Fatal(err)
	}
	changedArtifacts, err := DeriveT14AArtifacts(changed)
	if err != nil {
		t.Fatal(err)
	}
	if baseArtifacts.CapabilityDigest != changedArtifacts.CapabilityDigest || baseArtifacts.LaunchConfigDigest != changedArtifacts.LaunchConfigDigest {
		t.Fatal("unrelated prompt field changed transport launch capability")
	}
}

func TestT14ASeparatesHostBindPathFromGuestHomePath(t *testing.T) {
	config := t14ATestConfig(codex.ProviderTransportPolicyNativeDefault)
	config.HostHomePath = "/runtime/t14a/home"
	artifacts, err := DeriveT14AArtifacts(config)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(artifacts.LaunchArgs, " ")
	if !strings.Contains(joined, "/runtime/t14a/home /home/codex") || strings.Contains(joined, "/home/codex /home/codex") {
		t.Fatalf("host/guest home paths are conflated: %s", joined)
	}
}

func TestT14ADerivationIsDeterministicAndReadAfterFreezeSafe(t *testing.T) {
	config := t14ATestConfig(codex.ProviderTransportPolicyNativeDefault)
	want, err := DeriveT14AArtifacts(config)
	if err != nil {
		t.Fatal(err)
	}
	again, err := DeriveT14AArtifacts(config)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, again) {
		t.Fatal("repeated derivation changed artifacts")
	}
	const workers = 8
	results := make([]T14ADerivedArtifacts, workers)
	errs := make([]error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			results[index], errs[index] = DeriveT14AArtifacts(config)
		}(i)
	}
	wg.Wait()
	for i := range results {
		if errs[i] != nil || !reflect.DeepEqual(results[i], want) {
			t.Fatalf("concurrent derivation %d diverged: artifact=%+v err=%v", i, results[i], errs[i])
		}
	}
}

func t14ATestConfig(policy string) T14AEffectiveExecutionConfig {
	return T14AEffectiveExecutionConfig{
		BinaryPath:                 "/candidate/codex",
		BinarySHA256:               strings.Repeat("a", 64),
		CodeModeHostPath:           "/candidate/codex-code-mode-host",
		CodeModeHostSHA256:         strings.Repeat("b", 64),
		CodexVersion:               "0.153.4",
		Model:                      "gpt-5.6-luna",
		Effort:                     "medium",
		RuntimeProfile:             "wsl-linux-amd64",
		SandboxClass:               "read-only",
		AuthFile:                   "/external/auth.json",
		HostHomePath:               "/runtime/t14a/home",
		Auth:                       codex.AuthFingerprintManifest{AuthSourceClass: "mounted_codex_auth_file", AuthIdentityFingerprintSchemaVersion: codex.AuthIdentityFingerprintSchemaVersion, AuthIdentityFingerprint: strings.Repeat("c", 64), AuthIdentityFingerprintStatus: "available", AuthCredentialRevisionFingerprintSchemaVersion: codex.AuthCredentialRevisionFingerprintSchemaVersion, AuthCredentialRevisionFingerprint: strings.Repeat("d", 64), AuthCredentialRevisionFingerprintStatus: "available"},
		ProxyURL:                   "",
		ProxyEnvironment:           map[string]string{"HTTP_PROXY": "absent", "HTTPS_PROXY": "absent", "ALL_PROXY": "absent", "NO_PROXY": "absent", "POLIS_NATIVE_PROXY": "absent"},
		WebSocketPolicy:            codex.TransportPolicyManifest{ProviderTransportPolicy: policy, WebSocketPolicy: policy},
		Home:                       "/home/codex",
		CWD:                        "/work",
		Invocation:                 "app-server --stdio",
		DynamicToolCount:           0,
		ToolSchemaBytes:            0,
		DeveloperInstructionDigest: strings.Repeat("e", 64),
		PromptDigest:               strings.Repeat("e", 64),
		FirstOutputDeadlineMS:      90000,
		StreamingIdleDeadlineMS:    90000,
		ReconnectGraceMS:           30000,
		TotalDeadlineMS:            600000,
		StopSemantics:              "process-group-kill-and-waited-proof",
		NativeProtocolDigest:       strings.Repeat("f", 64),
	}
}
