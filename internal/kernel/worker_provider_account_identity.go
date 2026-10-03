// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/hex"
	"strings"
	"time"

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
		if snapshot.Status == "available" {
			// The Company lifecycle row is already locked by txWrite. Global
			// account keys are acquired only after that lock; future operations
			// touching both scopes must follow the same order.
			if _, err := tx.Exec(ctx, `INSERT INTO provider_account_registry(provider_class,account_fingerprint,first_seen_at)
VALUES($1,$2,clock_timestamp()) ON CONFLICT(provider_class,account_fingerprint) DO NOTHING`, snapshot.ProviderClass, snapshot.Fingerprint); err != nil {
				return Receipt{}, err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO provider_account_observations(company_id,worker_session_id,provider_class,account_fingerprint)
VALUES($1,$2,$3,$4)`, b.scope.company, b.session, snapshot.ProviderClass, snapshot.Fingerprint); err != nil {
				return Receipt{}, err
			}
		}
		return Receipt{ID: b.session, Status: "provider_account_identity_recorded"}, nil
	})
}

type ObservedProviderAccount struct {
	ProviderClass      string    `json:"providerClass"`
	LocatorFingerprint string    `json:"locatorFingerprint"`
	FirstSeenAt        time.Time `json:"firstSeenAt"`
	LastSeenAt         time.Time `json:"lastSeenAt"`
	CompanyCount       int64     `json:"companyCount"`
	WorkerSessionCount int64     `json:"workerSessionCount"`
}

type ObservedProviderAccountList struct {
	SchemaVersion string                    `json:"schemaVersion"`
	Accounts      []ObservedProviderAccount `json:"accounts"`
}

// ListObservedProviderAccounts is an installation-owner-only, read-only
// projection. The account locator remains an observation, not a billing unit.
func (k *Kernel) ListObservedProviderAccounts(ctx context.Context, scope InstallationOwnerScope, limit int) (ObservedProviderAccountList, error) {
	if k == nil || k.pool == nil || ctx == nil || scope.incarnation == "" || scope.incarnation != k.incarnation || limit < 1 || limit > 500 {
		return ObservedProviderAccountList{}, core.Denied
	}
	tx, err := k.pool.Begin(ctx)
	if err != nil {
		return ObservedProviderAccountList{}, err
	}
	defer tx.Rollback(ctx)
	if err := k.checkRuntimeLease(ctx, tx); err != nil {
		return ObservedProviderAccountList{}, err
	}
	rows, err := tx.Query(ctx, `SELECT r.provider_class,r.account_fingerprint,r.first_seen_at,
 max(s.captured_at),count(DISTINCT o.company_id)::bigint,count(*)::bigint
FROM provider_account_registry r
JOIN provider_account_observations o USING(provider_class,account_fingerprint)
JOIN worker_provider_account_identity_snapshots s
 ON s.company_id=o.company_id AND s.worker_session_id=o.worker_session_id
 AND s.provider_class=o.provider_class AND s.account_fingerprint=o.account_fingerprint
GROUP BY r.provider_class,r.account_fingerprint,r.first_seen_at
ORDER BY max(s.captured_at) DESC,r.provider_class,r.account_fingerprint
LIMIT $1`, limit)
	if err != nil {
		return ObservedProviderAccountList{}, err
	}
	defer rows.Close()
	result := ObservedProviderAccountList{SchemaVersion: "observed-provider-accounts@1", Accounts: make([]ObservedProviderAccount, 0)}
	for rows.Next() {
		var account ObservedProviderAccount
		if err := rows.Scan(&account.ProviderClass, &account.LocatorFingerprint, &account.FirstSeenAt, &account.LastSeenAt, &account.CompanyCount, &account.WorkerSessionCount); err != nil {
			return ObservedProviderAccountList{}, err
		}
		result.Accounts = append(result.Accounts, account)
	}
	if err := rows.Err(); err != nil {
		return ObservedProviderAccountList{}, err
	}
	return result, nil
}
