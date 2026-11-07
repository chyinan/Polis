// pattern: Functional Core
package probe

import "testing"

func TestParseT17ConfigRedactsSensitiveValuesAndPreservesEffectiveKeys(t *testing.T) {
	raw := []byte(`model = "gpt-5.6-luna"
model_reasoning_effort = "xhigh"
network_access = "enabled"
[features]
apps = true
[shell_environment_policy.set]
ANTHROPIC_AUTH_TOKEN = "secret"
ANTHROPIC_BASE_URL = "https://example.invalid"
`)
	parsed, err := ParseT17Config(raw, true)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Fields["model"] != "gpt-5.6-luna" || parsed.Fields["features.apps"] != "true" {
		t.Fatalf("parsed fields = %+v", parsed.Fields)
	}
	if parsed.Fields["shell_environment_policy.set.ANTHROPIC_AUTH_TOKEN"] != T17SensitiveRedacted || parsed.Fields["shell_environment_policy.set.ANTHROPIC_BASE_URL"] != T17SensitiveRedacted {
		t.Fatalf("sensitive values were not redacted: %+v", parsed.Fields)
	}
}

func TestCompareT17ConfigFieldsDoesNotTreatAbsentDefaultsAsSame(t *testing.T) {
	a := T17ParsedConfig{Exists: true, Fields: map[string]string{"model": "gpt-5.6-luna"}}
	b := T17ParsedConfig{Exists: true, Fields: map[string]string{"model": "gpt-5.6-luna", "supports_websockets": T17DefaultInB}}
	fields := CompareT17ConfigFields(a, b, []string{"model", "supports_websockets"})
	if fields["model"].Comparison != T17ConfirmedSame {
		t.Fatalf("same field comparison = %+v", fields["model"])
	}
	if fields["supports_websockets"].AClassification != T17Unknown || fields["supports_websockets"].BClassification != T17DefaultInB || fields["supports_websockets"].Comparison != T17Unknown {
		t.Fatalf("default field comparison = %+v", fields["supports_websockets"])
	}
}

func TestT17FileClassificationSeparatesCredentialsConfigCachesAndEphemeralState(t *testing.T) {
	cases := map[string]string{
		"auth.json":             T17CredentialClass,
		"config.toml":           T17ConfigurationClass,
		"models_cache.json":     T17CacheClass,
		"history.jsonl":         T17EphemeralStateClass,
		"thread_history.sqlite": T17EphemeralStateClass,
	}
	for name, want := range cases {
		if got := ClassifyT17File(name); got != want {
			t.Errorf("%s class=%s want=%s", name, got, want)
		}
	}
}

func TestT17ConfigAggregatesCountMCPTablesRatherThanOnlyEnabledKeys(t *testing.T) {
	config := T17ParsedConfig{Fields: map[string]string{
		"mcp_servers.first.command":  "x",
		"mcp_servers.first.enabled":  "false",
		"mcp_servers.second.url":     "y",
		"mcp_servers.second.enabled": "true",
	}}
	addT17ConfigAggregates(&config)
	if config.Fields["mcp_server_count"] != "2" {
		t.Fatalf("MCP table count = %q", config.Fields["mcp_server_count"])
	}
}
