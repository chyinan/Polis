// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
	"polis/internal/environment"
)

type ServiceBorrowerLeaseAcquireInput struct {
	JobID      string `json:"jobId"`
	Generation int    `json:"generation"`
}

type ServiceBorrowerLeaseRecord struct {
	CompanyID         string `json:"companyId"`
	LeaseID           string `json:"leaseId"`
	JobID             string `json:"jobId"`
	Generation        int    `json:"generation"`
	MissionID         string `json:"missionId"`
	OwnerTaskID       string `json:"ownerTaskId"`
	OwnerSessionID    string `json:"ownerSessionId"`
	BorrowerTaskID    string `json:"borrowerTaskId"`
	BorrowerSessionID string `json:"borrowerSessionId"`
	State             string `json:"state"`
	ExpiresAt         string `json:"expiresAt"`
	IdleUntil         string `json:"idleUntil"`
	LastUsedAt        string `json:"lastUsedAt"`
	Actor             string `json:"actor"`
	Reason            string `json:"reason"`
	RequestID         string `json:"requestId"`
	CreatedAt         string `json:"createdAt"`
}

func (k *Kernel) TXAcquireServiceBorrowerLease(ctx context.Context, binding Binding, input ServiceBorrowerLeaseAcquireInput, requestID string) (ServiceBorrowerLeaseRecord, error) {
	if k == nil || !core.ValidID(binding.scope.company) || !core.ValidID(binding.task) || !core.ValidID(binding.session) || !core.ValidID(input.JobID) || input.Generation <= 0 || !core.ValidID(requestID) {
		return ServiceBorrowerLeaseRecord{}, core.Malformed
	}
	writeReceipt, err := k.TXWrite(ctx, binding.scope, &binding, requestID, "service.borrower_lease.acquire", input, func(tx pgx.Tx) (Receipt, error) {
		var ownerTaskID, missionID, borrowerMissionID, ownerSessionID, readiness string
		var generation int
		var endpointExpiry time.Time
		if err := tx.QueryRow(ctx, `SELECT j.task_id,t.mission_id,borrower_task.mission_id,j.session_id,e.generation,e.readiness,e.lease_expires_at
FROM job_runs j
JOIN tasks t ON t.company_id=j.company_id AND t.id=j.task_id
JOIN tasks borrower_task ON borrower_task.company_id=j.company_id AND borrower_task.id=$4
JOIN worker_sessions owner ON owner.company_id=j.company_id AND owner.id=j.session_id AND owner.state='active'
JOIN LATERAL (SELECT generation,readiness,lease_expires_at FROM service_endpoint_events WHERE company_id=j.company_id AND job_id=j.job_id ORDER BY generation DESC,event_seq DESC LIMIT 1) e ON true
WHERE j.company_id=$1 AND j.job_id=$2 AND j.kind='service' AND e.generation=$3
FOR UPDATE OF j,owner,borrower_task`, binding.scope.company, input.JobID, input.Generation, binding.task).Scan(&ownerTaskID, &missionID, &borrowerMissionID, &ownerSessionID, &generation, &readiness, &endpointExpiry); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		var now time.Time
		if err := tx.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&now); err != nil {
			return Receipt{}, err
		}
		now = now.UTC()
		if readiness != "ready" || !endpointExpiry.After(now) {
			return Receipt{}, core.ConflictError{Reason: "service endpoint is not currently borrowable", CurrentState: readiness}
		}
		if err := validateServiceBorrowerLeaseScope(binding.scope.company, missionID, borrowerMissionID, ownerTaskID, binding.task, ownerSessionID, binding.session, generation); err != nil {
			return Receipt{}, err
		}
		var existingLeaseID, existingState string
		var existingExpiresAt, existingIdleUntil, existingLastUsedAt time.Time
		if err := tx.QueryRow(ctx, `SELECT l.lease_id,e.state,e.expires_at,e.idle_until,e.last_used_at
FROM service_borrower_lease_records l
JOIN LATERAL (SELECT state,expires_at,idle_until,last_used_at FROM service_borrower_lease_events WHERE company_id=l.company_id AND lease_id=l.lease_id ORDER BY event_seq DESC LIMIT 1) e ON true
WHERE l.company_id=$1 AND l.job_id=$2 AND l.generation=$3 AND l.borrower_session_id=$4`, binding.scope.company, input.JobID, input.Generation, binding.session).Scan(&existingLeaseID, &existingState, &existingExpiresAt, &existingIdleUntil, &existingLastUsedAt); err == nil {
			if existingState == "active" {
				if serviceBorrowerLeaseExpired(now, existingExpiresAt, existingIdleUntil) {
					expiredEventID := stableCapabilityID("service-borrower-lease-event", binding.scope.company, requestID, "expired")
					if _, err := tx.Exec(ctx, `INSERT INTO service_borrower_lease_events(company_id,event_id,lease_id,state,expires_at,idle_until,last_used_at,actor,reason,request_id)
VALUES($1,$2,$3,'expired',$4,$5,$6,'system','borrower lease expired',$7)`, binding.scope.company, expiredEventID, existingLeaseID, existingExpiresAt, existingIdleUntil, existingLastUsedAt, requestID); err != nil {
						return Receipt{}, err
					}
					return Receipt{ID: existingLeaseID, Status: "expired"}, nil
				}
				return Receipt{ID: existingLeaseID, Status: existingState}, nil
			}
			return Receipt{}, core.ConflictError{Reason: "a borrower lease already reached a terminal state for this service generation", CurrentState: existingState}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, err
		}
		expiresAt, idleUntil := serviceBorrowerLeaseTimes(now, endpointExpiry)
		leaseID := stableCapabilityID("service-borrower-lease", binding.scope.company, input.JobID, strconv.Itoa(input.Generation), binding.session)
		eventID := stableCapabilityID("service-borrower-lease-event", binding.scope.company, requestID)
		if _, err := tx.Exec(ctx, `INSERT INTO service_borrower_lease_records(company_id,lease_id,job_id,generation,mission_id,owner_task_id,owner_session_id,borrower_task_id,borrower_session_id)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, binding.scope.company, leaseID, input.JobID, input.Generation, missionID, ownerTaskID, ownerSessionID, binding.task, binding.session); err != nil {
			return Receipt{}, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO service_borrower_lease_events(company_id,event_id,lease_id,state,expires_at,idle_until,last_used_at,actor,reason,request_id)
VALUES($1,$2,$3,'active',$4,$5,$6,$7,$8,$9)`, binding.scope.company, eventID, leaseID, expiresAt, idleUntil, now, binding.employee, "borrower lease granted", requestID); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: leaseID, Status: "active"}, nil
	})
	if err != nil {
		return ServiceBorrowerLeaseRecord{}, err
	}
	return k.ServiceBorrowerLease(ctx, binding.scope, writeReceipt.ID)
}

func (k *Kernel) ServiceBorrowerLease(ctx context.Context, scope Scope, leaseID string) (ServiceBorrowerLeaseRecord, error) {
	if !core.ValidID(scope.company) || !core.ValidID(leaseID) {
		return ServiceBorrowerLeaseRecord{}, core.Malformed
	}
	var record ServiceBorrowerLeaseRecord
	var expiresAt, idleUntil, lastUsedAt, createdAt, databaseNow time.Time
	err := k.pool.QueryRow(ctx, `SELECT l.company_id,l.lease_id,l.job_id,l.generation,l.mission_id,l.owner_task_id,l.owner_session_id,l.borrower_task_id,l.borrower_session_id,e.state,e.expires_at,e.idle_until,e.last_used_at,e.actor,e.reason,e.request_id,l.created_at,clock_timestamp()
FROM service_borrower_lease_records l
JOIN LATERAL (SELECT state,expires_at,idle_until,last_used_at,actor,reason,request_id FROM service_borrower_lease_events WHERE company_id=l.company_id AND lease_id=l.lease_id ORDER BY event_seq DESC LIMIT 1) e ON true
WHERE l.company_id=$1 AND l.lease_id=$2`, scope.company, leaseID).Scan(&record.CompanyID, &record.LeaseID, &record.JobID, &record.Generation, &record.MissionID, &record.OwnerTaskID, &record.OwnerSessionID, &record.BorrowerTaskID, &record.BorrowerSessionID, &record.State, &expiresAt, &idleUntil, &lastUsedAt, &record.Actor, &record.Reason, &record.RequestID, &createdAt, &databaseNow)
	if errors.Is(err, pgx.ErrNoRows) {
		return ServiceBorrowerLeaseRecord{}, core.OutOfScope
	}
	if err != nil {
		return ServiceBorrowerLeaseRecord{}, err
	}
	if record.State == "active" && serviceBorrowerLeaseExpired(databaseNow.UTC(), expiresAt, idleUntil) {
		record.State = "expired"
		record.Actor = "system"
		record.Reason = "borrower lease expired"
	}
	record.ExpiresAt = expiresAt.UTC().Format(time.RFC3339Nano)
	record.IdleUntil = idleUntil.UTC().Format(time.RFC3339Nano)
	record.LastUsedAt = lastUsedAt.UTC().Format(time.RFC3339Nano)
	record.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	return record, nil
}

func (k *Kernel) TXTouchServiceBorrowerLease(ctx context.Context, binding Binding, leaseID, requestID string) (ServiceBorrowerLeaseRecord, error) {
	if k == nil || !core.ValidID(binding.scope.company) || !core.ValidID(binding.session) || !core.ValidID(leaseID) || !core.ValidID(requestID) {
		return ServiceBorrowerLeaseRecord{}, core.Malformed
	}
	writeReceipt, err := k.TXWrite(ctx, binding.scope, &binding, requestID, "service.borrower_lease.touch", leaseID, func(tx pgx.Tx) (Receipt, error) {
		var leaseGeneration, endpointGeneration int
		var borrowerTask, borrowerSession, state, endpointReadiness string
		var expiresAt, idleUntil, lastUsedAt, endpointExpiry time.Time
		if err := tx.QueryRow(ctx, `SELECT l.generation,l.borrower_task_id,l.borrower_session_id,e.state,e.expires_at,e.idle_until,e.last_used_at,endpoint.generation,endpoint.readiness,endpoint.lease_expires_at
FROM service_borrower_lease_records l
JOIN worker_sessions owner ON owner.company_id=l.company_id AND owner.id=l.owner_session_id AND owner.task_id=l.owner_task_id AND owner.state='active'
JOIN LATERAL (SELECT state,expires_at,idle_until,last_used_at FROM service_borrower_lease_events WHERE company_id=l.company_id AND lease_id=l.lease_id ORDER BY event_seq DESC LIMIT 1) e ON true
JOIN LATERAL (SELECT generation,readiness,lease_expires_at FROM service_endpoint_events WHERE company_id=l.company_id AND job_id=l.job_id ORDER BY generation DESC,event_seq DESC LIMIT 1) endpoint ON true
WHERE l.company_id=$1 AND l.lease_id=$2 FOR UPDATE OF l,owner`, binding.scope.company, leaseID).Scan(&leaseGeneration, &borrowerTask, &borrowerSession, &state, &expiresAt, &idleUntil, &lastUsedAt, &endpointGeneration, &endpointReadiness, &endpointExpiry); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if borrowerTask != binding.task || borrowerSession != binding.session {
			return Receipt{}, core.OutOfScope
		}
		if state != "active" {
			return Receipt{}, core.ConflictError{Reason: "borrower lease is not active", CurrentState: state}
		}
		var now time.Time
		if err := tx.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&now); err != nil {
			return Receipt{}, err
		}
		now = now.UTC()
		if endpointGeneration != leaseGeneration || endpointReadiness != string(environment.ServiceReady) || !endpointExpiry.After(now) {
			eventID := stableCapabilityID("service-borrower-lease-event", binding.scope.company, requestID, "endpoint-revoked")
			if _, err := tx.Exec(ctx, `INSERT INTO service_borrower_lease_events(company_id,event_id,lease_id,state,expires_at,idle_until,last_used_at,actor,reason,request_id)
VALUES($1,$2,$3,'revoked',$4,$5,$6,'system','service generation is no longer borrowable',$7)`, binding.scope.company, eventID, leaseID, expiresAt, idleUntil, lastUsedAt, requestID); err != nil {
				return Receipt{}, err
			}
			return Receipt{ID: leaseID, Status: "revoked"}, nil
		}
		if serviceBorrowerLeaseExpired(now, expiresAt, idleUntil) {
			eventID := stableCapabilityID("service-borrower-lease-event", binding.scope.company, requestID, "expired")
			if _, err := tx.Exec(ctx, `INSERT INTO service_borrower_lease_events(company_id,event_id,lease_id,state,expires_at,idle_until,last_used_at,actor,reason,request_id)
VALUES($1,$2,$3,'expired',$4,$5,$6,'system','borrower lease expired',$7)`, binding.scope.company, eventID, leaseID, expiresAt, idleUntil, lastUsedAt, requestID); err != nil {
				return Receipt{}, err
			}
			return Receipt{ID: leaseID, Status: "expired"}, nil
		}
		newIdleUntil := now.Add(ServiceBorrowerLeaseIdleGrace)
		if newIdleUntil.After(expiresAt) {
			newIdleUntil = expiresAt
		}
		eventID := stableCapabilityID("service-borrower-lease-event", binding.scope.company, requestID)
		if _, err := tx.Exec(ctx, `INSERT INTO service_borrower_lease_events(company_id,event_id,lease_id,state,expires_at,idle_until,last_used_at,actor,reason,request_id)
VALUES($1,$2,$3,'active',$4,$5,$6,$7,$8,$9)`, binding.scope.company, eventID, leaseID, expiresAt, newIdleUntil, now, binding.employee, "borrower lease touched", requestID); err != nil {
			return Receipt{}, err
		}
		_ = lastUsedAt
		return Receipt{ID: leaseID, Status: "active"}, nil
	})
	if err != nil {
		return ServiceBorrowerLeaseRecord{}, err
	}
	return k.ServiceBorrowerLease(ctx, binding.scope, writeReceipt.ID)
}

func (k *Kernel) TXReleaseServiceBorrowerLease(ctx context.Context, binding Binding, leaseID, requestID string) (ServiceBorrowerLeaseRecord, error) {
	if k == nil || !core.ValidID(binding.scope.company) || !core.ValidID(binding.task) || !core.ValidID(binding.session) || !core.ValidID(leaseID) || !core.ValidID(requestID) {
		return ServiceBorrowerLeaseRecord{}, core.Malformed
	}
	writeReceipt, err := k.TXWrite(ctx, binding.scope, &binding, requestID, "service.borrower_lease.release", leaseID, func(tx pgx.Tx) (Receipt, error) {
		var leaseGeneration, endpointGeneration int
		var borrowerTask, borrowerSession, state, endpointReadiness string
		var expiresAt, idleUntil, lastUsedAt, endpointExpiry time.Time
		if err := tx.QueryRow(ctx, `SELECT l.generation,l.borrower_task_id,l.borrower_session_id,e.state,e.expires_at,e.idle_until,e.last_used_at,endpoint.generation,endpoint.readiness,endpoint.lease_expires_at
FROM service_borrower_lease_records l
JOIN worker_sessions owner ON owner.company_id=l.company_id AND owner.id=l.owner_session_id AND owner.task_id=l.owner_task_id AND owner.state='active'
JOIN LATERAL (SELECT state,expires_at,idle_until,last_used_at FROM service_borrower_lease_events WHERE company_id=l.company_id AND lease_id=l.lease_id ORDER BY event_seq DESC LIMIT 1) e ON true
JOIN LATERAL (SELECT generation,readiness,lease_expires_at FROM service_endpoint_events WHERE company_id=l.company_id AND job_id=l.job_id ORDER BY generation DESC,event_seq DESC LIMIT 1) endpoint ON true
WHERE l.company_id=$1 AND l.lease_id=$2 FOR UPDATE OF l,owner`, binding.scope.company, leaseID).Scan(&leaseGeneration, &borrowerTask, &borrowerSession, &state, &expiresAt, &idleUntil, &lastUsedAt, &endpointGeneration, &endpointReadiness, &endpointExpiry); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if borrowerTask != binding.task || borrowerSession != binding.session {
			return Receipt{}, core.OutOfScope
		}
		if state != "active" {
			return Receipt{}, core.ConflictError{Reason: "borrower lease is already terminal", CurrentState: state}
		}
		var now time.Time
		if err := tx.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&now); err != nil {
			return Receipt{}, err
		}
		now = now.UTC()
		if endpointGeneration != leaseGeneration || endpointReadiness != string(environment.ServiceReady) || !endpointExpiry.After(now) {
			eventID := stableCapabilityID("service-borrower-lease-event", binding.scope.company, requestID, "endpoint-revoked")
			if _, err := tx.Exec(ctx, `INSERT INTO service_borrower_lease_events(company_id,event_id,lease_id,state,expires_at,idle_until,last_used_at,actor,reason,request_id)
VALUES($1,$2,$3,'revoked',$4,$5,$6,'system','service generation is no longer borrowable',$7)`, binding.scope.company, eventID, leaseID, expiresAt, idleUntil, lastUsedAt, requestID); err != nil {
				return Receipt{}, err
			}
			return Receipt{ID: leaseID, Status: "revoked"}, nil
		}
		if serviceBorrowerLeaseExpired(now, expiresAt, idleUntil) {
			eventID := stableCapabilityID("service-borrower-lease-event", binding.scope.company, requestID, "expired")
			if _, err := tx.Exec(ctx, `INSERT INTO service_borrower_lease_events(company_id,event_id,lease_id,state,expires_at,idle_until,last_used_at,actor,reason,request_id)
VALUES($1,$2,$3,'expired',$4,$5,$6,'system','borrower lease expired',$7)`, binding.scope.company, eventID, leaseID, expiresAt, idleUntil, lastUsedAt, requestID); err != nil {
				return Receipt{}, err
			}
			return Receipt{ID: leaseID, Status: "expired"}, nil
		}
		eventID := stableCapabilityID("service-borrower-lease-event", binding.scope.company, requestID)
		if _, err := tx.Exec(ctx, `INSERT INTO service_borrower_lease_events(company_id,event_id,lease_id,state,expires_at,idle_until,last_used_at,actor,reason,request_id)
VALUES($1,$2,$3,'released',$4,$5,$6,$7,$8,$9)`, binding.scope.company, eventID, leaseID, expiresAt, idleUntil, lastUsedAt, binding.employee, "borrower lease released", requestID); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: leaseID, Status: "released"}, nil
	})
	if err != nil {
		return ServiceBorrowerLeaseRecord{}, err
	}
	return k.ServiceBorrowerLease(ctx, binding.scope, writeReceipt.ID)
}

func (k *Kernel) TXRevokeServiceBorrowerLeases(ctx context.Context, scope Scope, jobID string, generation int, reason, requestID string) (int, error) {
	if k == nil || !core.ValidID(scope.company) || !core.ValidID(jobID) || generation <= 0 || !core.ValidID(requestID) || strings.TrimSpace(reason) == "" || len(reason) > 4096 {
		return 0, core.Malformed
	}
	var revoked int
	writeReceipt, err := k.TXWrite(ctx, scope, nil, requestID, "service.borrower_lease.revoke", struct {
		JobID      string
		Generation int
		Reason     string
	}{jobID, generation, strings.TrimSpace(reason)}, func(tx pgx.Tx) (Receipt, error) {
		rows, err := tx.Query(ctx, `SELECT l.lease_id,e.expires_at,e.idle_until,e.last_used_at
FROM service_borrower_lease_records l
JOIN LATERAL (SELECT state,expires_at,idle_until,last_used_at FROM service_borrower_lease_events WHERE company_id=l.company_id AND lease_id=l.lease_id ORDER BY event_seq DESC LIMIT 1) e ON true
WHERE l.company_id=$1 AND l.job_id=$2 AND l.generation=$3 AND e.state='active' FOR UPDATE`, scope.company, jobID, generation)
		if err != nil {
			return Receipt{}, err
		}
		defer rows.Close()
		type leaseRevocation struct {
			leaseID                          string
			expiresAt, idleUntil, lastUsedAt time.Time
		}
		leases := make([]leaseRevocation, 0)
		for rows.Next() {
			var lease leaseRevocation
			if err = rows.Scan(&lease.leaseID, &lease.expiresAt, &lease.idleUntil, &lease.lastUsedAt); err != nil {
				return Receipt{}, err
			}
			leases = append(leases, lease)
		}
		if err = rows.Err(); err != nil {
			return Receipt{}, err
		}
		rows.Close()
		var now time.Time
		if err = tx.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&now); err != nil {
			return Receipt{}, err
		}
		for _, lease := range leases {
			leaseID, expiresAt, idleUntil := lease.leaseID, lease.expiresAt, lease.idleUntil
			eventID := stableCapabilityID("service-borrower-lease-event", scope.company, requestID, leaseID)
			leaseRequestID := "lease-revoke-" + fingerprint([]string{requestID, leaseID})[:48]
			if _, err = tx.Exec(ctx, `INSERT INTO service_borrower_lease_events(company_id,event_id,lease_id,state,expires_at,idle_until,last_used_at,actor,reason,request_id)
VALUES($1,$2,$3,'revoked',$4,$5,$6,'system',$7,$8)`, scope.company, eventID, leaseID, expiresAt, idleUntil, now, reason, leaseRequestID); err != nil {
				return Receipt{}, err
			}
			revoked++
		}
		return Receipt{ID: jobID, Status: "revoked", Revision: int64(revoked)}, nil
	})
	if err != nil {
		return 0, err
	}
	return int(writeReceipt.Revision), nil
}

func revokeServiceBorrowerLeasesTX(ctx context.Context, tx pgx.Tx, companyID, jobID string, generation int, reason string) (int, error) {
	rows, err := tx.Query(ctx, `SELECT l.lease_id,e.expires_at,e.idle_until,e.last_used_at
FROM service_borrower_lease_records l
JOIN LATERAL (SELECT state,expires_at,idle_until,last_used_at FROM service_borrower_lease_events WHERE company_id=l.company_id AND lease_id=l.lease_id ORDER BY event_seq DESC LIMIT 1) e ON true
WHERE l.company_id=$1 AND l.job_id=$2 AND l.generation=$3 AND e.state='active' FOR UPDATE`, companyID, jobID, generation)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	type leaseRevocation struct {
		leaseID                          string
		expiresAt, idleUntil, lastUsedAt time.Time
	}
	leases := make([]leaseRevocation, 0)
	for rows.Next() {
		var lease leaseRevocation
		if err = rows.Scan(&lease.leaseID, &lease.expiresAt, &lease.idleUntil, &lease.lastUsedAt); err != nil {
			return 0, err
		}
		leases = append(leases, lease)
	}
	if err = rows.Err(); err != nil {
		return 0, err
	}
	rows.Close()
	var now time.Time
	if err = tx.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&now); err != nil {
		return 0, err
	}
	now = now.UTC()
	revoked := 0
	for _, lease := range leases {
		leaseID, expiresAt, idleUntil := lease.leaseID, lease.expiresAt, lease.idleUntil
		eventID := stableCapabilityID("service-borrower-lease-event", companyID, jobID, strconv.Itoa(generation), leaseID, "revoked")
		requestID := "lease-revoke-" + fingerprint([]string{companyID, jobID, strconv.Itoa(generation), leaseID})[:48]
		if _, err = tx.Exec(ctx, `INSERT INTO service_borrower_lease_events(company_id,event_id,lease_id,state,expires_at,idle_until,last_used_at,actor,reason,request_id)
VALUES($1,$2,$3,'revoked',$4,$5,$6,'system',$7,$8)`, companyID, eventID, leaseID, expiresAt, idleUntil, now, reason, requestID); err != nil {
			return 0, err
		}
		revoked++
	}
	return revoked, nil
}

func revokeServiceBorrowerLeasesForSessionTX(ctx context.Context, tx pgx.Tx, companyID, sessionID, reason string) (int, error) {
	rows, err := tx.Query(ctx, `SELECT l.lease_id,e.expires_at,e.idle_until,e.last_used_at
FROM service_borrower_lease_records l
JOIN LATERAL (SELECT state,expires_at,idle_until,last_used_at FROM service_borrower_lease_events WHERE company_id=l.company_id AND lease_id=l.lease_id ORDER BY event_seq DESC LIMIT 1) e ON true
WHERE l.company_id=$1 AND (l.owner_session_id=$2 OR l.borrower_session_id=$2) AND e.state='active' FOR UPDATE`, companyID, sessionID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	type leaseRevocation struct {
		leaseID                          string
		expiresAt, idleUntil, lastUsedAt time.Time
	}
	leases := make([]leaseRevocation, 0)
	for rows.Next() {
		var lease leaseRevocation
		if err = rows.Scan(&lease.leaseID, &lease.expiresAt, &lease.idleUntil, &lease.lastUsedAt); err != nil {
			return 0, err
		}
		leases = append(leases, lease)
	}
	if err = rows.Err(); err != nil {
		return 0, err
	}
	rows.Close()
	var now time.Time
	if err = tx.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&now); err != nil {
		return 0, err
	}
	now = now.UTC()
	revoked := 0
	for _, lease := range leases {
		leaseID, expiresAt, idleUntil := lease.leaseID, lease.expiresAt, lease.idleUntil
		eventID := stableCapabilityID("service-borrower-lease-event", companyID, sessionID, leaseID, "session-revoked")
		requestID := "lease-session-revoke-" + fingerprint([]string{companyID, sessionID, leaseID})[:40]
		if _, err = tx.Exec(ctx, `INSERT INTO service_borrower_lease_events(company_id,event_id,lease_id,state,expires_at,idle_until,last_used_at,actor,reason,request_id)
VALUES($1,$2,$3,'revoked',$4,$5,$6,'system',$7,$8)`, companyID, eventID, leaseID, expiresAt, idleUntil, now, reason, requestID); err != nil {
			return 0, err
		}
		revoked++
	}
	return revoked, nil
}
