// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
	"polis/internal/environment"
)

type ServiceEndpointEventInput struct {
	JobID                string
	Generation           int
	BindAddress          string
	Port                 uint16
	Readiness            string
	SourceRevisionSHA256 string
	HealthcheckSHA256    string
	ProbedAt             time.Time
	LeaseExpiresAt       *time.Time
	RequestID            string
}

type ServiceEndpointRecord struct {
	CompanyID            string
	JobID                string
	Generation           int
	BindAddress          string
	Port                 uint16
	Readiness            string
	SourceRevisionSHA256 string
	HealthcheckSHA256    string
	ProbedAt             time.Time
}

func (k *Kernel) GetLatestServiceEndpointRecord(ctx context.Context, companyID, jobID string) (ServiceEndpointRecord, bool, error) {
	if !core.ValidID(companyID) || !core.ValidID(jobID) {
		return ServiceEndpointRecord{}, false, core.Malformed
	}
	var record ServiceEndpointRecord
	var port int
	err := k.pool.QueryRow(ctx, `SELECT company_id,job_id,generation,bind_address,port,readiness,source_revision_sha256,healthcheck_sha256,created_at
FROM service_endpoint_events WHERE company_id=$1 AND job_id=$2 ORDER BY generation DESC,event_seq DESC LIMIT 1`, companyID, jobID).Scan(
		&record.CompanyID, &record.JobID, &record.Generation, &record.BindAddress, &port, &record.Readiness,
		&record.SourceRevisionSHA256, &record.HealthcheckSHA256, &record.ProbedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ServiceEndpointRecord{}, false, nil
	}
	if err != nil {
		return ServiceEndpointRecord{}, false, err
	}
	if port < 1 || port > 65535 {
		return ServiceEndpointRecord{}, false, core.Integrity
	}
	record.Port = uint16(port)
	return record, true, nil
}

// TXRecordServiceEndpointEvent persists a policy-pinned service observation.
// The caller must first run ProbeServiceEndpoint with a process-owner verifier.
// The event and active JobRun readiness projection commit in one transaction.
func (k *Kernel) TXRecordServiceEndpointEvent(ctx context.Context, companyID string, input ServiceEndpointEventInput) (Receipt, error) {
	if !core.ValidID(companyID) || !core.ValidID(input.JobID) || !core.ValidID(input.RequestID) || input.Generation < 1 ||
		(input.BindAddress != "127.0.0.1" && input.BindAddress != "::1") || input.Port == 0 ||
		!validServiceEndpointReadiness(input.Readiness) || !validEnvironmentSHA256(input.SourceRevisionSHA256) || !validEnvironmentSHA256(input.HealthcheckSHA256) {
		return Receipt{}, core.Malformed
	}
	if input.ProbedAt.IsZero() || input.ProbedAt.After(time.Now().UTC().Add(5*time.Second)) {
		return Receipt{}, core.Malformed
	}
	if input.Readiness == environment.ServiceRevoked {
		if input.LeaseExpiresAt != nil {
			return Receipt{}, core.Malformed
		}
	} else if input.LeaseExpiresAt == nil || !input.LeaseExpiresAt.After(time.Now().UTC()) || !input.LeaseExpiresAt.After(input.ProbedAt) || input.LeaseExpiresAt.Sub(input.ProbedAt) > 10*time.Minute {
		return Receipt{}, core.Malformed
	}

	input.ProbedAt = input.ProbedAt.UTC()
	if input.LeaseExpiresAt != nil {
		leaseExpiry := input.LeaseExpiresAt.UTC()
		input.LeaseExpiresAt = &leaseExpiry
	}
	endpointEventID := stableCapabilityID("service-endpoint", companyID, input.RequestID)
	jobEventID := stableCapabilityID("job-event", companyID, input.RequestID)
	return k.TXWrite(ctx, k.LocalScope(companyID), nil, input.RequestID, "service.endpoint.event", input, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureEnvironmentCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		var kind, serviceID, sourceRevision, environmentRevisionID, storedPolicyDigest string
		var policyManifest []byte
		if err := tx.QueryRow(ctx, `SELECT j.kind,COALESCE(j.service_id,''),j.source_revision_sha256,j.environment_revision_id,r.policy_sha256,r.policy_manifest
FROM job_runs j JOIN project_environment_revisions r ON r.company_id=j.company_id AND r.revision_id=j.environment_revision_id
WHERE j.company_id=$1 AND j.job_id=$2 FOR UPDATE OF j,r`, companyID, input.JobID).Scan(&kind, &serviceID, &sourceRevision, &environmentRevisionID, &storedPolicyDigest, &policyManifest); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if kind != "service" {
			return Receipt{}, core.Denied
		}
		if sourceRevision != input.SourceRevisionSHA256 {
			return Receipt{}, core.Integrity
		}
		policy, _, policyDigest, err := environment.ParseProjectEnvironmentPolicy(policyManifest)
		if err != nil || policyDigest == "" || policyDigest != storedPolicyDigest {
			return Receipt{}, core.Integrity
		}
		policyDecision, decisionDigest, err := latestEnvironmentPolicyDecision(ctx, tx, companyID, environmentRevisionID)
		if err != nil {
			return Receipt{}, err
		}
		if input.Readiness != environment.ServiceRevoked && (policyDecision != "approved" || decisionDigest != policyDigest) {
			return Receipt{}, core.Denied
		}
		probeSpec, pinned := policyServiceProbe(policy.Services, serviceID, input)
		if !pinned {
			return Receipt{}, core.Denied
		}
		if input.Readiness != environment.ServiceRevoked && input.LeaseExpiresAt.Sub(input.ProbedAt) != time.Duration(probeSpec.LeaseDurationMS)*time.Millisecond {
			return Receipt{}, core.Malformed
		}

		previousGeneration, previousAddress, previousPort, previousReadiness, previousSource, previousHealth, previousProbedAt, exists, err := latestServiceEndpointEvent(ctx, tx, companyID, input.JobID)
		if err != nil {
			return Receipt{}, err
		}
		if exists && !input.ProbedAt.After(previousProbedAt) {
			return Receipt{}, core.Conflict
		}
		if !exists {
			if input.Generation != 1 {
				return Receipt{}, core.Conflict
			}
		} else if input.Generation == previousGeneration {
			if input.BindAddress != previousAddress || int(input.Port) != previousPort || input.SourceRevisionSHA256 != previousSource || input.HealthcheckSHA256 != previousHealth || !environment.CanTransitionServiceReadiness(previousReadiness, input.Readiness) {
				return Receipt{}, core.Conflict
			}
		} else if input.Generation != previousGeneration+1 || previousReadiness != environment.ServiceRevoked {
			return Receipt{}, core.Conflict
		}

		jobState, jobReadiness, stdoutOffset, stderrOffset, err := latestJobRunEvent(ctx, tx, companyID, input.JobID)
		if err != nil {
			return Receipt{}, err
		}
		if jobState == "" {
			return Receipt{}, core.Integrity
		}
		if input.Readiness != environment.ServiceRevoked && jobState != string(environment.JobRunning) {
			return Receipt{}, core.Denied
		}
		if jobState == string(environment.JobRunning) {
			jobReadinessValue := input.Readiness
			if jobReadinessValue == environment.ServiceRevoked {
				jobReadinessValue = environment.ServiceUnhealthy
			}
			if jobReadiness != jobReadinessValue {
				if !environment.CanTransitionServiceReadiness(jobReadiness, jobReadinessValue) {
					return Receipt{}, core.Conflict
				}
				if _, err = tx.Exec(ctx, `INSERT INTO job_run_events(company_id,event_id,job_id,state,readiness,reason_code,stdout_offset,stderr_offset)
VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, companyID, jobEventID, input.JobID, jobState, jobReadinessValue, "service_endpoint_"+jobReadinessValue, stdoutOffset, stderrOffset); err != nil {
					return Receipt{}, err
				}
			}
		}

		if _, err = tx.Exec(ctx, `INSERT INTO service_endpoint_events(company_id,event_id,job_id,generation,bind_address,port,readiness,source_revision_sha256,healthcheck_sha256,lease_expires_at,created_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, companyID, endpointEventID, input.JobID, input.Generation, input.BindAddress, int(input.Port), input.Readiness, input.SourceRevisionSHA256, input.HealthcheckSHA256, input.LeaseExpiresAt, input.ProbedAt); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: endpointEventID, Status: input.Readiness}, nil
	})
}

func latestServiceEndpointEvent(ctx context.Context, tx pgx.Tx, companyID, jobID string) (int, string, int, string, string, string, time.Time, bool, error) {
	var generation, port int
	var address, readiness, source, health string
	var probedAt time.Time
	err := tx.QueryRow(ctx, `SELECT generation,bind_address,port,readiness,source_revision_sha256,healthcheck_sha256,created_at
FROM service_endpoint_events WHERE company_id=$1 AND job_id=$2 ORDER BY generation DESC,event_seq DESC LIMIT 1`, companyID, jobID).Scan(&generation, &address, &port, &readiness, &source, &health, &probedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, "", 0, "", "", "", time.Time{}, false, nil
	}
	return generation, address, port, readiness, source, health, probedAt, err == nil, err
}

func policyServiceProbe(services []environment.ProjectServiceDefinition, serviceID string, input ServiceEndpointEventInput) (environment.ServiceProbeSpec, bool) {
	for _, service := range services {
		if service.ID != serviceID {
			continue
		}
		if service.Probe.BindAddress != input.BindAddress || service.Probe.Port != input.Port {
			continue
		}
		digest, err := environment.ServiceProbeSpecSHA256(service.Probe)
		return service.Probe, err == nil && digest == input.HealthcheckSHA256
	}
	return environment.ServiceProbeSpec{}, false
}

func validServiceEndpointReadiness(value string) bool {
	switch value {
	case environment.ServiceNotReady, environment.ServiceReady, environment.ServiceUnhealthy, environment.ServiceRevoked:
		return true
	default:
		return false
	}
}
