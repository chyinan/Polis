// pattern: Functional Core
package probe

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
)

const RuntimeDatabaseBindingSchema = "r03a-runtime-database-binding@1"
const WindowsDatabaseAccessStrategyRevision = "r03a-windows-localhost-forwarding@1"
const WindowsWSLDirectTCPStrategyRevision = "r03a-windows-wsl-direct-tcp@1"

type RuntimeDatabaseBinding struct {
	SchemaVersion           string `json:"schema_version"`
	PostgresMajor           int    `json:"postgres_major"`
	ClusterSystemIdentifier string `json:"cluster_system_identifier"`
	DatabaseName            string `json:"database_name"`
	DatabaseOID             string `json:"database_oid"`
	SchemaMigrationIdentity string `json:"schema_migration_identity"`
	SemanticClosureDigest   string `json:"semantic_closure_digest,omitempty"`
	Fingerprint             string `json:"fingerprint"`
}

type DatabaseAccessView struct {
	SchemaVersion             string `json:"schema_version"`
	ConsumerOS                string `json:"consumer_os"`
	Transport                 string `json:"transport"`
	Host                      string `json:"host"`
	Port                      int    `json:"port"`
	SocketDirectory           string `json:"socket_directory,omitempty"`
	EndpointSource            string `json:"endpoint_source"`
	StrategyRevision          string `json:"access_strategy_revision"`
	RuntimeBindingFingerprint string `json:"runtime_database_binding_fingerprint"`
	TargetWSLDistribution     string `json:"target_wsl_distribution,omitempty"`
	ResolutionMethod          string `json:"resolution_method,omitempty"`
	HBAAddress                string `json:"hba_source_address,omitempty"`
}

type DatabaseAccessPreflightReport struct {
	Status                 string                 `json:"status"`
	ReasonCode             string                 `json:"reason_code,omitempty"`
	ActionableSummary      string                 `json:"actionable_summary,omitempty"`
	ConsumerOS             string                 `json:"consumer_os"`
	ConfiguredDSN          string                 `json:"configured_dsn_redacted"`
	AccessView             DatabaseAccessView     `json:"access_view"`
	RuntimeBinding         RuntimeDatabaseBinding `json:"runtime_database_binding"`
	ObservedPostgresMajor  int                    `json:"observed_postgres_major,omitempty"`
	ObservedClusterID      string                 `json:"observed_cluster_system_identifier,omitempty"`
	ObservedDatabaseName   string                 `json:"observed_database_name,omitempty"`
	ObservedDatabaseOID    string                 `json:"observed_database_oid,omitempty"`
	ObservedSchemaIdentity string                 `json:"observed_schema_migration_identity,omitempty"`
	Mutation               bool                   `json:"mutation"`
}

func (b RuntimeDatabaseBinding) RecomputeFingerprint() RuntimeDatabaseBinding {
	b.Fingerprint = ""
	raw, _ := json.Marshal(b)
	digest := sha256.Sum256(raw)
	b.Fingerprint = hex.EncodeToString(digest[:])
	return b
}

func (b RuntimeDatabaseBinding) Validate() error {
	if b.SchemaVersion != RuntimeDatabaseBindingSchema || b.PostgresMajor <= 0 || b.ClusterSystemIdentifier == "" || b.DatabaseName == "" || b.DatabaseOID == "" || b.SchemaMigrationIdentity == "" || b.Fingerprint == "" {
		return errors.New("runtime database binding is incomplete")
	}
	if b.RecomputeFingerprint().Fingerprint != b.Fingerprint {
		return errors.New("runtime database binding fingerprint mismatch")
	}
	return nil
}

func (v DatabaseAccessView) Validate(binding RuntimeDatabaseBinding) error {
	if v.ConsumerOS != "windows" || v.Transport != "tcp" || v.Host == "" || v.Port <= 0 || v.EndpointSource == "" || v.RuntimeBindingFingerprint != binding.Fingerprint {
		return errors.New("database access view is incomplete or not bound to the runtime database")
	}
	if v.SchemaVersion == "r03a-database-access-view@1" && v.StrategyRevision == WindowsDatabaseAccessStrategyRevision {
		return nil
	}
	if v.SchemaVersion == "r03a-database-access-view@2" && v.StrategyRevision == WindowsWSLDirectTCPStrategyRevision && net.ParseIP(v.Host) != nil && !net.ParseIP(v.Host).IsLoopback() && v.TargetWSLDistribution != "" && v.ResolutionMethod != "" && net.ParseIP(strings.TrimSuffix(v.HBAAddress, "/32")) != nil {
		return nil
	}
	return errors.New("database access view is incomplete or uses an unsupported strategy")
}

func LoadRuntimeDatabaseBinding(path string) (RuntimeDatabaseBinding, error) {
	var binding RuntimeDatabaseBinding
	raw, err := os.ReadFile(path)
	if err != nil {
		return binding, err
	}
	if err := json.Unmarshal(raw, &binding); err != nil {
		return binding, err
	}
	return binding, binding.Validate()
}

func LoadDatabaseAccessView(path string, binding RuntimeDatabaseBinding) (DatabaseAccessView, error) {
	var view DatabaseAccessView
	raw, err := os.ReadFile(path)
	if err != nil {
		return view, err
	}
	if err := json.Unmarshal(raw, &view); err != nil {
		return view, err
	}
	return view, view.Validate(binding)
}

func DiscoverRuntimeDatabaseBinding(ctx context.Context, dsn string) (RuntimeDatabaseBinding, error) {
	var binding RuntimeDatabaseBinding
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		return binding, err
	}
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return binding, err
	}
	defer conn.Close(ctx)
	var serverVersion, databaseName, databaseOID, clusterID, schemaIdentity string
	if err := conn.QueryRow(ctx, "SHOW server_version_num").Scan(&serverVersion); err != nil {
		return binding, err
	}
	var versionNumber int
	if _, err := fmt.Sscanf(serverVersion, "%d", &versionNumber); err != nil {
		return binding, err
	}
	if err := conn.QueryRow(ctx, "SELECT current_database(), oid::text FROM pg_database WHERE datname=current_database()").Scan(&databaseName, &databaseOID); err != nil {
		return binding, err
	}
	if err := conn.QueryRow(ctx, "SELECT system_identifier::text FROM pg_control_system()").Scan(&clusterID); err != nil {
		return binding, err
	}
	if err := conn.QueryRow(ctx, "SELECT COALESCE(max(version_id),0)::text FROM goose_db_version WHERE is_applied").Scan(&schemaIdentity); err != nil {
		return binding, err
	}
	binding = RuntimeDatabaseBinding{SchemaVersion: RuntimeDatabaseBindingSchema, PostgresMajor: versionNumber / 10000, ClusterSystemIdentifier: clusterID, DatabaseName: databaseName, DatabaseOID: databaseOID, SchemaMigrationIdentity: schemaIdentity}.RecomputeFingerprint()
	return binding, binding.Validate()
}

func WindowsDatabaseAccessPreflight(ctx context.Context, dsn string, binding RuntimeDatabaseBinding, view DatabaseAccessView) (DatabaseAccessPreflightReport, error) {
	report := DatabaseAccessPreflightReport{Status: "WINDOWS_DATABASE_ACCESS_PREFLIGHT_FAILED", ConsumerOS: "windows", ConfiguredDSN: redactDSN(dsn), AccessView: view, RuntimeBinding: binding, Mutation: false}
	if err := binding.Validate(); err != nil {
		report.ReasonCode, report.ActionableSummary = "runtime_database_binding_invalid", err.Error()
		return report, err
	}
	if err := view.Validate(binding); err != nil {
		report.ReasonCode, report.ActionableSummary = "database_access_view_invalid", err.Error()
		return report, err
	}
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		report.ReasonCode, report.ActionableSummary = "database_access_config_invalid", "database connection configuration could not be parsed"
		return report, err
	}
	if config.Host != view.Host || config.Port != uint16(view.Port) {
		report.ReasonCode, report.ActionableSummary = "database_access_view_mismatch", "configured DB endpoint does not match the qualified Windows access view"
		return report, errors.New(report.ActionableSummary)
	}
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		report.ReasonCode, report.ActionableSummary = "database_access_unavailable", "Windows database endpoint could not be reached"
		return report, err
	}
	defer conn.Close(ctx)
	var serverVersion string
	var databaseName, databaseOID string
	var clusterID string
	var schemaIdentity string
	if err = conn.QueryRow(ctx, "SHOW server_version_num").Scan(&serverVersion); err != nil {
		return databaseAccessQueryFailure(&report, err)
	}
	if _, err = fmt.Sscanf(serverVersion, "%d", new(int)); err != nil {
		return databaseAccessQueryFailure(&report, err)
	}
	var versionNumber int
	_, _ = fmt.Sscanf(serverVersion, "%d", &versionNumber)
	report.ObservedPostgresMajor = versionNumber / 10000
	if err = conn.QueryRow(ctx, "SELECT current_database(), oid::text FROM pg_database WHERE datname=current_database()").Scan(&databaseName, &databaseOID); err != nil {
		return databaseAccessQueryFailure(&report, err)
	}
	if err = conn.QueryRow(ctx, "SELECT system_identifier::text FROM pg_control_system()").Scan(&clusterID); err != nil {
		return databaseAccessQueryFailure(&report, err)
	}
	if err = conn.QueryRow(ctx, "SELECT COALESCE(max(version_id),0)::text FROM goose_db_version WHERE is_applied").Scan(&schemaIdentity); err != nil {
		return databaseAccessQueryFailure(&report, err)
	}
	report.ObservedClusterID, report.ObservedDatabaseName, report.ObservedDatabaseOID, report.ObservedSchemaIdentity = clusterID, databaseName, databaseOID, schemaIdentity
	if report.ObservedPostgresMajor != binding.PostgresMajor || clusterID != binding.ClusterSystemIdentifier || databaseName != binding.DatabaseName || databaseOID != binding.DatabaseOID || schemaIdentity != binding.SchemaMigrationIdentity {
		report.ReasonCode, report.ActionableSummary = "database_identity_mismatch", "reachable PostgreSQL does not match the authoritative runtime database binding"
		return report, errors.New(report.ActionableSummary)
	}
	report.Status = "WINDOWS_DATABASE_ACCESS_PREFLIGHT_PASSED"
	return report, nil
}

func databaseAccessQueryFailure(report *DatabaseAccessPreflightReport, err error) (DatabaseAccessPreflightReport, error) {
	report.ReasonCode, report.ActionableSummary = "database_identity_query_failed", "database endpoint was reachable but authoritative identity could not be read"
	return *report, err
}

func redactDSN(dsn string) string {
	if strings.Contains(dsn, "password=") {
		parts := strings.Fields(dsn)
		for i, part := range parts {
			if strings.HasPrefix(part, "password=") {
				parts[i] = "password=<redacted>"
			}
		}
		return strings.Join(parts, " ")
	}
	return dsn
}
