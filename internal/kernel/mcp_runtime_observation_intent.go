// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

type StdioMCPRuntimeObservationReservationInput struct {
	CapabilityID              string
	CapabilityQualificationID string
	PackageRevisionID         string
	PackageManifestSHA256     string
	RequestID                 string
}

type StdioMCPRuntimeObservationReservation struct {
	Start    bool
	Existing *StdioMCPRuntimeQualification
}

func (k *Kernel) TXReserveStdioMCPRuntimeObservation(ctx context.Context, companyID string, input StdioMCPRuntimeObservationReservationInput) (StdioMCPRuntimeObservationReservation, error) {
	if ctx == nil || !core.ValidID(companyID) || !core.ValidID(input.CapabilityID) || !core.ValidID(input.CapabilityQualificationID) ||
		!core.ValidID(input.PackageRevisionID) || !core.ValidID(input.RequestID) || !validCapabilityDigest(input.PackageManifestSHA256) {
		return StdioMCPRuntimeObservationReservation{}, core.Malformed
	}
	writeKey := stableCapabilityID("mcp-runtime-observation-reserve", companyID, input.RequestID)
	created := false
	_, err := k.TXWrite(ctx, k.LocalScope(companyID), nil, writeKey, "capability.mcp.runtime.observe.reserve", input, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureCapabilityCatalogCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		if err := lockCapabilityMCPServerInTransaction(ctx, tx, companyID, input.CapabilityID); err != nil {
			return Receipt{}, err
		}
		if err := validateCurrentStdioMCPObservationTarget(ctx, tx, companyID, input); err != nil {
			return Receipt{}, err
		}
		intentFingerprint := fingerprint(struct {
			CompanyID                 string
			CapabilityID              string
			CapabilityQualificationID string
			PackageRevisionID         string
			PackageManifestSHA256     string
		}{companyID, input.CapabilityID, input.CapabilityQualificationID, input.PackageRevisionID, input.PackageManifestSHA256})
		if _, err := tx.Exec(ctx, `INSERT INTO mcp_runtime_observation_intents(
company_id,request_id,capability_id,capability_qualification_id,package_revision_id,package_manifest_sha256,fingerprint)
VALUES($1,$2,$3,$4,$5,$6,$7)`, companyID, input.RequestID, input.CapabilityID, input.CapabilityQualificationID,
			input.PackageRevisionID, input.PackageManifestSHA256, intentFingerprint); err != nil {
			if isUniqueViolation(err) {
				return Receipt{}, core.Conflict
			}
			return Receipt{}, err
		}
		if err := appendMCPRuntimeObservationIntentEvent(ctx, tx, companyID, input.RequestID, input.PackageRevisionID, input.PackageManifestSHA256,
			"reserved", "", "package observation reserved before process start"); err != nil {
			return Receipt{}, err
		}
		created = true
		return Receipt{ID: input.RequestID, Status: "reserved"}, nil
	})
	if err != nil {
		return StdioMCPRuntimeObservationReservation{}, err
	}
	if created {
		return StdioMCPRuntimeObservationReservation{Start: true}, nil
	}
	var status, runtimeQualificationID string
	err = k.pool.QueryRow(ctx, `SELECT status,COALESCE(runtime_qualification_id,'')
FROM mcp_runtime_observation_intent_events
WHERE company_id=$1 AND observation_request_id=$2 ORDER BY event_seq DESC LIMIT 1`, companyID, input.RequestID).Scan(&status, &runtimeQualificationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return StdioMCPRuntimeObservationReservation{}, core.Integrity
	}
	if err != nil {
		return StdioMCPRuntimeObservationReservation{}, err
	}
	if status != "completed" || runtimeQualificationID == "" {
		return StdioMCPRuntimeObservationReservation{}, core.Conflict
	}
	record, err := k.getStdioMCPRuntimeQualification(ctx, companyID, runtimeQualificationID)
	if err != nil {
		return StdioMCPRuntimeObservationReservation{}, err
	}
	return StdioMCPRuntimeObservationReservation{Existing: &record}, nil
}

func (k *Kernel) TXMarkStdioMCPRuntimeObservationUnknown(ctx context.Context, companyID, requestID, rationale string) error {
	if ctx == nil || !core.ValidID(companyID) || !core.ValidID(requestID) || len(rationale) > 512 {
		return core.Malformed
	}
	writeKey := stableCapabilityID("mcp-runtime-observation-unknown", companyID, requestID)
	_, err := k.TXWrite(ctx, k.LocalScope(companyID), nil, writeKey, "capability.mcp.runtime.observe.unknown", struct {
		RequestID string
		Rationale string
	}{requestID, rationale}, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureCapabilityCatalogCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		var capabilityID, packageRevisionID, packageDigest, priorStatus string
		if err := tx.QueryRow(ctx, `SELECT i.capability_id,i.package_revision_id,i.package_manifest_sha256,e.status
FROM mcp_runtime_observation_intents i
JOIN LATERAL (SELECT status FROM mcp_runtime_observation_intent_events
 WHERE company_id=i.company_id AND observation_request_id=i.request_id ORDER BY event_seq DESC LIMIT 1) e ON true
WHERE i.company_id=$1 AND i.request_id=$2 FOR SHARE OF i`, companyID, requestID).Scan(&capabilityID, &packageRevisionID, &packageDigest, &priorStatus); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if priorStatus == "completed" || priorStatus == "outcome_unknown" {
			return Receipt{ID: requestID, Status: priorStatus}, nil
		}
		if priorStatus != "reserved" {
			return Receipt{}, core.Conflict
		}
		if err := appendMCPRuntimeObservationIntentEvent(ctx, tx, companyID, requestID, packageRevisionID, packageDigest,
			"outcome_unknown", "", rationale); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: requestID, Status: "outcome_unknown"}, nil
	})
	return err
}

func (k *Kernel) TXReconcileStdioMCPRuntimeObservationIntents(ctx context.Context, reconciliationID string) error {
	if ctx == nil || !core.ValidID(reconciliationID) {
		return core.Malformed
	}
	rows, err := k.pool.Query(ctx, `SELECT DISTINCT i.company_id,i.capability_id
FROM mcp_runtime_observation_intents i
JOIN LATERAL (SELECT status FROM mcp_runtime_observation_intent_events
 WHERE company_id=i.company_id AND observation_request_id=i.request_id ORDER BY event_seq DESC LIMIT 1) e ON true
WHERE e.status='reserved' ORDER BY i.company_id,i.capability_id`)
	if err != nil {
		return err
	}
	type observationServer struct{ companyID, serverID string }
	servers := make([]observationServer, 0)
	for rows.Next() {
		var server observationServer
		if err = rows.Scan(&server.companyID, &server.serverID); err != nil {
			rows.Close()
			return err
		}
		servers = append(servers, server)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	for _, server := range servers {
		lockedContext, releaseServerLock, lockErr := k.LockStdioMCPPackageServer(ctx, server.companyID, server.serverID)
		if errors.Is(lockErr, core.Conflict) {
			continue
		}
		if lockErr != nil {
			return lockErr
		}
		reconcileErr := func() error {
			defer releaseServerLock()
			writeKey := stableCapabilityID("mcp-runtime-observation-reconcile", reconciliationID, server.companyID, server.serverID)
			_, txErr := k.TXWrite(lockedContext, k.LocalScope(server.companyID), nil, writeKey, "capability.mcp.runtime.observe.reconcile", struct {
				ReconciliationID string
				ServerID         string
			}{reconciliationID, server.serverID}, func(tx pgx.Tx) (Receipt, error) {
				if err := ensureCapabilityCatalogCompanyWritable(lockedContext, tx, server.companyID); err != nil {
					return Receipt{}, err
				}
				if err := lockCapabilityMCPServerInTransaction(lockedContext, tx, server.companyID, server.serverID); err != nil {
					return Receipt{}, err
				}
				pendingRows, queryErr := tx.Query(lockedContext, `SELECT i.request_id,i.package_revision_id,i.package_manifest_sha256
FROM mcp_runtime_observation_intents i
JOIN LATERAL (SELECT status FROM mcp_runtime_observation_intent_events
 WHERE company_id=i.company_id AND observation_request_id=i.request_id ORDER BY event_seq DESC LIMIT 1) e ON true
				WHERE i.company_id=$1 AND i.capability_id=$2 AND e.status='reserved' ORDER BY i.request_id FOR UPDATE OF i`, server.companyID, server.serverID)
				if queryErr != nil {
					return Receipt{}, queryErr
				}
				type pendingObservation struct{ requestID, packageID, manifestDigest string }
				pending := make([]pendingObservation, 0)
				for pendingRows.Next() {
					var item pendingObservation
					if queryErr = pendingRows.Scan(&item.requestID, &item.packageID, &item.manifestDigest); queryErr != nil {
						pendingRows.Close()
						return Receipt{}, queryErr
					}
					pending = append(pending, item)
				}
				pendingRows.Close()
				if queryErr = pendingRows.Err(); queryErr != nil {
					return Receipt{}, queryErr
				}
				for _, item := range pending {
					if queryErr = appendMCPRuntimeObservationIntentEvent(lockedContext, tx, server.companyID, item.requestID, item.packageID, item.manifestDigest,
						"outcome_unknown", "", "process restarted before observation result was recorded"); queryErr != nil {
						return Receipt{}, queryErr
					}
				}
				return Receipt{ID: server.serverID, Status: "outcome_unknown"}, nil
			})
			return txErr
		}()
		if reconcileErr != nil {
			return reconcileErr
		}
	}
	return nil
}

func (k *Kernel) TXRevokeStdioMCPRuntimeQualificationsForPackageRevision(ctx context.Context, companyID, serverID, packageRevisionID, observerID string) error {
	if ctx == nil || !core.ValidID(companyID) || !core.ValidID(serverID) || !core.ValidID(packageRevisionID) || !core.ValidID(observerID) {
		return core.Malformed
	}
	writeKey := stableCapabilityID("mcp-runtime-workspace-close", companyID, observerID, packageRevisionID)
	_, err := k.TXWrite(ctx, k.LocalScope(companyID), nil, writeKey, "capability.mcp.runtime.workspace_closed", struct {
		ServerID          string
		PackageRevisionID string
		ObserverID        string
	}{serverID, packageRevisionID, observerID}, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureCapabilityCatalogCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		if err := lockCapabilityMCPServerInTransaction(ctx, tx, companyID, serverID); err != nil {
			return Receipt{}, err
		}
		rows, err := tx.Query(ctx, `SELECT r.runtime_qualification_id,r.version_digest,r.tool_schema_sha256,e.status
FROM mcp_runtime_qualification_records r
JOIN LATERAL (SELECT status FROM mcp_runtime_qualification_events
 WHERE company_id=r.company_id AND runtime_qualification_id=r.runtime_qualification_id ORDER BY event_seq DESC LIMIT 1) e ON true
WHERE r.company_id=$1 AND r.capability_id=$2 AND r.process_spec->>'SourcePackageRevisionID'=$3
 AND e.status IN ('observed_unqualified','qualified') ORDER BY r.runtime_qualification_id`, companyID, serverID, packageRevisionID)
		if err != nil {
			return Receipt{}, err
		}
		type activeRuntime struct{ id, version, schema string }
		active := make([]activeRuntime, 0)
		for rows.Next() {
			var item activeRuntime
			var status string
			if err = rows.Scan(&item.id, &item.version, &item.schema, &status); err != nil {
				rows.Close()
				return Receipt{}, err
			}
			active = append(active, item)
		}
		rows.Close()
		if err = rows.Err(); err != nil {
			return Receipt{}, err
		}
		for _, item := range active {
			requestID := stableCapabilityID("mcp-runtime-workspace-revoke", observerID, item.id)
			eventID := stableCapabilityID("mcp-runtime-workspace-revoked", observerID, item.id)
			if _, err = tx.Exec(ctx, `INSERT INTO mcp_runtime_qualification_events(
company_id,event_id,runtime_qualification_id,capability_id,version_digest,status,tool_schema_sha256,rationale,request_id)
VALUES($1,$2,$3,$4,$5,'revoked',$6,'AppContainer workspace closed; re-observation required',$7)`, companyID, eventID, item.id, serverID, item.version, item.schema, requestID); err != nil {
				return Receipt{}, err
			}
		}
		return Receipt{ID: serverID, Status: "revoked"}, nil
	})
	return err
}

func (k *Kernel) TXRevokeStdioMCPRuntimeQualificationsOutsideSandbox(ctx context.Context, sandboxRoot, observerID string) error {
	if ctx == nil || observerID == "" {
		return core.Malformed
	}
	rows, err := k.pool.Query(ctx, `SELECT DISTINCT company_id FROM mcp_runtime_qualification_records
WHERE process_spec->>'SourcePackageRevisionID' IS NOT NULL AND process_spec->'Launch'->>'WorkspaceRoot' IS DISTINCT FROM $1 ORDER BY company_id`, sandboxRoot)
	if err != nil {
		return err
	}
	companies := make([]string, 0)
	for rows.Next() {
		var companyID string
		if err = rows.Scan(&companyID); err != nil {
			rows.Close()
			return err
		}
		companies = append(companies, companyID)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	for _, companyID := range companies {
		writeKey := stableCapabilityID("mcp-runtime-stale-sandbox", observerID, companyID)
		_, err = k.TXWrite(ctx, k.LocalScope(companyID), nil, writeKey, "capability.mcp.runtime.stale_sandbox", struct {
			SandboxRoot string
			ObserverID  string
		}{sandboxRoot, observerID}, func(tx pgx.Tx) (Receipt, error) {
			rows, queryErr := tx.Query(ctx, `SELECT r.runtime_qualification_id,r.capability_id,r.version_digest,r.tool_schema_sha256,e.status
FROM mcp_runtime_qualification_records r
JOIN LATERAL (SELECT status FROM mcp_runtime_qualification_events
 WHERE company_id=r.company_id AND runtime_qualification_id=r.runtime_qualification_id ORDER BY event_seq DESC LIMIT 1) e ON true
WHERE r.company_id=$1 AND r.process_spec->>'SourcePackageRevisionID' IS NOT NULL
 AND process_spec->'Launch'->>'WorkspaceRoot' IS DISTINCT FROM $2
 AND e.status IN ('observed_unqualified','qualified') ORDER BY r.runtime_qualification_id`, companyID, sandboxRoot)
			if queryErr != nil {
				return Receipt{}, queryErr
			}
			type staleRuntime struct{ id, capability, version, schema string }
			stale := make([]staleRuntime, 0)
			for rows.Next() {
				var item staleRuntime
				var status string
				if queryErr = rows.Scan(&item.id, &item.capability, &item.version, &item.schema, &status); queryErr != nil {
					rows.Close()
					return Receipt{}, queryErr
				}
				stale = append(stale, item)
			}
			rows.Close()
			if queryErr = rows.Err(); queryErr != nil {
				return Receipt{}, queryErr
			}
			for _, item := range stale {
				requestID := stableCapabilityID("mcp-runtime-stale-sandbox-revoke", observerID, item.id)
				eventID := stableCapabilityID("mcp-runtime-stale-sandbox-revoked", observerID, item.id)
				if _, queryErr = tx.Exec(ctx, `INSERT INTO mcp_runtime_qualification_events(
company_id,event_id,runtime_qualification_id,capability_id,version_digest,status,tool_schema_sha256,rationale,request_id)
VALUES($1,$2,$3,$4,$5,'revoked',$6,'AppContainer workspace changed; re-observation required',$7)`, companyID, eventID, item.id, item.capability, item.version, item.schema, requestID); queryErr != nil {
					return Receipt{}, queryErr
				}
			}
			return Receipt{ID: companyID, Status: "revoked"}, nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func validateCurrentStdioMCPObservationTarget(ctx context.Context, tx pgx.Tx, companyID string, input StdioMCPRuntimeObservationReservationInput) error {
	var descriptorDigest, descriptorStatus, transport string
	if err := tx.QueryRow(ctx, `SELECT descriptor_digest,status,transport FROM mcp_server_definitions WHERE company_id=$1 AND id=$2 FOR SHARE`, companyID, input.CapabilityID).Scan(&descriptorDigest, &descriptorStatus, &transport); errors.Is(err, pgx.ErrNoRows) {
		return core.OutOfScope
	} else if err != nil {
		return err
	}
	if transport != "stdio" || descriptorStatus != "approved" {
		return core.Denied
	}
	var qualificationStatus, profile, versionDigest string
	if err := tx.QueryRow(ctx, `SELECT status,profile,version_digest FROM capability_qualification_records
WHERE company_id=$1 AND qualification_id=$2 AND capability_kind='mcp' AND capability_id=$3`, companyID, input.CapabilityQualificationID, input.CapabilityID).Scan(&qualificationStatus, &profile, &versionDigest); errors.Is(err, pgx.ErrNoRows) {
		return core.Denied
	} else if err != nil {
		return err
	}
	if qualificationStatus != "metadata_verified" || profile != "stdio_mcp@1" || versionDigest != descriptorDigest {
		return core.Denied
	}
	var decision string
	if err := tx.QueryRow(ctx, `SELECT decision FROM capability_decisions
WHERE company_id=$1 AND capability_kind='mcp' AND capability_id=$2 AND version_digest=$3 AND qualification_id=$4
ORDER BY created_at DESC,decision_id DESC LIMIT 1`, companyID, input.CapabilityID, descriptorDigest, input.CapabilityQualificationID).Scan(&decision); errors.Is(err, pgx.ErrNoRows) {
		return core.Denied
	} else if err != nil {
		return err
	}
	if decision != "approved" {
		return core.Denied
	}
	var latestRevision, latestDigest string
	if err := tx.QueryRow(ctx, `SELECT package_revision_id,manifest_digest FROM mcp_server_package_revisions
WHERE company_id=$1 AND server_id=$2 ORDER BY created_at DESC,package_revision_id LIMIT 1 FOR SHARE`, companyID, input.CapabilityID).Scan(&latestRevision, &latestDigest); err != nil {
		return core.Denied
	}
	if latestRevision != input.PackageRevisionID || latestDigest != input.PackageManifestSHA256 {
		return core.Denied
	}
	return nil
}

func appendMCPRuntimeObservationIntentEvent(ctx context.Context, tx pgx.Tx, companyID, originalRequestID, packageRevisionID, packageDigest, status, runtimeQualificationID, rationale string) error {
	requestID := stableCapabilityID("mcp-runtime-observation-event-request", originalRequestID, status)
	eventID := stableCapabilityID("mcp-runtime-observation-event", companyID, originalRequestID, status)
	_, err := tx.Exec(ctx, `INSERT INTO mcp_runtime_observation_intent_events(
company_id,event_id,observation_request_id,status,package_revision_id,package_manifest_sha256,runtime_qualification_id,rationale,request_id)
VALUES($1,$2,$3,$4,$5,$6,NULLIF($7,''),$8,$9)`, companyID, eventID, originalRequestID, status,
		packageRevisionID, packageDigest, runtimeQualificationID, rationale, requestID)
	return err
}
