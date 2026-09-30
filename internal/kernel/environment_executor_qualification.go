// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
	"polis/internal/environment"
	"polis/internal/intake"
)

type EnvironmentExecutorQualificationInput struct {
	RevisionID            string
	Decision              string
	EvidenceInputID       string
	EvidenceInputRevision int64
	QualifiedUntil        *time.Time
	Rationale             string
	RequestID             string
}

func (k *Kernel) TXRecordEnvironmentExecutorQualification(ctx context.Context, companyID string, input EnvironmentExecutorQualificationInput) (Receipt, error) {
	if !core.ValidID(companyID) || !core.ValidID(input.RevisionID) || !core.ValidID(input.RequestID) || !core.ValidID(input.EvidenceInputID) || input.EvidenceInputRevision <= 0 || (input.Decision != "qualified" && input.Decision != "revoked") || strings.TrimSpace(input.Rationale) == "" || len(input.Rationale) > 512 {
		return Receipt{}, core.Malformed
	}
	if input.Decision == "revoked" && input.QualifiedUntil != nil || input.QualifiedUntil != nil && !input.QualifiedUntil.After(time.Now().UTC()) {
		return Receipt{}, core.Malformed
	}
	var profileID, toolchainSHA256, missionID string
	if err := k.pool.QueryRow(ctx, `SELECT profile_id,toolchain_sha256,COALESCE(mission_id,'') FROM project_environment_revisions WHERE company_id=$1 AND revision_id=$2`, companyID, input.RevisionID).Scan(&profileID, &toolchainSHA256, &missionID); errors.Is(err, pgx.ErrNoRows) {
		return Receipt{}, core.OutOfScope
	} else if err != nil {
		return Receipt{}, err
	}
	isolationProfile, supported := environment.IsolationProfileForNodeProfile(profileID)
	fingerprint, fingerprintAvailable := k.CurrentEnvironmentExecutorFingerprint(profileID)
	if !supported || missionID == "" || !fingerprintAvailable || !validTaskInputDigest(toolchainSHA256) || !validTaskInputDigest(fingerprint.ExecutorSHA256) || !validTaskInputDigest(fingerprint.HostSHA256) || !validTaskInputDigest(fingerprint.IsolationPolicySHA256) {
		return Receipt{}, core.Denied
	}
	var evidenceSHA256, evidenceMediaType, evidenceState string
	var evidenceBytes int64
	if err := k.pool.QueryRow(ctx, `SELECT content_digest,media_type,state,byte_size FROM mission_inputs
WHERE company_id=$1 AND mission_id=$2 AND input_id=$3 AND revision=$4`, companyID, missionID, input.EvidenceInputID, input.EvidenceInputRevision).Scan(&evidenceSHA256, &evidenceMediaType, &evidenceState, &evidenceBytes); errors.Is(err, pgx.ErrNoRows) {
		return Receipt{}, core.OutOfScope
	} else if err != nil {
		return Receipt{}, err
	}
	if !validExecutorEvidence(evidenceState, evidenceMediaType, evidenceSHA256, evidenceBytes) || evidenceMediaType != "application/json" {
		return Receipt{}, core.Denied
	}
	evidence, err := readBlob(k.root, companyID, evidenceSHA256)
	if err != nil || int64(len(evidence)) != evidenceBytes {
		return Receipt{}, core.Integrity
	}
	if _, err = environment.ValidateEnvironmentExecutorQualificationEvidence(evidence, profileID, toolchainSHA256, fingerprint, input.Decision); err != nil {
		return Receipt{}, core.Denied
	}
	eventID := stableCapabilityID("env-executor-qualification", companyID, input.RequestID)
	scope := k.LocalScope(companyID)
	writePayload := struct {
		Input            EnvironmentExecutorQualificationInput
		ProfileID        string
		ToolchainSHA256  string
		IsolationProfile string
		Identity         environment.EnvironmentExecutorFingerprint
		EvidenceSHA256   string
	}{input, profileID, toolchainSHA256, isolationProfile, fingerprint, evidenceSHA256}
	return k.TXWrite(ctx, scope, nil, input.RequestID, "environment.executor.qualification."+input.Decision, writePayload, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureEnvironmentCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		var currentProfileID, currentToolchainSHA256, currentMissionID string
		if err := tx.QueryRow(ctx, `SELECT profile_id,toolchain_sha256,COALESCE(mission_id,'') FROM project_environment_revisions
WHERE company_id=$1 AND revision_id=$2 FOR SHARE`, companyID, input.RevisionID).Scan(&currentProfileID, &currentToolchainSHA256, &currentMissionID); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if currentProfileID != profileID || currentToolchainSHA256 != toolchainSHA256 || currentMissionID != missionID {
			return Receipt{}, core.Conflict
		}
		var evidenceSHA256, evidenceMediaType, evidenceState string
		var evidenceBytes int64
		if err := tx.QueryRow(ctx, `SELECT content_digest,media_type,state,byte_size FROM mission_inputs
WHERE company_id=$1 AND mission_id=$2 AND input_id=$3 AND revision=$4`, companyID, missionID, input.EvidenceInputID, input.EvidenceInputRevision).Scan(&evidenceSHA256, &evidenceMediaType, &evidenceState, &evidenceBytes); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if !validExecutorEvidence(evidenceState, evidenceMediaType, evidenceSHA256, evidenceBytes) || evidenceSHA256 != writePayload.EvidenceSHA256 {
			return Receipt{}, core.Denied
		}
		reportBytes, err := readBlob(k.root, companyID, evidenceSHA256)
		if err != nil || int64(len(reportBytes)) != evidenceBytes {
			return Receipt{}, core.Integrity
		}
		verifiedReport, err := environment.ValidateEnvironmentExecutorQualificationEvidence(reportBytes, profileID, toolchainSHA256, fingerprint, input.Decision)
		if err != nil {
			return Receipt{}, core.Denied
		}
		verifiedEvidenceBytes := int64(0)
		for _, check := range verifiedReport.Checks {
			if check.EvidenceInputID == input.EvidenceInputID && check.EvidenceInputRevision == input.EvidenceInputRevision {
				return Receipt{}, core.Denied
			}
			var checkDigest, checkState string
			var checkBytes int64
			if err := tx.QueryRow(ctx, `SELECT content_digest,state,byte_size FROM mission_inputs
WHERE company_id=$1 AND mission_id=$2 AND input_id=$3 AND revision=$4`, companyID, missionID, check.EvidenceInputID, check.EvidenceInputRevision).Scan(&checkDigest, &checkState, &checkBytes); errors.Is(err, pgx.ErrNoRows) {
				return Receipt{}, core.OutOfScope
			} else if err != nil {
				return Receipt{}, err
			}
			if checkDigest != check.EvidenceSHA256 || (checkState != string(intake.StateUsable) && checkState != string(intake.StatePartial)) || checkBytes <= 0 || checkBytes > intake.MaxUploadBytes {
				return Receipt{}, core.Integrity
			}
			checkEvidence, readErr := readBlob(k.root, companyID, checkDigest)
			if readErr != nil || int64(len(checkEvidence)) != checkBytes {
				return Receipt{}, core.Integrity
			}
			verifiedEvidenceBytes += checkBytes
			if verifiedEvidenceBytes > intake.MaxModelInputSourceBytes {
				return Receipt{}, core.TooLarge
			}
		}
		_, err = tx.Exec(ctx, `INSERT INTO environment_executor_qualification_events
(company_id,event_id,profile_id,executor_fingerprint_sha256,host_fingerprint_sha256,isolation_policy_sha256,toolchain_sha256,isolation_profile,decision,evidence_sha256,evidence_input_id,evidence_input_revision,qualified_until,actor,rationale,request_id)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,'local-owner',$14,$15)`,
			companyID, eventID, profileID, fingerprint.ExecutorSHA256, fingerprint.HostSHA256, fingerprint.IsolationPolicySHA256, toolchainSHA256,
			isolationProfile, input.Decision, evidenceSHA256, input.EvidenceInputID, input.EvidenceInputRevision, input.QualifiedUntil, strings.TrimSpace(input.Rationale), input.RequestID)
		if err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: eventID, Status: input.Decision}, nil
	})
}

func validExecutorEvidence(state, mediaType, digest string, byteSize int64) bool {
	return (state == string(intake.StateUsable) || state == string(intake.StatePartial)) &&
		(mediaType == "text/plain" || mediaType == "text/markdown" || mediaType == "application/json") &&
		byteSize > 0 && byteSize <= 64<<10 && validTaskInputDigest(digest)
}
