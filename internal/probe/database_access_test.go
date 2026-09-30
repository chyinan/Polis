// pattern: Functional Core
package probe

import "testing"

func testRuntimeDatabaseBinding() RuntimeDatabaseBinding {
	return RuntimeDatabaseBinding{
		SchemaVersion:           RuntimeDatabaseBindingSchema,
		PostgresMajor:           18,
		ClusterSystemIdentifier: "7685937128561021478",
		DatabaseName:            "polis_r0_3a_frontend_clean_baseline_v6",
		DatabaseOID:             "16384",
		SchemaMigrationIdentity: "5",
	}.RecomputeFingerprint()
}

func TestRuntimeDatabaseBindingFingerprintRoundTrip(t *testing.T) {
	binding := testRuntimeDatabaseBinding()
	if err := binding.Validate(); err != nil {
		t.Fatalf("valid runtime database binding rejected: %v", err)
	}
	binding.ClusterSystemIdentifier = "wrong"
	if err := binding.Validate(); err == nil {
		t.Fatal("runtime database binding drift accepted")
	}
}

func TestDatabaseAccessViewBindsRuntimeIdentity(t *testing.T) {
	binding := testRuntimeDatabaseBinding()
	view := DatabaseAccessView{
		SchemaVersion:             "r03a-database-access-view@1",
		ConsumerOS:                "windows",
		Transport:                 "tcp",
		Host:                      "127.0.0.1",
		Port:                      55432,
		EndpointSource:            "qualified-localhost-forwarding",
		StrategyRevision:          WindowsDatabaseAccessStrategyRevision,
		RuntimeBindingFingerprint: binding.Fingerprint,
	}
	if err := view.Validate(binding); err != nil {
		t.Fatalf("valid database access view rejected: %v", err)
	}
	view.Port = 55434
	if err := view.Validate(binding); err != nil {
		t.Fatalf("endpoint representation should remain structurally valid: %v", err)
	}
	view.RuntimeBindingFingerprint = "wrong"
	if err := view.Validate(binding); err == nil {
		t.Fatal("access view bound to a different runtime database was accepted")
	}
}

func TestDatabaseAccessViewV2RequiresResolvedDirectWSLHost(t *testing.T) {
	binding := testRuntimeDatabaseBinding()
	view := DatabaseAccessView{
		SchemaVersion:              "r03a-database-access-view@2",
		ConsumerOS:                 "windows",
		Transport:                 "tcp",
		Host:                      "172.24.72.52",
		Port:                      55432,
		EndpointSource:            "v7-qualified-wsl-direct-tcp",
		StrategyRevision:          WindowsWSLDirectTCPStrategyRevision,
		RuntimeBindingFingerprint: binding.Fingerprint,
		TargetWSLDistribution:     "Ubuntu-22.04",
		ResolutionMethod:          "wsl.exe -d Ubuntu-22.04 -- hostname -I",
		HBAAddress:                "172.24.64.1/32",
	}
	if err := view.Validate(binding); err != nil {
		t.Fatalf("valid direct WSL TCP view rejected: %v", err)
	}
	view.Host = "127.0.0.1"
	if err := view.Validate(binding); err == nil {
		t.Fatal("loopback host accepted for direct WSL TCP view")
	}
}

func TestRedactDSNDoesNotExposePassword(t *testing.T) {
	redacted := redactDSN("host=127.0.0.1 port=55432 user=polis_runtime password=secret dbname=polis")
	if redacted == "" || containsSecret(redacted, "secret") {
		t.Fatalf("password leaked in redacted DSN: %q", redacted)
	}
}

func containsSecret(value, secret string) bool {
	for i := 0; i+len(secret) <= len(value); i++ {
		if value[i:i+len(secret)] == secret {
			return true
		}
	}
	return false
}
