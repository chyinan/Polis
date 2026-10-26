// pattern: Imperative Shell
package kernel

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
	"polis/internal/environment"
)

type DependencyChangeMaterializationRecord struct {
	CompanyID            string `json:"companyId"`
	MaterializationID    string `json:"materializationId"`
	ProposalID           string `json:"proposalId"`
	BaseRevisionID       string `json:"baseRevisionId"`
	RequestID            string `json:"requestId"`
	RequestSHA256        string `json:"requestSha256"`
	State                string `json:"state"`
	ReasonCode           string `json:"reasonCode"`
	DerivedInputID       string `json:"derivedInputId,omitempty"`
	DerivedInputRevision string `json:"derivedInputRevision,omitempty"`
	SuccessorRevisionID  string `json:"successorRevisionId,omitempty"`
	PackageJSONSHA256    string `json:"packageJsonSha256,omitempty"`
	LockfileSHA256       string `json:"lockfileSha256,omitempty"`
	ResolverLogsSHA256   string `json:"resolverLogsSha256,omitempty"`
	CreatedAt            string `json:"createdAt"`
}

type DependencyChangeMaterializationFinish struct {
	MaterializationID    string
	State                string
	ReasonCode           string
	DerivedInputID       string
	DerivedInputRevision int64
	SuccessorRevisionID  string
	PackageJSONSHA256    string
	LockfileSHA256       string
	ResolverLogsSHA256   string
}

// TXBeginDependencyChangeMaterialization durably claims the approved proposal
// before any resolver can run. Replays with the same request ID return the
// original attempt; running and unknown attempts are never replayed.
func (k *Kernel) TXBeginDependencyChangeMaterialization(ctx context.Context, scope Scope, proposalID, requestID string) (DependencyChangeMaterializationRecord, bool, error) {
	if k == nil {
		return DependencyChangeMaterializationRecord{}, false, core.StaleEpoch
	}
	if !core.ValidID(scope.company) || !core.ValidID(proposalID) || !core.ValidID(requestID) {
		return DependencyChangeMaterializationRecord{}, false, core.Malformed
	}
	payload := struct {
		ProposalID string
		RequestID  string
	}{proposalID, requestID}
	requestSHA256 := fingerprint(payload)
	materializationID := stableCapabilityID("dependency-materialization", scope.company, requestID)
	started := false
	_, err := k.TXWrite(ctx, scope, nil, requestID, "environment.dependency_change.materialize.start", payload, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureEnvironmentCompanyWritable(ctx, tx, scope.company); err != nil {
			return Receipt{}, err
		}
		var baseRevisionID, storedProposalSHA256 string
		var rawProposal []byte
		if err := tx.QueryRow(ctx, `SELECT base_revision_id,proposal,proposal_sha256
FROM environment_dependency_change_proposals WHERE company_id=$1 AND proposal_id=$2 FOR UPDATE`, scope.company, proposalID).Scan(&baseRevisionID, &rawProposal, &storedProposalSHA256); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		var proposal environment.DependencyChangeProposal
		if err := json.Unmarshal(rawProposal, &proposal); err != nil {
			return Receipt{}, core.Integrity
		}
		_, _, digest, canonicalErr := environment.CanonicalizeDependencyChangeProposal(proposal)
		if canonicalErr != nil || digest != storedProposalSHA256 || proposal.BaseRevisionID != baseRevisionID {
			return Receipt{}, core.Integrity
		}
		var proposalState string
		if err := tx.QueryRow(ctx, `SELECT state FROM environment_dependency_change_events WHERE company_id=$1 AND proposal_id=$2 ORDER BY event_seq DESC LIMIT 1 FOR UPDATE`, scope.company, proposalID).Scan(&proposalState); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.Integrity
		} else if err != nil {
			return Receipt{}, err
		}
		if proposalState != "approved" {
			return Receipt{}, core.ConflictError{Reason: "dependency change proposal is not approved", CurrentState: proposalState}
		}
		var revisionMissionID, profileID, policySHA256, toolchainSHA256 string
		var policyManifest []byte
		if err := tx.QueryRow(ctx, `SELECT mission_id,profile_id,policy_sha256,policy_manifest,toolchain_sha256
FROM project_environment_revisions WHERE company_id=$1 AND revision_id=$2 FOR SHARE`, scope.company, baseRevisionID).Scan(&revisionMissionID, &profileID, &policySHA256, &policyManifest, &toolchainSHA256); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		policy, _, actualPolicySHA256, policyErr := environment.ParseProjectEnvironmentPolicy(policyManifest)
		if policyErr != nil || actualPolicySHA256 != policySHA256 || revisionMissionID != proposal.MissionID || !dependencyChangeHostsAllowed(policy.RegistryHosts, proposal.RegistryHosts) {
			return Receipt{}, core.Denied
		}
		policyDecision, policyDecisionSHA256, err := latestEnvironmentPolicyDecision(ctx, tx, scope.company, baseRevisionID)
		if err != nil {
			return Receipt{}, err
		}
		if policyDecision != "approved" || policyDecisionSHA256 != policySHA256 {
			return Receipt{}, core.ConflictError{Reason: "base environment policy is not currently approved", CurrentState: policyDecision}
		}
		qualified, err := k.environmentExecutorQualified(ctx, tx, scope.company, profileID, toolchainSHA256)
		if err != nil {
			return Receipt{}, err
		}
		if !qualified {
			return Receipt{}, core.ConflictError{Reason: "base environment executor qualification is unavailable", CurrentState: "blocked_unqualified"}
		}
		var latestState string
		err = tx.QueryRow(ctx, `SELECT e.state FROM environment_dependency_change_materializations m
JOIN LATERAL (SELECT state FROM environment_dependency_change_materialization_events
 WHERE company_id=m.company_id AND materialization_id=m.materialization_id ORDER BY event_seq DESC LIMIT 1) e ON true
WHERE m.company_id=$1 AND m.proposal_id=$2 ORDER BY m.created_at DESC LIMIT 1 FOR UPDATE OF m`, scope.company, proposalID).Scan(&latestState)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, err
		}
		if err == nil && (latestState == "running" || latestState == "outcome_unknown") {
			return Receipt{}, core.ConflictError{Reason: "a dependency materialization attempt is unresolved and will not be replayed", CurrentState: latestState}
		}
		if err == nil && latestState == "materialized" {
			return Receipt{}, core.ConflictError{Reason: "dependency change is already materialized", CurrentState: latestState}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO environment_dependency_change_materializations(company_id,materialization_id,proposal_id,base_revision_id,request_id,request_sha256)
VALUES($1,$2,$3,$4,$5,$6)`, scope.company, materializationID, proposalID, baseRevisionID, requestID, requestSHA256); err != nil {
			return Receipt{}, err
		}
		eventID := stableCapabilityID("dependency-materialization-event", materializationID, "running")
		if _, err := tx.Exec(ctx, `INSERT INTO environment_dependency_change_materialization_events(company_id,event_id,materialization_id,state,reason_code)
VALUES($1,$2,$3,'running','resolver_not_started')`, scope.company, eventID, materializationID); err != nil {
			return Receipt{}, err
		}
		started = true
		return Receipt{ID: materializationID, Status: "running"}, nil
	})
	if err != nil {
		return DependencyChangeMaterializationRecord{}, false, err
	}
	record, err := k.DependencyChangeMaterializationByRequest(ctx, scope, requestID)
	return record, started, err
}

// TXFinishDependencyChangeMaterialization records exactly one terminal event.
// An unknown result is durable and cannot be retried with a successor request.
func (k *Kernel) TXFinishDependencyChangeMaterialization(ctx context.Context, scope Scope, input DependencyChangeMaterializationFinish) (DependencyChangeMaterializationRecord, error) {
	if k == nil {
		return DependencyChangeMaterializationRecord{}, core.StaleEpoch
	}
	if !core.ValidID(scope.company) || !core.ValidID(input.MaterializationID) ||
		(input.State != "blocked" && input.State != "failed" && input.State != "outcome_unknown" && input.State != "materialized") ||
		len(input.ReasonCode) > 128 || strings.ContainsAny(input.ReasonCode, "\x00\r\n") {
		return DependencyChangeMaterializationRecord{}, core.Malformed
	}
	if input.State == "materialized" {
		if !core.ValidID(input.DerivedInputID) || input.DerivedInputRevision <= 0 || !core.ValidID(input.SuccessorRevisionID) ||
			!validEnvironmentSHA256(input.PackageJSONSHA256) || !validEnvironmentSHA256(input.LockfileSHA256) {
			return DependencyChangeMaterializationRecord{}, core.Malformed
		}
	} else if input.DerivedInputID != "" || input.DerivedInputRevision != 0 || input.SuccessorRevisionID != "" || input.PackageJSONSHA256 != "" || input.LockfileSHA256 != "" {
		return DependencyChangeMaterializationRecord{}, core.Malformed
	}
	if input.ResolverLogsSHA256 != "" && !validEnvironmentSHA256(input.ResolverLogsSHA256) {
		return DependencyChangeMaterializationRecord{}, core.Malformed
	}
	requestID := stableCapabilityID("dependency-materialization-finish", scope.company, input.MaterializationID, input.State)
	_, err := k.TXWrite(ctx, scope, nil, requestID, "environment.dependency_change.materialize.finish", input, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureEnvironmentCompanyWritable(ctx, tx, scope.company); err != nil {
			return Receipt{}, err
		}
		var baseRevisionID, proposalID string
		if err := tx.QueryRow(ctx, `SELECT base_revision_id,proposal_id FROM environment_dependency_change_materializations
WHERE company_id=$1 AND materialization_id=$2 FOR UPDATE`, scope.company, input.MaterializationID).Scan(&baseRevisionID, &proposalID); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		var currentState string
		if err := tx.QueryRow(ctx, `SELECT state FROM environment_dependency_change_materialization_events
WHERE company_id=$1 AND materialization_id=$2 ORDER BY event_seq DESC LIMIT 1 FOR UPDATE`, scope.company, input.MaterializationID).Scan(&currentState); err != nil {
			return Receipt{}, err
		}
		if currentState != "running" {
			return Receipt{}, core.ConflictError{Reason: "dependency materialization attempt is already terminal", CurrentState: currentState}
		}
		if input.State == "materialized" {
			var missionID string
			var currentProposalState string
			if err := tx.QueryRow(ctx, `SELECT p.mission_id,e.state FROM environment_dependency_change_proposals p
JOIN LATERAL (SELECT state FROM environment_dependency_change_events WHERE company_id=p.company_id AND proposal_id=p.proposal_id ORDER BY event_seq DESC LIMIT 1) e ON true
WHERE p.company_id=$1 AND p.proposal_id=$2`, scope.company, proposalID).Scan(&missionID, &currentProposalState); err != nil {
				return Receipt{}, err
			}
			if currentProposalState != "approved" {
				return Receipt{}, core.Denied
			}
			var profileID, policySHA256, toolchainSHA256 string
			var policyManifest []byte
			if err := tx.QueryRow(ctx, `SELECT profile_id,policy_sha256,policy_manifest,toolchain_sha256 FROM project_environment_revisions WHERE company_id=$1 AND revision_id=$2 FOR SHARE`, scope.company, baseRevisionID).Scan(&profileID, &policySHA256, &policyManifest, &toolchainSHA256); err != nil {
				return Receipt{}, err
			}
			_, _, actualPolicySHA256, policyErr := environment.ParseProjectEnvironmentPolicy(policyManifest)
			policyDecision, policyDecisionSHA256, err := latestEnvironmentPolicyDecision(ctx, tx, scope.company, baseRevisionID)
			if err != nil {
				return Receipt{}, err
			}
			if policyErr != nil || actualPolicySHA256 != policySHA256 || policyDecision != "approved" || policyDecisionSHA256 != policySHA256 {
				return Receipt{}, core.Denied
			}
			qualified, err := k.environmentExecutorQualified(ctx, tx, scope.company, profileID, toolchainSHA256)
			if err != nil {
				return Receipt{}, err
			}
			if !qualified {
				return Receipt{}, core.Denied
			}
			var storedInputMission, inputState string
			if err := tx.QueryRow(ctx, `SELECT mission_id,state FROM mission_inputs WHERE company_id=$1 AND input_id=$2 AND revision=$3`, scope.company, input.DerivedInputID, input.DerivedInputRevision).Scan(&storedInputMission, &inputState); err != nil {
				return Receipt{}, err
			}
			if storedInputMission != missionID || (inputState != "usable" && inputState != "partial") {
				return Receipt{}, core.OutOfScope
			}
			var successorSourceID, successorPackageSHA256, successorLockSHA256 string
			var successorSourceRevision int64
			if err := tx.QueryRow(ctx, `SELECT source_input_id,source_input_revision,package_json_sha256,lockfile_sha256 FROM project_environment_revisions WHERE company_id=$1 AND revision_id=$2`, scope.company, input.SuccessorRevisionID).Scan(&successorSourceID, &successorSourceRevision, &successorPackageSHA256, &successorLockSHA256); err != nil {
				return Receipt{}, err
			}
			if successorSourceID != input.DerivedInputID || successorSourceRevision != input.DerivedInputRevision || successorPackageSHA256 != input.PackageJSONSHA256 || successorLockSHA256 != input.LockfileSHA256 {
				return Receipt{}, core.Integrity
			}
		}
		eventID := stableCapabilityID("dependency-materialization-event", input.MaterializationID, input.State)
		_, err := tx.Exec(ctx, `INSERT INTO environment_dependency_change_materialization_events(company_id,event_id,materialization_id,state,reason_code,derived_input_id,derived_input_revision,successor_revision_id,package_json_sha256,lockfile_sha256,resolver_logs_sha256)
VALUES($1,$2,$3,$4,$5,NULLIF($6,''),NULLIF($7,0),NULLIF($8,''),NULLIF($9,''),NULLIF($10,''),NULLIF($11,''))`, scope.company, eventID, input.MaterializationID, input.State, input.ReasonCode, input.DerivedInputID, input.DerivedInputRevision, input.SuccessorRevisionID, input.PackageJSONSHA256, input.LockfileSHA256, input.ResolverLogsSHA256)
		if err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: input.MaterializationID, Status: input.State}, nil
	})
	if err != nil {
		return DependencyChangeMaterializationRecord{}, err
	}
	return k.DependencyChangeMaterialization(ctx, scope, input.MaterializationID)
}

func (k *Kernel) StoreDependencyChangeMaterializationLogs(ctx context.Context, scope Scope, materializationID string, logs []byte) (string, error) {
	if k == nil {
		return "", core.StaleEpoch
	}
	if !core.ValidID(scope.company) || !core.ValidID(materializationID) || len(logs) == 0 || len(logs) > 64<<10 {
		return "", core.Malformed
	}
	record, err := k.DependencyChangeMaterialization(ctx, scope, materializationID)
	if err != nil {
		return "", err
	}
	if record.State != "running" {
		return "", core.ConflictError{Reason: "materialization logs can only be stored for a running attempt", CurrentState: record.State}
	}
	return k.putBlobWithClaim(ctx, scope.company, logs)
}

func (k *Kernel) DependencyChangeMaterializationByRequest(ctx context.Context, scope Scope, requestID string) (DependencyChangeMaterializationRecord, error) {
	if !core.ValidID(scope.company) || !core.ValidID(requestID) {
		return DependencyChangeMaterializationRecord{}, core.Malformed
	}
	var materializationID string
	if err := k.pool.QueryRow(ctx, `SELECT materialization_id FROM environment_dependency_change_materializations WHERE company_id=$1 AND request_id=$2`, scope.company, requestID).Scan(&materializationID); errors.Is(err, pgx.ErrNoRows) {
		return DependencyChangeMaterializationRecord{}, core.OutOfScope
	} else if err != nil {
		return DependencyChangeMaterializationRecord{}, err
	}
	return k.DependencyChangeMaterialization(ctx, scope, materializationID)
}

func (k *Kernel) DependencyChangeMaterialization(ctx context.Context, scope Scope, materializationID string) (DependencyChangeMaterializationRecord, error) {
	if !core.ValidID(scope.company) || !core.ValidID(materializationID) {
		return DependencyChangeMaterializationRecord{}, core.Malformed
	}
	var record DependencyChangeMaterializationRecord
	var inputID, successorRevision, packageDigest, lockDigest, logsDigest sql.NullString
	var inputRevision sql.NullInt64
	var createdAt time.Time
	err := k.pool.QueryRow(ctx, `SELECT m.company_id,m.materialization_id,m.proposal_id,m.base_revision_id,m.request_id,m.request_sha256,
e.state,e.reason_code,e.derived_input_id,e.derived_input_revision,e.successor_revision_id,e.package_json_sha256,e.lockfile_sha256,e.resolver_logs_sha256,m.created_at
FROM environment_dependency_change_materializations m
JOIN LATERAL (SELECT state,reason_code,derived_input_id,derived_input_revision,successor_revision_id,package_json_sha256,lockfile_sha256,resolver_logs_sha256
 FROM environment_dependency_change_materialization_events WHERE company_id=m.company_id AND materialization_id=m.materialization_id ORDER BY event_seq DESC LIMIT 1) e ON true
WHERE m.company_id=$1 AND m.materialization_id=$2`, scope.company, materializationID).Scan(
		&record.CompanyID, &record.MaterializationID, &record.ProposalID, &record.BaseRevisionID, &record.RequestID, &record.RequestSHA256,
		&record.State, &record.ReasonCode, &inputID, &inputRevision, &successorRevision, &packageDigest, &lockDigest, &logsDigest, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return DependencyChangeMaterializationRecord{}, core.OutOfScope
	}
	if err != nil {
		return DependencyChangeMaterializationRecord{}, err
	}
	record.DerivedInputID = inputID.String
	if inputRevision.Valid {
		record.DerivedInputRevision = strconv.FormatInt(inputRevision.Int64, 10)
	}
	record.SuccessorRevisionID = successorRevision.String
	record.PackageJSONSHA256 = packageDigest.String
	record.LockfileSHA256 = lockDigest.String
	record.ResolverLogsSHA256 = logsDigest.String
	record.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	return record, nil
}

func (k *Kernel) LatestDependencyChangeMaterialization(ctx context.Context, scope Scope, proposalID string) (DependencyChangeMaterializationRecord, error) {
	if !core.ValidID(scope.company) || !core.ValidID(proposalID) {
		return DependencyChangeMaterializationRecord{}, core.Malformed
	}
	var id string
	err := k.pool.QueryRow(ctx, `SELECT materialization_id FROM environment_dependency_change_materializations
WHERE company_id=$1 AND proposal_id=$2 ORDER BY created_at DESC LIMIT 1`, scope.company, proposalID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return DependencyChangeMaterializationRecord{}, core.OutOfScope
	}
	if err != nil {
		return DependencyChangeMaterializationRecord{}, err
	}
	return k.DependencyChangeMaterialization(ctx, scope, id)
}

// TXReconcileUnrestoredDependencyChangeMaterializations is called only after
// Kernel.Open has acquired the process-wide runtime lock. Any resolver still
// marked running therefore belongs to a previous runtime and is outcome_unknown;
// this recovery path never invokes or replays a resolver.
func (k *Kernel) TXReconcileUnrestoredDependencyChangeMaterializations(ctx context.Context, startupRequestID string) (int, error) {
	if k == nil {
		return 0, core.StaleEpoch
	}
	if !core.ValidID(startupRequestID) {
		return 0, core.Malformed
	}
	rows, err := k.pool.Query(ctx, `SELECT DISTINCT m.company_id FROM environment_dependency_change_materializations m
JOIN LATERAL (SELECT state FROM environment_dependency_change_materialization_events
 WHERE company_id=m.company_id AND materialization_id=m.materialization_id ORDER BY event_seq DESC LIMIT 1) e ON true
WHERE e.state='running' ORDER BY m.company_id`)
	if err != nil {
		return 0, err
	}
	companies := make([]string, 0, 8)
	for rows.Next() {
		var companyID string
		if err := rows.Scan(&companyID); err != nil {
			rows.Close()
			return 0, err
		}
		companies = append(companies, companyID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	total := 0
	for _, companyID := range companies {
		scope := k.LocalScope(companyID)
		recoveryID := stableCapabilityID("dependency-materialization-recovery", startupRequestID, companyID)
		var reconciled int
		receipt, err := k.TXWrite(ctx, scope, nil, recoveryID, "environment.dependency_change.materialize.reconcile", startupRequestID, func(tx pgx.Tx) (Receipt, error) {
			attemptRows, err := tx.Query(ctx, `SELECT m.materialization_id FROM environment_dependency_change_materializations m
JOIN LATERAL (SELECT state FROM environment_dependency_change_materialization_events
 WHERE company_id=m.company_id AND materialization_id=m.materialization_id ORDER BY event_seq DESC LIMIT 1) e ON true
WHERE m.company_id=$1 AND e.state='running' ORDER BY m.materialization_id FOR UPDATE OF m`, companyID)
			if err != nil {
				return Receipt{}, err
			}
			ids := make([]string, 0, 4)
			for attemptRows.Next() {
				var materializationID string
				if err := attemptRows.Scan(&materializationID); err != nil {
					attemptRows.Close()
					return Receipt{}, err
				}
				ids = append(ids, materializationID)
			}
			attemptRows.Close()
			if err := attemptRows.Err(); err != nil {
				return Receipt{}, err
			}
			for _, materializationID := range ids {
				eventID := stableCapabilityID("dependency-materialization-event", materializationID, "outcome_unknown")
				if _, err := tx.Exec(ctx, `INSERT INTO environment_dependency_change_materialization_events(company_id,event_id,materialization_id,state,reason_code)
VALUES($1,$2,$3,'outcome_unknown','runtime_restarted_during_resolution')`, companyID, eventID, materializationID); err != nil {
					return Receipt{}, err
				}
			}
			reconciled = len(ids)
			return Receipt{ID: companyID, Status: "reconciled", Revision: int64(reconciled)}, nil
		})
		if err != nil {
			return total, err
		}
		if reconciled == 0 {
			reconciled = int(receipt.Revision)
		}
		total += reconciled
	}
	return total, nil
}
