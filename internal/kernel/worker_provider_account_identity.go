// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/hex"
	"strings"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

const WorkerProviderAccountIdentitySnapshotSchemaV1 = "provider-account-identity@1"

type WorkerProviderAccountIdentitySnapshot struct {
	SchemaVersion string `json:"schema_version"`
	ProviderClass string `json:"provider_class"`
	Status        string `json:"status"`
	Fingerprint   string `json:"fingerprint,omitempty"`
	ReasonCode    string `json:"reason_code,omitempty"`
}

func (s WorkerProviderAccountIdentitySnapshot) Validate() error {
	if s.SchemaVersion != WorkerProviderAccountIdentitySnapshotSchemaV1 || !core.ValidID(s.ProviderClass) {
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

// TXRecordWorkerProviderAccountIdentity binds an opaque provider account
// locator to a WorkerSession before execution. The locator is not proof of
// billing scope and does not enable account-level financial controls.
func (k *Kernel) TXRecordWorkerProviderAccountIdentity(ctx context.Context, b Binding, snapshot WorkerProviderAccountIdentitySnapshot) (Receipt, error) {
	if k == nil || ctx == nil || !core.ValidID(b.scope.company) || !core.ValidID(b.session) || snapshot.Validate() != nil {
		return Receipt{}, core.Malformed
	}
	requestID := stableCapabilityID("provider-account-identity-snapshot", b.scope.company, b.session)
	return k.txWrite(ctx, b.scope, &b, requestID, "worker.provider_account_identity.snapshot", snapshot, false, func(tx pgx.Tx) (Receipt, error) {
		state, err := k.checkSession(ctx, tx, b, false)
		if err != nil {
			return Receipt{}, err
		}
		if state != "restoring" {
			return Receipt{}, core.Conflict
		}
		if _, err := tx.Exec(ctx, `INSERT INTO worker_provider_account_identity_snapshots(company_id,worker_session_id,
snapshot_schema_version,provider_class,identity_status,account_fingerprint,reason_code)
VALUES($1,$2,$3,$4,$5,NULLIF($6,''),NULLIF($7,''))`, b.scope.company, b.session,
			snapshot.SchemaVersion, snapshot.ProviderClass, snapshot.Status, snapshot.Fingerprint, snapshot.ReasonCode); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: b.session, Status: "provider_account_identity_recorded"}, nil
	})
}
