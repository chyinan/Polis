// pattern: Functional Core
package environment

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestWindowsNodePolicyManifestIsCanonicalAndLocked(t *testing.T) {
	first, firstJSON, firstDigest, err := BuildWindowsNodeEnvironmentPolicy([]string{"registry.npmjs.org", "registry.example.invalid"}, 180_000, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	second, _, secondDigest, err := BuildWindowsNodeEnvironmentPolicy([]string{"REGISTRY.EXAMPLE.INVALID", "registry.npmjs.org"}, 180_000, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if firstDigest != secondDigest || first.ProfileID != WindowsNodeNPMProfile || first.InstallPolicy != WindowsNodeInstallPolicy || first.LifecycleScriptsPolicy != "ignore" || first.NetworkPolicy != "registry_allowlist" {
		t.Fatalf("environment policy is not canonical or locked: first=%+v second=%+v", first, second)
	}
	if strings.Contains(string(firstJSON), `"services"`) {
		t.Fatalf("legacy policy gained an empty services field: %s", firstJSON)
	}
	parsed, canonical, parsedDigest, err := ParseWindowsNodeEnvironmentPolicy(firstJSON)
	if err != nil || parsedDigest != firstDigest || string(canonical) != string(firstJSON) || len(parsed.RegistryHosts) != 2 {
		t.Fatalf("canonical environment policy roundtrip = %+v %q %s %v", parsed, canonical, parsedDigest, err)
	}
}

func TestWindowsNodePolicyManifestRejectsUnboundedOrUnapprovedFields(t *testing.T) {
	valid, _, _, err := BuildWindowsNodeEnvironmentPolicy([]string{"registry.npmjs.org"}, 180_000, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*ProjectEnvironmentPolicyManifest){
		"scripts enabled":   func(value *ProjectEnvironmentPolicyManifest) { value.LifecycleScriptsPolicy = "allow_all" },
		"unbounded timeout": func(value *ProjectEnvironmentPolicyManifest) { value.TimeoutMS = 0 },
		"unbounded output":  func(value *ProjectEnvironmentPolicyManifest) { value.OutputLimitBytes = 2 << 20 },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := valid
			candidate.RegistryHosts = append([]string(nil), valid.RegistryHosts...)
			mutate(&candidate)
			raw, marshalErr := json.Marshal(candidate)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			if _, _, _, parseErr := ParseWindowsNodeEnvironmentPolicy(raw); parseErr == nil {
				t.Fatal("unsafe environment policy was accepted")
			}
		})
	}
	withUnknownField := append([]byte(nil), []byte(`{"schemaVersion":"project-environment-policy@1","profileId":"windows-node-npm@1","registryHosts":["registry.npmjs.org"],"installPolicy":"npm ci --ignore-scripts --no-audit --no-fund","lifecycleScriptsPolicy":"ignore","networkPolicy":"registry_allowlist","timeoutMs":180000,"outputLimitBytes":1048576,"shell":"cmd.exe"}`)...)
	if _, _, _, err := ParseWindowsNodeEnvironmentPolicy(withUnknownField); err == nil {
		t.Fatal("unknown environment policy field was accepted")
	}
}

func TestEnvironmentPolicyPinsCanonicalServiceCommandsAndHealthchecks(t *testing.T) {
	base, _, baseDigest, err := BuildWindowsNodeEnvironmentPolicy([]string{"registry.npmjs.org"}, 180_000, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	serviceA := ProjectServiceDefinition{
		ID: "ui", ScriptPath: "ui/server.mjs",
		Probe: ServiceProbeSpec{BindAddress: "127.0.0.1", Port: 3400, Path: "/healthz", ExpectedStatusCode: 200, ExpectedBodySHA256: digestServiceProbeBytes([]byte("ready")), TimeoutMS: 500, LeaseDurationMS: 30_000},
	}
	serviceB := ProjectServiceDefinition{
		ID: "api", ScriptPath: "api/server.js",
		Probe: ServiceProbeSpec{BindAddress: "::1", Port: 8100, Path: "/ready", ExpectedStatusCode: 204, ExpectedBodySHA256: digestServiceProbeBytes(nil), TimeoutMS: 750, LeaseDurationMS: 60_000},
	}
	first := base
	first.Services = []ProjectServiceDefinition{serviceA, serviceB}
	canonicalFirst, firstJSON, firstDigest, err := CanonicalizeWindowsNodeEnvironmentPolicy(first)
	if err != nil {
		t.Fatal(err)
	}
	second := base
	second.Services = []ProjectServiceDefinition{serviceB, serviceA}
	canonicalSecond, _, secondDigest, err := CanonicalizeWindowsNodeEnvironmentPolicy(second)
	if err != nil || firstDigest == baseDigest || firstDigest != secondDigest || len(canonicalFirst.Services) != 2 || canonicalFirst.Services[0].ID != "api" || canonicalSecond.Services[1].ID != "ui" {
		t.Fatalf("service policy ordering/digest differs: first=%+v second=%+v digest=%s/%s err=%v", canonicalFirst.Services, canonicalSecond.Services, firstDigest, secondDigest, err)
	}
	parsed, roundtrip, digest, err := ParseWindowsNodeEnvironmentPolicy(firstJSON)
	if err != nil || digest != firstDigest || string(roundtrip) != string(firstJSON) || len(parsed.Services) != 2 {
		t.Fatalf("service policy roundtrip=(%+v,%s,%v)", parsed.Services, digest, err)
	}
}

func TestEnvironmentPolicyRejectsUnsafeServiceDefinitions(t *testing.T) {
	base, _, _, err := BuildWindowsNodeEnvironmentPolicy([]string{"registry.npmjs.org"}, 180_000, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	valid := ProjectServiceDefinition{
		ID: "api", ScriptPath: "server.js", Probe: ServiceProbeSpec{
			BindAddress: "127.0.0.1", Port: 8080, Path: "/healthz", ExpectedStatusCode: 200,
			ExpectedBodySHA256: digestServiceProbeBytes([]byte("ok")), TimeoutMS: 500, LeaseDurationMS: 30_000,
		},
	}
	for name, mutate := range map[string]func(*ProjectEnvironmentPolicyManifest){
		"non-loopback probe": func(value *ProjectEnvironmentPolicyManifest) { value.Services[0].Probe.BindAddress = "192.0.2.1" },
		"unsafe script path": func(value *ProjectEnvironmentPolicyManifest) { value.Services[0].ScriptPath = "../server.js" },
		"duplicate port": func(value *ProjectEnvironmentPolicyManifest) {
			other := value.Services[0]
			other.ID = "api-copy"
			other.ScriptPath = "copy.js"
			value.Services = append(value.Services, other)
		},
	} {
		t.Run(name, func(t *testing.T) {
			candidate := base
			candidate.Services = []ProjectServiceDefinition{valid}
			mutate(&candidate)
			if _, _, _, err := CanonicalizeWindowsNodeEnvironmentPolicy(candidate); err == nil {
				t.Fatal("unsafe service definition was accepted")
			}
		})
	}
}
