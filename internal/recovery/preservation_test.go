// pattern: Functional Core
package recovery

import "testing"

func validManifest() PreservedPostgresAccessManifest {
	return PreservedPostgresAccessManifest{
		ManifestRevision: PreservedPostgresAccessManifestRevision,
		PostgresVersion:  "18.6", PGDataCanonicalPath: `D:\recovery\pgdata`, PGDataDigest: "pgdata-digest",
		ClusterSystemIdentifier: "cluster-1", DatabaseName: "polis_r0_3a_sample", DatabaseOID: "1234", DatabaseOwner: "chyinan",
		RuntimeRoleName: "polis_runtime", RoleProvisioningRevision: "role-provisioning@1", RoleGrantSpecDigest: "grants-digest",
		MigrationSchemaDigest: "migration-digest", CompanyID: "company", MissionID: "mission", BackendTaskID: "backend-task", FrontendTaskID: "frontend-task",
		CreatedAt: "2026-09-14T00:00:00Z", PreservationReason: "Backend terminal recovery cut", CredentialsIncluded: false,
	}
}

func validInspection() PostgresPhysicalInspection {
	return PostgresPhysicalInspection{
		PostgresVersion: "18.6", PGDataCanonicalPath: `D:\recovery\pgdata`, PGDataDigest: "pgdata-digest",
		PGVersionPresent: true, PGControlPresent: true, ClusterSystemIdentifier: "cluster-1", DatabaseName: "polis_r0_3a_sample", DatabaseOID: "1234", DatabaseOwner: "chyinan", RuntimeRolePresent: true,
	}
}

func TestDecideReopenRequiresExactPhysicalIdentity(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*PostgresPhysicalInspection)
		want   ReopenStatus
	}{
		{"exact match", func(*PostgresPhysicalInspection) {}, ReopenQualified},
		{"missing PG_VERSION", func(i *PostgresPhysicalInspection) { i.PGVersionPresent = false }, ReopenIncomplete},
		{"missing pg_control", func(i *PostgresPhysicalInspection) { i.PGControlPresent = false }, ReopenIncomplete},
		{"new cluster system identifier", func(i *PostgresPhysicalInspection) { i.ClusterSystemIdentifier = "cluster-2" }, ReopenClusterIdentityMismatch},
		{"same DB name different OID", func(i *PostgresPhysicalInspection) { i.DatabaseOID = "9999" }, ReopenClusterIdentityMismatch},
		{"runtime role missing on exact cluster", func(i *PostgresPhysicalInspection) { i.RuntimeRolePresent = false }, ReopenRuntimeRoleMissing},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			i := validInspection()
			tc.mutate(&i)
			if got := DecideReopen(validManifest(), i).Status; got != tc.want {
				t.Fatalf("status=%s want=%s", got, tc.want)
			}
		})
	}
}

func TestPreservedPGDataGuardDeniesDestructiveActionsUntilRecoveryComplete(t *testing.T) {
	m := validManifest()
	for _, action := range []string{"initdb", "cleanup", "recreate", "temp-dir-reset", "drop"} {
		if err := GuardPreservedPGDataAction(m, action, false); err == nil {
			t.Fatalf("action %s was allowed", action)
		}
	}
	if err := GuardPreservedPGDataAction(m, "initdb", true); err != nil {
		t.Fatalf("completed recovery did not release guard: %v", err)
	}
}

func TestManifestRejectsPlaintextCredentialStateAndMissingIdentity(t *testing.T) {
	m := validManifest()
	m.CredentialsIncluded = true
	if err := ValidateManifest(m); err == nil {
		t.Fatal("plaintext credential state was accepted")
	}
	m = validManifest()
	m.DatabaseOID = ""
	if err := ValidateManifest(m); err == nil {
		t.Fatal("missing database OID was accepted")
	}
}

func TestSalvageManifestRequiresExplicitPhysicalProvenance(t *testing.T) {
	m := validManifest()
	m.ManifestCreationPhase = "post_failure_salvage"
	m.Source = "physical_cluster_inspection"
	m.BusinessStateSource = "existing_database_only"
	m.ForensicClonePath = "/home/chyinan/.local/state/polis-recovery/salvage/clone"
	m.ForensicCloneManifest = "clone-manifest-digest"
	if err := ValidateSalvageManifest(m); err != nil {
		t.Fatalf("valid salvage provenance rejected: %v", err)
	}
	m.HistoricalManifestPresent = true
	if err := ValidateSalvageManifest(m); err == nil {
		t.Fatal("salvage manifest marked as historical was accepted")
	}
}

func TestManifestHashMustMatch(t *testing.T) {
	if err := ValidateManifestHash("ABC", "abc"); err != nil {
		t.Fatalf("case-insensitive matching hash rejected: %v", err)
	}
	if err := ValidateManifestHash("abc", "def"); err == nil {
		t.Fatal("corrupt manifest hash was accepted")
	}
}
