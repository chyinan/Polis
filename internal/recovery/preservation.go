// pattern: Functional Core
package recovery

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

const PreservedPostgresAccessManifestRevision = "r0.3a-preserved-postgres-access@1"

type PreservedPostgresAccessManifest struct {
	ManifestRevision          string `json:"manifest_revision"`
	PostgresVersion           string `json:"postgres_version"`
	PGDataCanonicalPath       string `json:"pgdata_canonical_path"`
	PGDataDigest              string `json:"pgdata_digest"`
	ClusterSystemIdentifier   string `json:"cluster_system_identifier"`
	DatabaseName              string `json:"database_name"`
	DatabaseOID               string `json:"database_oid"`
	DatabaseOwner             string `json:"database_owner"`
	RuntimeRoleName           string `json:"runtime_role_name"`
	RoleProvisioningRevision  string `json:"role_provisioning_revision"`
	RoleGrantSpecDigest       string `json:"role_grant_spec_digest"`
	MigrationSchemaDigest     string `json:"migration_schema_digest"`
	CompanyID                 string `json:"company_id"`
	MissionID                 string `json:"mission_id"`
	BackendTaskID             string `json:"backend_task_id"`
	FrontendTaskID            string `json:"frontend_task_id"`
	CreatedAt                 string `json:"created_at"`
	PreservationReason        string `json:"preservation_reason"`
	CredentialsIncluded       bool   `json:"credentials_included"`
	ManifestCreationPhase     string `json:"manifest_creation_phase,omitempty"`
	HistoricalManifestPresent bool   `json:"historical_manifest_present,omitempty"`
	Source                    string `json:"source,omitempty"`
	BusinessStateSource       string `json:"business_state_source,omitempty"`
	ForensicClonePath         string `json:"forensic_clone_path,omitempty"`
	ForensicCloneManifest     string `json:"forensic_clone_file_manifest_sha256,omitempty"`
}

type PostgresPhysicalInspection struct {
	PostgresVersion         string
	PGDataCanonicalPath     string
	PGDataDigest            string
	PGVersionPresent        bool
	PGControlPresent        bool
	ClusterSystemIdentifier string
	DatabaseName            string
	DatabaseOID             string
	DatabaseOwner           string
	RuntimeRolePresent      bool
}

type ReopenStatus string

const (
	ReopenQualified               ReopenStatus = "QUALIFIED_REOPEN"
	ReopenIncomplete              ReopenStatus = "PRESERVATION_INCOMPLETE"
	ReopenClusterIdentityMismatch ReopenStatus = "PRESERVED_CLUSTER_IDENTITY_MISMATCH"
	ReopenRuntimeRoleMissing      ReopenStatus = "RUNTIME_ROLE_PRESERVATION_OR_REPROVISION_DEFECT"
	ReopenSourceDatabaseMissing   ReopenStatus = "SOURCE_DATABASE_MISSING"
)

type ReopenDecision struct {
	Status ReopenStatus
	Reason string
}

var ErrPreservationIncomplete = errors.New("preservation manifest incomplete")
var ErrPreservedManifestHashMismatch = errors.New("preservation manifest hash mismatch")
var ErrPreservedDataDirGuard = errors.New("preserved PGDATA mutation denied")

func ValidateManifest(m PreservedPostgresAccessManifest) error {
	if m.ManifestRevision != PreservedPostgresAccessManifestRevision || m.PostgresVersion == "" || m.PGDataCanonicalPath == "" || m.PGDataDigest == "" || m.ClusterSystemIdentifier == "" || m.DatabaseName == "" || m.DatabaseOID == "" || m.DatabaseOwner == "" || m.RuntimeRoleName == "" || m.RoleProvisioningRevision == "" || m.RoleGrantSpecDigest == "" || m.MigrationSchemaDigest == "" || m.CompanyID == "" || m.MissionID == "" || m.BackendTaskID == "" || m.FrontendTaskID == "" || m.CreatedAt == "" || m.PreservationReason == "" {
		return ErrPreservationIncomplete
	}
	if m.CredentialsIncluded {
		return fmt.Errorf("%w: credentials_included must be false", ErrPreservationIncomplete)
	}
	if !isAbsolutePath(m.PGDataCanonicalPath) {
		return fmt.Errorf("%w: pgdata path must be absolute", ErrPreservationIncomplete)
	}
	return nil
}

func ValidateSalvageManifest(m PreservedPostgresAccessManifest) error {
	if err := ValidateManifest(m); err != nil {
		return err
	}
	if m.ManifestCreationPhase != "post_failure_salvage" || m.HistoricalManifestPresent || m.Source != "physical_cluster_inspection" || m.BusinessStateSource != "existing_database_only" || m.ForensicClonePath == "" || m.ForensicCloneManifest == "" {
		return fmt.Errorf("%w: salvage provenance is incomplete or misclassified", ErrPreservationIncomplete)
	}
	return nil
}

func ValidateManifestHash(declared, actual string) error {
	if strings.TrimSpace(declared) == "" || strings.TrimSpace(actual) == "" || !strings.EqualFold(strings.TrimSpace(declared), strings.TrimSpace(actual)) {
		return ErrPreservedManifestHashMismatch
	}
	return nil
}

func isAbsolutePath(path string) bool {
	if filepath.IsAbs(path) {
		return true
	}
	return len(path) >= 3 && ((path[0] >= 'A' && path[0] <= 'Z') || (path[0] >= 'a' && path[0] <= 'z')) && path[1] == ':' && (path[2] == '\\' || path[2] == '/')
}

func DecideReopen(m PreservedPostgresAccessManifest, i PostgresPhysicalInspection) ReopenDecision {
	if err := ValidateManifest(m); err != nil {
		return ReopenDecision{Status: ReopenIncomplete, Reason: err.Error()}
	}
	if !i.PGVersionPresent || !i.PGControlPresent || i.ClusterSystemIdentifier == "" || i.DatabaseName == "" || i.DatabaseOID == "" || i.PGDataCanonicalPath == "" {
		return ReopenDecision{Status: ReopenIncomplete, Reason: "required physical identity is not observable"}
	}
	if filepath.Clean(i.PGDataCanonicalPath) != filepath.Clean(m.PGDataCanonicalPath) || i.PGDataDigest != m.PGDataDigest || i.ClusterSystemIdentifier != m.ClusterSystemIdentifier || i.PostgresVersion != m.PostgresVersion || i.DatabaseName != m.DatabaseName || i.DatabaseOID != m.DatabaseOID || i.DatabaseOwner != m.DatabaseOwner {
		return ReopenDecision{Status: ReopenClusterIdentityMismatch, Reason: "physical cluster or database identity differs from preservation manifest"}
	}
	if !i.RuntimeRolePresent {
		return ReopenDecision{Status: ReopenRuntimeRoleMissing, Reason: "exact preserved cluster and database are present but runtime role is missing"}
	}
	return ReopenDecision{Status: ReopenQualified, Reason: "physical cluster, database and runtime access identity match"}
}

func GuardPreservedPGDataAction(m PreservedPostgresAccessManifest, action string, recoveryComplete bool) error {
	if err := ValidateManifest(m); err != nil {
		return err
	}
	if recoveryComplete || strings.EqualFold(action, "authorized_preservation_release") {
		return nil
	}
	switch strings.ToLower(action) {
	case "initdb", "cleanup", "recreate", "temp-dir-reset", "delete", "drop":
		return fmt.Errorf("%w: action=%s path=%s", ErrPreservedDataDirGuard, action, m.PGDataCanonicalPath)
	default:
		return nil
	}
}
