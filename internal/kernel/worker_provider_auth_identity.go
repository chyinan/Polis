// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/hex"
	"strings"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

const WorkerProviderAuthIdentitySnapshotSchemaV1 = "provider-auth-identity@1"

type WorkerProviderAuthIdentitySnapshot struct {
	SchemaVersion string `json:"schema_version"`
	SourceClass   string `json:"source_class"`
	Status        string `json:"status"`
	Fingerprint   string `json:"fingerprint,omitempty"`
	ReasonCode    string `json:"reason_code,omitempty"`
}

func (s WorkerProviderAuthIdentitySnapshot) Validate() error {
	if s.SchemaVersion != WorkerProviderAuthIdentitySnapshotSchemaV1 || !core.ValidID(s.SourceClass) {
		return core.Malformed
	}
	switch s.Status {
	case "available":
		raw, err := hex.DecodeString(s.Fingerprint)
		if err != nil || len(raw) != 32 || s.Fingerprint != strings.ToLower(s.Fingerprint) || s.ReasonCode != "" {
			return core.Malformed
		}
	case "unavailable", "unsupported":
		if s.Fingerprint != "" || !core.ValidID(s.ReasonCode) {
			return core.Malformed
		}
	default:
		return core.Malformed
	}
	return nil
}

// TXRecordWorkerProviderAuthIdentity binds a validated provider auth-principal
// observation to one WorkerSession before execution. The fingerprint is not
// treated as a ProviderAccount ID or a financial budget identity.
func (k *Kernel) TXRecordWorkerProviderAuthIdentity(ctx context.Context, b Binding, snapshot WorkerProviderAuthIdentitySnapshot) (Receipt, error) {
	if k == nil || ctx == nil || !core.ValidID(b.scope.company) || !core.ValidID(b.session) || snapshot.Validate() != nil {
		return Receipt{}, core.Malformed
	}
	requestID := stableCapabilityID("provider-auth-snapshot", b.scope.company, b.session)
	return k.txWrite(ctx, b.scope, &b, requestID, "worker.provider_auth_identity.snapshot", snapshot, false, func(tx pgx.Tx) (Receipt, error) {
		state, err := k.checkSession(ctx, tx, b, false)
		if err != nil {
			return Receipt{}, err
		}
		if state != "restoring" {
			return Receipt{}, core.Conflict
		}
		if _, err := tx.Exec(ctx, `INSERT INTO worker_provider_auth_identity_snapshots(company_id,worker_session_id,
snapshot_schema_version,source_class,identity_status,identity_fingerprint,reason_code)
VALUES($1,$2,$3,$4,$5,NULLIF($6,''),NULLIF($7,''))`, b.scope.company, b.session,
			snapshot.SchemaVersion, snapshot.SourceClass, snapshot.Status, snapshot.Fingerprint, snapshot.ReasonCode); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: b.session, Status: "provider_auth_identity_recorded"}, nil
	})
}
