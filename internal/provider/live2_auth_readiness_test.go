// pattern: Imperative Shell
package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"polis/internal/codex"
)

func TestCodexRuntimeReadinessRequiresStructuredChatGPTCredentials(t *testing.T) {
	for name, raw := range map[string][]byte{
		"invalid json":     []byte("not-json"),
		"missing refresh":  live2AuthFile(t, "access-a", ""),
		"missing access":   live2AuthFile(t, "", "refresh-a"),
		"missing identity": []byte(`{"auth_mode":"chatgpt","tokens":{"access_token":"access-a","refresh_token":"refresh-a"}}`),
	} {
		t.Run(name, func(t *testing.T) {
			_, authPath, _ := live2CodexRuntimeWithAuth(t, raw)
			if _, err := readLive2AuthSource(authPath); err == nil {
				t.Fatal("LIVE_2 runtime accepted an unready credential source")
			}
		})
	}
	_, authPath, _ := live2CodexRuntimeWithAuth(t, live2AuthFile(t, "access-a", "refresh-a"))
	snapshot, err := readLive2AuthSource(authPath)
	if err != nil || snapshot.identityFingerprint == "" || snapshot.credentialRevisionFingerprint == "" {
		t.Fatalf("valid ChatGPT credentials did not yield an opaque identity/revision snapshot: snapshot=%+v error=%v", snapshot, err)
	}
}

func TestCodexRuntimeReserveRejectsAuthRevisionDriftBeforeAllowance(t *testing.T) {
	runtime, authPath, allowancePath := live2CodexRuntimeWithAuth(t, live2AuthFile(t, "access-a", "refresh-a"))
	runtime.config.Purpose = Live2AuthorizationPurpose
	if err := runtime.captureLive2AuthSource(); err != nil {
		t.Fatalf("valid local auth source rejected by LIVE_2 parser: %v", err)
	}
	if err := os.WriteFile(authPath, live2AuthFile(t, "access-b", "refresh-a"), 0600); err != nil {
		t.Fatal(err)
	}
	authorization := live2AuthorizationFixture()
	authorization.ProviderMode = "real"
	authorization.Purpose = runtime.ExecutionProfile().Purpose
	if _, err := runtime.Reserve(context.Background(), authorization); err == nil {
		t.Fatal("Codex runtime reserved after the credential revision changed since readiness")
	}
	if _, err := os.Stat(allowancePath); !os.IsNotExist(err) {
		t.Fatalf("credential drift created an allowance file: stat error = %v", err)
	}
}

func TestCodexRuntimeReadinessRejectsLIVE2BinaryAndHelperDrift(t *testing.T) {
	runtime, _, _ := live2CodexRuntimeWithAuth(t, live2AuthFile(t, "access-a", "refresh-a"))
	config := runtime.config
	config.Purpose = Live2AuthorizationPurpose
	config.ExpectedVersion = ProductProviderRuntimeVersionV2
	config.BinarySHA256 = ProductProviderBinarySHA256V2
	config.HelperSHA256 = ProductProviderHelperSHA256V2
	config.HelperBinary = filepath.Join(filepath.Dir(config.Binary), "codex-code-mode-host.exe")
	if err := os.WriteFile(config.HelperBinary, []byte("helper placeholder"), 0600); err != nil {
		t.Fatal(err)
	}

	for name, mutate := range map[string]func(*CodexRuntimeConfig){
		"wrong expected version": func(c *CodexRuntimeConfig) { c.ExpectedVersion = "stale-version" },
		"binary hash drift": func(c *CodexRuntimeConfig) {
			c.BinarySHA256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		},
		"helper hash drift": func(c *CodexRuntimeConfig) {
			c.HelperSHA256 = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		},
		"helper path drift": func(c *CodexRuntimeConfig) {
			c.HelperBinary = filepath.Join(filepath.Dir(c.Binary), "unqualified-helper.exe")
		},
	} {
		t.Run(name, func(t *testing.T) {
			changed := config
			mutate(&changed)
			if err := NewCodexRuntime(changed).Readiness(context.Background()); err == nil {
				t.Fatal("CodexRuntime accepted a LIVE_2 runtime binary/helper drift")
			}
		})
	}
}

func TestValidateLive2RuntimePinsRequiresB4Identity(t *testing.T) {
	if err := validateLive2RuntimePins(ProductProviderRuntimeVersionV2, ProductProviderBinarySHA256V2, ProductProviderHelperSHA256V2); err != nil {
		t.Fatalf("current B4 runtime identity rejected: %v", err)
	}
	for name, pins := range map[string][3]string{
		"stale version": {"0.154.0-alpha.5.0", ProductProviderBinarySHA256V2, ProductProviderHelperSHA256V2},
		"binary drift":  {ProductProviderRuntimeVersionV2, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ProductProviderHelperSHA256V2},
		"helper drift":  {ProductProviderRuntimeVersionV2, ProductProviderBinarySHA256V2, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateLive2RuntimePins(pins[0], pins[1], pins[2]); err == nil {
				t.Fatal("stale LIVE_2 runtime identity pins were accepted")
			}
		})
	}
}

func TestCodexRuntimeReserveRejectsBinaryHashDriftBeforeAllowance(t *testing.T) {
	runtime, _, allowancePath := live2CodexRuntimeWithAuth(t, live2AuthFile(t, "access-a", "refresh-a"))
	runtime.config.Purpose = Live2AuthorizationPurpose
	if err := runtime.captureLive2AuthSource(); err != nil {
		t.Fatal(err)
	}
	runtime.config.ExpectedVersion = ProductProviderRuntimeVersionV2
	runtime.config.BinarySHA256 = ProductProviderBinarySHA256V2
	runtime.config.HelperSHA256 = ProductProviderHelperSHA256V2
	runtime.config.HelperBinary = filepath.Join(filepath.Dir(runtime.config.Binary), "codex-code-mode-host.exe")
	if err := os.WriteFile(runtime.config.HelperBinary, []byte("different helper"), 0600); err != nil {
		t.Fatal(err)
	}
	authorization := live2AuthorizationFixture()
	authorization.ProviderMode = "real"
	authorization.Purpose = Live2AuthorizationPurpose
	if _, err := runtime.Reserve(context.Background(), authorization); err == nil {
		t.Fatal("Codex runtime reserved after its provider binary/helper differed from the B4 identity")
	}
	if _, err := os.Stat(allowancePath); !os.IsNotExist(err) {
		t.Fatalf("runtime artifact drift created an allowance file: stat error = %v", err)
	}
}

func live2CodexRuntimeWithAuth(t *testing.T, auth []byte) (*CodexRuntime, string, string) {
	t.Helper()
	root := t.TempDir()
	binary := filepath.Join(root, "codex.exe")
	authPath := filepath.Join(root, "auth.json")
	evidenceRoot := filepath.Join(root, "evidence")
	if err := os.Mkdir(evidenceRoot, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary, []byte("test binary placeholder"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(authPath, auth, 0600); err != nil {
		t.Fatal(err)
	}
	allowancePath := filepath.Join(root, "allowance.json")
	runtime := NewCodexRuntime(CodexRuntimeConfig{
		Binary: binary, AuthFile: authPath, Root: filepath.Join(root, "provider-root"), EvidenceRoot: evidenceRoot,
		Model: "gpt-5.6-luna", Effort: "medium", ExpectedVersion: "0.154.0-alpha.6.2",
		TransportPolicy: codex.DefaultTransportPolicy(), ToolSurface: ProductToolSurface(),
		Purpose: "product-artifact", ExactSurfaceExecutionFingerprint: ProductExactSurfaceExecutionFingerprint,
		ProductProviderL2Fingerprint: ProductProviderL2Fingerprint, ExecutionEnvelope: ProductProviderRuntimeEnvelopeFingerprintV2,
		ToolSurfaceQualification: ProductToolSurfaceQualification, AllowancePath: allowancePath,
		MediumLimit: 1, HighLimit: 0, ToolCallLimit: 16,
	})
	return runtime, authPath, allowancePath
}

func live2AuthFile(t *testing.T, accessToken, refreshToken string) []byte {
	t.Helper()
	header, err := json.Marshal(map[string]string{"alg": "none"})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := json.Marshal(map[string]string{"iss": "https://auth.openai.com", "sub": "fixture-user", "auth_provider": "chatgpt"})
	if err != nil {
		t.Fatal(err)
	}
	idToken := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims) + ".fixture-signature"
	raw, err := json.Marshal(map[string]any{
		"auth_mode": "chatgpt",
		"tokens":    map[string]string{"id_token": idToken, "access_token": accessToken, "refresh_token": refreshToken},
	})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
