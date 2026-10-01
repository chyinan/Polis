// pattern: Imperative Shell
package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

const maxMemoryRecordContentBytes = 32 * 1024

type MemorySourceReference struct {
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	Revision int64  `json:"revision"`
	SHA256   string `json:"sha256"`
}

type MemoryRecordInput struct {
	RecordID    string                 `json:"recordId"`
	Kind        string                 `json:"kind"`
	Scope       string                 `json:"scope"`
	MissionID   string                 `json:"missionId,omitempty"`
	EmployeeID  string                 `json:"employeeId,omitempty"`
	Sensitivity string                 `json:"sensitivity"`
	Content     string                 `json:"content"`
	ObservedAt  time.Time              `json:"observedAt"`
	Source      *MemorySourceReference `json:"source,omitempty"`
}

type MemoryRecordRevision struct {
	CompanyID   string                 `json:"companyId"`
	RecordID    string                 `json:"recordId"`
	Revision    int64                  `json:"revision"`
	Kind        string                 `json:"kind"`
	Scope       string                 `json:"scope"`
	MissionID   string                 `json:"missionId,omitempty"`
	EmployeeID  string                 `json:"employeeId,omitempty"`
	Sensitivity string                 `json:"sensitivity"`
	Content     string                 `json:"content"`
	ContentSHA  string                 `json:"contentSha256"`
	ObservedAt  string                 `json:"observedAt"`
	Source      *MemorySourceReference `json:"source,omitempty"`
	State       string                 `json:"state"`
	CreatedBy   string                 `json:"createdBy"`
	CreatedAt   string                 `json:"createdAt"`
}

type MemoryRevisionReviewInput struct {
	RecordID string `json:"recordId"`
	Revision int64  `json:"revision"`
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}

type MemoryCorrectionInput struct {
	CorrectionID string                `json:"correctionId"`
	RecordID     string                `json:"recordId"`
	BaseRevision int64                 `json:"baseRevision"`
	Content      string                `json:"content"`
	ObservedAt   time.Time             `json:"observedAt"`
	Source       MemorySourceReference `json:"source"`
	Reason       string                `json:"reason"`
}

type MemoryCorrectionReviewInput struct {
	CorrectionID string `json:"correctionId"`
	Decision     string `json:"decision"`
	Reason       string `json:"reason"`
}

type MemoryDependencyInput struct {
	RecordID       string `json:"recordId"`
	RecordRevision int64  `json:"recordRevision"`
	TargetKind     string `json:"targetKind"`
	TargetID       string `json:"targetId"`
	TargetRevision int64  `json:"targetRevision"`
	TargetSHA256   string `json:"targetSha256"`
	RiskLevel      string `json:"riskLevel"`
}

type MemoryDependency struct {
	CompanyID      string `json:"companyId"`
	DependencyID   string `json:"dependencyId"`
	RecordID       string `json:"recordId"`
	RecordRevision int64  `json:"recordRevision"`
	TargetKind     string `json:"targetKind"`
	TargetID       string `json:"targetId"`
	TargetRevision int64  `json:"targetRevision"`
	TargetSHA256   string `json:"targetSha256"`
	RiskLevel      string `json:"riskLevel"`
	CreatedBy      string `json:"createdBy"`
	CreatedAt      string `json:"createdAt"`
}

// TXCreateMemoryRecord stores an immutable, source-pinned first revision in
// proposed state. Ordinary Worker bindings cannot create verified facts.
func (k *Kernel) TXCreateMemoryRecord(ctx context.Context, b Binding, input MemoryRecordInput, key string) (MemoryRecordRevision, error) {
	if ctx == nil || !validMemoryRecordInput(b, input, key) {
		return MemoryRecordRevision{}, core.Malformed
	}
	contentSum := sha256.Sum256([]byte(input.Content))
	contentSHA := hex.EncodeToString(contentSum[:])
	_, err := k.TXWrite(ctx, b.scope, &b, key, "memory.record.proposed", input, func(tx pgx.Tx) (Receipt, error) {
		if err := memoryScopeAllowedTX(ctx, tx, b, input.Scope, input.MissionID, input.EmployeeID); err != nil {
			return Receipt{}, err
		}
		observedAt := input.ObservedAt
		if input.Source != nil {
			sourceObservedAt, err := validateMemorySourceTX(ctx, tx, b.scope, input, *input.Source)
			if err != nil {
				return Receipt{}, err
			}
			if observedAt.IsZero() {
				observedAt = sourceObservedAt
			}
			if (input.Scope == "company" || (input.Scope == "employee" && input.MissionID == "")) && b.employee != "emp-planning" {
				return Receipt{}, core.Denied
			}
		} else if observedAt.IsZero() {
			observedAt = time.Now().UTC()
		}
		if _, err := tx.Exec(ctx, `INSERT INTO memory_records(company_id,record_id,record_kind,scope_kind,mission_id,employee_id,sensitivity,created_by)
VALUES($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,''),$7,$8)`, b.scope.company, input.RecordID, input.Kind, input.Scope,
			input.MissionID, input.EmployeeID, input.Sensitivity, b.employee); err != nil {
			if isUniqueViolation(err) {
				return Receipt{}, core.Conflict
			}
			return Receipt{}, err
		}
		var sourceKind, sourceID, sourceSHA any
		var sourceRevision any
		if input.Source != nil {
			sourceKind, sourceID, sourceSHA, sourceRevision = input.Source.Kind, input.Source.ID, input.Source.SHA256, input.Source.Revision
		}
		_, err := tx.Exec(ctx, `INSERT INTO memory_record_revisions(company_id,record_id,revision,content,content_sha256,source_kind,source_id,source_revision,source_sha256,observed_at,created_by)
VALUES($1,$2,1,$3,$4,$5,$6,$7,$8,$9,$10)`, b.scope.company, input.RecordID, input.Content, contentSHA,
			sourceKind, sourceID, sourceRevision, sourceSHA, observedAt, b.employee)
		if err != nil {
			return Receipt{}, err
		}
		if err = appendEvent(ctx, tx, b.scope, "memory.revision.proposed", map[string]any{
			"record_id": input.RecordID, "revision": 1, "kind": input.Kind, "content_sha256": contentSHA,
			"source_kind": sourceKind, "source_id": sourceID, "source_revision": sourceRevision,
		}); err != nil {
			return Receipt{}, err
		}
		seq, err := currentCompanySequenceTX(ctx, tx, b.scope.company)
		if err != nil {
			return Receipt{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO memory_revision_state_events(company_id,company_seq,record_id,revision,state,actor,reason)
VALUES($1,$2,$3,1,'proposed',$4,'initial proposal')`, b.scope.company, seq, input.RecordID, b.employee); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: input.RecordID, Status: "proposed", Revision: 1}, nil
	})
	if err != nil {
		return MemoryRecordRevision{}, err
	}
	return k.GetMemoryRecordRevision(ctx, b, input.RecordID, 1)
}

// TXReviewMemoryRevision records a separate reviewer decision. The record's
// author cannot approve their own proposal, and the reviewer must be one of the
// fixed Planning or Review employees.
func (k *Kernel) TXReviewMemoryRevision(ctx context.Context, b Binding, input MemoryRevisionReviewInput, key string) (Receipt, error) {
	if ctx == nil || !core.ValidID(input.RecordID) || input.Revision < 1 || !core.ValidID(key) ||
		(b.employee != "emp-planning" && b.employee != "emp-review") ||
		(input.Decision != "verified" && input.Decision != "disputed") || !validMemoryReason(input.Reason) {
		return Receipt{}, core.Malformed
	}
	payload := struct {
		RecordID, Decision, Reason string
		Revision                   int64
	}{input.RecordID, input.Decision, strings.TrimSpace(input.Reason), input.Revision}
	return k.TXWrite(ctx, b.scope, &b, key, "memory.revision.reviewed", payload, func(tx pgx.Tx) (Receipt, error) {
		var author, kind, scopeKind, missionID, employeeID, sensitivity string
		var sourceKind, sourceID, sourceSHA string
		var sourceRevision *int64
		err := tx.QueryRow(ctx, `SELECT r.created_by,r.record_kind,r.scope_kind,COALESCE(r.mission_id,''),COALESCE(r.employee_id,''),r.sensitivity,
COALESCE(v.source_kind,''),COALESCE(v.source_id,''),v.source_revision,COALESCE(v.source_sha256,'')
FROM memory_records r JOIN memory_record_revisions v ON v.company_id=r.company_id AND v.record_id=r.record_id
WHERE r.company_id=$1 AND r.record_id=$2 AND v.revision=$3`, b.scope.company, input.RecordID, input.Revision).Scan(
			&author, &kind, &scopeKind, &missionID, &employeeID, &sensitivity, &sourceKind, &sourceID, &sourceRevision, &sourceSHA,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		}
		if err != nil {
			return Receipt{}, err
		}
		if author == b.employee || !memoryRoleMayReview(kind, b.employee) {
			return Receipt{}, core.Denied
		}
		if b.employee == "emp-review" && sensitivity == "restricted" {
			return Receipt{}, core.Denied
		}
		if err = memoryReviewScopeAllowedTX(ctx, tx, b, scopeKind, missionID, employeeID); err != nil {
			return Receipt{}, err
		}
		var currentState string
		err = tx.QueryRow(ctx, `SELECT state FROM memory_revision_state_events WHERE company_id=$1 AND record_id=$2 AND revision=$3 ORDER BY company_seq DESC LIMIT 1`, b.scope.company, input.RecordID, input.Revision).Scan(&currentState)
		if errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.Integrity
		}
		if err != nil {
			return Receipt{}, err
		}
		if currentState != "proposed" {
			return Receipt{}, core.ConflictError{Reason: "memory revision is no longer awaiting review", CurrentState: currentState}
		}
		if input.Decision == "verified" {
			if sourceKind != "" {
				if sourceRevision == nil {
					return Receipt{}, core.ConflictError{Reason: "memory source changed or is no longer usable", CurrentState: "source_stale"}
				}
				if _, err = validateMemorySourceTX(ctx, tx, b.scope, MemoryRecordInput{MissionID: missionID}, MemorySourceReference{
					Kind: sourceKind, ID: sourceID, Revision: *sourceRevision, SHA256: sourceSHA,
				}); err != nil {
					return Receipt{}, err
				}
			} else if kind == "verified_fact" || kind == "decision" {
				return Receipt{}, core.ConflictError{Reason: "facts and decisions require a durable source reference", CurrentState: "source_missing"}
			}
		}
		if err = appendEvent(ctx, tx, b.scope, "memory.revision."+input.Decision, map[string]any{
			"record_id": input.RecordID, "revision": input.Revision, "actor": b.employee, "reason": payload.Reason,
		}); err != nil {
			return Receipt{}, err
		}
		seq, err := currentCompanySequenceTX(ctx, tx, b.scope.company)
		if err != nil {
			return Receipt{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO memory_revision_state_events(company_id,company_seq,record_id,revision,state,actor,reason)
VALUES($1,$2,$3,$4,$5,$6,$7)`, b.scope.company, seq, input.RecordID, input.Revision, input.Decision, b.employee, payload.Reason); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: input.RecordID, Status: input.Decision, Revision: input.Revision}, nil
	})
}

// TXProposeMemoryCorrection records a source-backed correction request against
// an exact verified revision. Approval is a separate operation; proposals do
// not change the effective revision or any dependent work.
func (k *Kernel) TXProposeMemoryCorrection(ctx context.Context, b Binding, input MemoryCorrectionInput, key string) (Receipt, error) {
	if ctx == nil || !validMemoryCorrectionInput(input, key) {
		return Receipt{}, core.Malformed
	}
	contentSum := sha256.Sum256([]byte(input.Content))
	contentSHA := hex.EncodeToString(contentSum[:])
	payload := struct {
		CorrectionID, RecordID, Content, ContentSHA, Reason string
		BaseRevision                                        int64
		ObservedAt                                          time.Time
		Source                                              MemorySourceReference
	}{input.CorrectionID, input.RecordID, input.Content, contentSHA, strings.TrimSpace(input.Reason), input.BaseRevision, input.ObservedAt, input.Source}
	return k.TXWrite(ctx, b.scope, &b, key, "memory.correction.proposed", payload, func(tx pgx.Tx) (Receipt, error) {
		record, err := memoryRecordAccessTX(ctx, tx, b, input.RecordID, input.BaseRevision)
		if err != nil {
			return Receipt{}, err
		}
		if record.State != "verified" {
			return Receipt{}, core.ConflictError{Reason: "only a verified memory revision can be corrected", CurrentState: record.State}
		}
		var latestRevision int64
		if err = tx.QueryRow(ctx, `SELECT max(revision) FROM memory_record_revisions WHERE company_id=$1 AND record_id=$2`, b.scope.company, input.RecordID).Scan(&latestRevision); err != nil {
			return Receipt{}, err
		}
		if latestRevision != input.BaseRevision {
			return Receipt{}, core.ConflictError{Reason: "memory correction base is no longer current", CurrentState: "stale_revision"}
		}
		observedAt := input.ObservedAt
		sourceObservedAt, err := validateMemorySourceTX(ctx, tx, b.scope, MemoryRecordInput{MissionID: record.MissionID}, input.Source)
		if err != nil {
			return Receipt{}, err
		}
		if observedAt.IsZero() {
			observedAt = sourceObservedAt
		}
		_, err = tx.Exec(ctx, `INSERT INTO memory_correction_requests(company_id,correction_id,record_id,base_revision,proposed_content,content_sha256,source_kind,source_id,source_revision,source_sha256,observed_at,proposer_reason,proposed_by)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, b.scope.company, input.CorrectionID, input.RecordID, input.BaseRevision,
			input.Content, contentSHA, input.Source.Kind, input.Source.ID, input.Source.Revision, input.Source.SHA256, observedAt,
			strings.TrimSpace(input.Reason), b.employee)
		if isUniqueViolation(err) {
			return Receipt{}, core.Conflict
		}
		if err != nil {
			return Receipt{}, err
		}
		if err = appendEvent(ctx, tx, b.scope, "memory.correction.proposed", map[string]any{
			"correction_id": input.CorrectionID, "record_id": input.RecordID, "base_revision": input.BaseRevision,
			"content_sha256": contentSHA, "source_kind": input.Source.Kind, "source_id": input.Source.ID,
			"source_revision": input.Source.Revision, "proposed_by": b.employee, "reason": strings.TrimSpace(input.Reason),
		}); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: input.CorrectionID, Status: "proposed", Revision: input.BaseRevision}, nil
	})
}

// TXReviewMemoryCorrection requires a separate fixed reviewer. Approval
// creates a verified replacement, supersedes the old revision, and invalidates
// every explicit dependency atomically with the review decision.
func (k *Kernel) TXReviewMemoryCorrection(ctx context.Context, b Binding, input MemoryCorrectionReviewInput, key string) (Receipt, error) {
	if ctx == nil || !core.ValidID(input.CorrectionID) || !core.ValidID(key) ||
		(b.employee != "emp-planning" && b.employee != "emp-review") ||
		(input.Decision != "approved" && input.Decision != "rejected") || !validMemoryReason(input.Reason) {
		return Receipt{}, core.Malformed
	}
	payload := struct{ CorrectionID, Decision, Reason string }{input.CorrectionID, input.Decision, strings.TrimSpace(input.Reason)}
	return k.TXWrite(ctx, b.scope, &b, key, "memory.correction.reviewed", payload, func(tx pgx.Tx) (Receipt, error) {
		var recordID, proposer, kind, scopeKind, missionID, employeeID, sensitivity, author string
		var content, contentSHA, sourceKind, sourceID, sourceSHA string
		var baseRevision, sourceRevision int64
		var observedAt time.Time
		err := tx.QueryRow(ctx, `SELECT c.record_id,c.base_revision,c.proposed_content,c.content_sha256,c.source_kind,c.source_id,c.source_revision,c.source_sha256,c.observed_at,c.proposed_by,
r.record_kind,r.scope_kind,COALESCE(r.mission_id,''),COALESCE(r.employee_id,''),r.sensitivity,r.created_by
FROM memory_correction_requests c JOIN memory_records r ON r.company_id=c.company_id AND r.record_id=c.record_id
WHERE c.company_id=$1 AND c.correction_id=$2`, b.scope.company, input.CorrectionID).Scan(
			&recordID, &baseRevision, &content, &contentSHA, &sourceKind, &sourceID, &sourceRevision, &sourceSHA, &observedAt, &proposer,
			&kind, &scopeKind, &missionID, &employeeID, &sensitivity, &author,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		}
		if err != nil {
			return Receipt{}, err
		}
		contentSum := sha256.Sum256([]byte(content))
		if hex.EncodeToString(contentSum[:]) != contentSHA {
			return Receipt{}, core.Integrity
		}
		if proposer == b.employee || author == b.employee || !memoryRoleMayReview(kind, b.employee) {
			return Receipt{}, core.Denied
		}
		if b.employee == "emp-review" && sensitivity == "restricted" {
			return Receipt{}, core.Denied
		}
		if err = memoryReviewScopeAllowedTX(ctx, tx, b, scopeKind, missionID, employeeID); err != nil {
			return Receipt{}, err
		}
		var alreadyReviewed bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM memory_correction_review_events WHERE company_id=$1 AND correction_id=$2)`, b.scope.company, input.CorrectionID).Scan(&alreadyReviewed); err != nil {
			return Receipt{}, err
		}
		if alreadyReviewed {
			return Receipt{}, core.Conflict
		}
		var currentState string
		err = tx.QueryRow(ctx, `SELECT state FROM memory_revision_state_events WHERE company_id=$1 AND record_id=$2 AND revision=$3 ORDER BY company_seq DESC LIMIT 1`, b.scope.company, recordID, baseRevision).Scan(&currentState)
		if errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.Integrity
		}
		if err != nil {
			return Receipt{}, err
		}
		resultRevision := int64(0)
		if input.Decision == "approved" {
			var latestRevision int64
			if err = tx.QueryRow(ctx, `SELECT max(revision) FROM memory_record_revisions WHERE company_id=$1 AND record_id=$2`, b.scope.company, recordID).Scan(&latestRevision); err != nil {
				return Receipt{}, err
			}
			if currentState != "verified" || latestRevision != baseRevision {
				return Receipt{}, core.ConflictError{Reason: "memory correction base is no longer current", CurrentState: "stale_revision"}
			}
			if _, err = validateMemorySourceTX(ctx, tx, b.scope, MemoryRecordInput{MissionID: missionID}, MemorySourceReference{
				Kind: sourceKind, ID: sourceID, Revision: sourceRevision, SHA256: sourceSHA,
			}); err != nil {
				return Receipt{}, err
			}
			resultRevision = latestRevision + 1
			if _, err = tx.Exec(ctx, `INSERT INTO memory_record_revisions(company_id,record_id,revision,content,content_sha256,source_kind,source_id,source_revision,source_sha256,observed_at,created_by)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, b.scope.company, recordID, resultRevision, content, contentSHA,
				sourceKind, sourceID, sourceRevision, sourceSHA, observedAt, b.employee); err != nil {
				return Receipt{}, err
			}
			if err = appendMemoryRevisionStateEventTX(ctx, tx, b.scope, recordID, baseRevision, "superseded", b.employee, strings.TrimSpace(input.Reason)); err != nil {
				return Receipt{}, err
			}
			if err = appendMemoryRevisionStateEventTX(ctx, tx, b.scope, recordID, resultRevision, "verified", b.employee, strings.TrimSpace(input.Reason)); err != nil {
				return Receipt{}, err
			}
		}
		if err = appendEvent(ctx, tx, b.scope, "memory.correction."+input.Decision, map[string]any{
			"correction_id": input.CorrectionID, "record_id": recordID, "base_revision": baseRevision,
			"result_revision": resultRevision, "actor": b.employee, "reason": strings.TrimSpace(input.Reason),
		}); err != nil {
			return Receipt{}, err
		}
		seq, err := currentCompanySequenceTX(ctx, tx, b.scope.company)
		if err != nil {
			return Receipt{}, err
		}
		var resultRevisionValue any
		if resultRevision > 0 {
			resultRevisionValue = resultRevision
		}
		if _, err = tx.Exec(ctx, `INSERT INTO memory_correction_review_events(company_id,company_seq,correction_id,record_id,decision,actor,reason,result_revision)
VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, b.scope.company, seq, input.CorrectionID, recordID, input.Decision, b.employee, strings.TrimSpace(input.Reason), resultRevisionValue); err != nil {
			return Receipt{}, err
		}
		if resultRevision > 0 {
			if err = invalidateMemoryDependenciesTX(ctx, tx, b, recordID, baseRevision, input.CorrectionID, strings.TrimSpace(input.Reason)); err != nil {
				return Receipt{}, err
			}
		}
		return Receipt{ID: input.CorrectionID, Status: input.Decision, Revision: resultRevision}, nil
	})
}

// TXCreateMemoryDependency pins one verified memory revision to an exact
// company-scoped source/target revision and its risk level.
func (k *Kernel) TXCreateMemoryDependency(ctx context.Context, b Binding, input MemoryDependencyInput, key string) (Receipt, error) {
	if ctx == nil || !validMemoryDependencyInput(input, key) {
		return Receipt{}, core.Malformed
	}
	dependencyID := newID()
	return k.TXWrite(ctx, b.scope, &b, key, "memory.dependency.created", input, func(tx pgx.Tx) (Receipt, error) {
		record, err := memoryRecordAccessTX(ctx, tx, b, input.RecordID, input.RecordRevision)
		if err != nil {
			return Receipt{}, err
		}
		if record.State != "verified" {
			return Receipt{}, core.ConflictError{Reason: "only a verified memory revision can be used as a dependency", CurrentState: record.State}
		}
		targetMissionID, err := validateMemoryDependencyTargetTX(ctx, tx, b.scope, input)
		if err != nil {
			return Receipt{}, err
		}
		if record.Scope == "mission" && record.MissionID != targetMissionID {
			return Receipt{}, core.OutOfScope
		}
		if record.Scope == "employee" && record.MissionID != "" && record.MissionID != targetMissionID {
			return Receipt{}, core.OutOfScope
		}
		if err = appendEvent(ctx, tx, b.scope, "memory.dependency.created", map[string]any{
			"dependency_id": dependencyID, "record_id": input.RecordID, "record_revision": input.RecordRevision,
			"target_kind": input.TargetKind, "target_id": input.TargetID, "target_revision": input.TargetRevision,
			"target_sha256": input.TargetSHA256, "risk_level": input.RiskLevel,
		}); err != nil {
			return Receipt{}, err
		}
		seq, err := currentCompanySequenceTX(ctx, tx, b.scope.company)
		if err != nil {
			return Receipt{}, err
		}
		_, err = tx.Exec(ctx, `INSERT INTO memory_dependencies(company_id,dependency_id,record_id,record_revision,target_kind,target_id,target_revision,target_sha256,risk_level,company_seq,created_by)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, b.scope.company, dependencyID, input.RecordID, input.RecordRevision,
			input.TargetKind, input.TargetID, input.TargetRevision, input.TargetSHA256, input.RiskLevel, seq, b.employee)
		if isUniqueViolation(err) {
			return Receipt{}, core.Conflict
		}
		if err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: dependencyID, Status: "active", Revision: input.TargetRevision}, nil
	})
}

func (k *Kernel) GetMemoryRecordRevision(ctx context.Context, b Binding, recordID string, revision int64) (MemoryRecordRevision, error) {
	if ctx == nil || !core.ValidID(recordID) || revision < 1 {
		return MemoryRecordRevision{}, core.Malformed
	}
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return MemoryRecordRevision{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = k.checkSession(ctx, tx, b, false); err != nil {
		return MemoryRecordRevision{}, err
	}
	record, err := memoryRecordAccessTX(ctx, tx, b, recordID, revision)
	if err != nil {
		return MemoryRecordRevision{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return MemoryRecordRevision{}, err
	}
	return record, nil
}

func memoryRecordAccessTX(ctx context.Context, tx pgx.Tx, b Binding, recordID string, revision int64) (MemoryRecordRevision, error) {
	var record MemoryRecordRevision
	var source MemorySourceReference
	var sourceKind, sourceID, sourceSHA, missionID, employeeID string
	var sourceRevision *int64
	err := tx.QueryRow(ctx, `SELECT r.record_id,v.revision,r.record_kind,r.scope_kind,COALESCE(r.mission_id,''),COALESCE(r.employee_id,''),r.sensitivity,
v.content,v.content_sha256,v.observed_at::text,COALESCE(v.source_kind,''),COALESCE(v.source_id,''),v.source_revision,COALESCE(v.source_sha256,''),
s.state,r.created_by,v.created_at::text
FROM memory_records r JOIN memory_record_revisions v ON v.company_id=r.company_id AND v.record_id=r.record_id
JOIN LATERAL (SELECT state FROM memory_revision_state_events e WHERE e.company_id=v.company_id AND e.record_id=v.record_id AND e.revision=v.revision ORDER BY e.company_seq DESC LIMIT 1) s ON true
WHERE r.company_id=$1 AND r.record_id=$2 AND v.revision=$3`, b.scope.company, recordID, revision).Scan(
		&record.RecordID, &record.Revision, &record.Kind, &record.Scope, &missionID, &employeeID, &record.Sensitivity,
		&record.Content, &record.ContentSHA, &record.ObservedAt, &sourceKind, &sourceID, &sourceRevision, &sourceSHA,
		&record.State, &record.CreatedBy, &record.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return MemoryRecordRevision{}, core.OutOfScope
	}
	if err != nil {
		return MemoryRecordRevision{}, err
	}
	allowed, accessErr := memoryReaderAllowedTX(ctx, tx, b, record.Scope, missionID, employeeID, record.Sensitivity, record.CreatedBy, record.State)
	if accessErr != nil {
		return MemoryRecordRevision{}, accessErr
	}
	if !allowed {
		return MemoryRecordRevision{}, core.Denied
	}
	record.CompanyID, record.MissionID, record.EmployeeID = b.scope.company, missionID, employeeID
	if sourceKind != "" {
		source = MemorySourceReference{Kind: sourceKind, ID: sourceID, SHA256: sourceSHA}
		if sourceRevision == nil {
			return MemoryRecordRevision{}, core.Integrity
		}
		source.Revision = *sourceRevision
		record.Source = &source
	}
	sum := sha256.Sum256([]byte(record.Content))
	if hex.EncodeToString(sum[:]) != record.ContentSHA {
		return MemoryRecordRevision{}, core.Integrity
	}
	return record, nil
}

func memoryReaderAllowedTX(ctx context.Context, tx pgx.Tx, b Binding, scopeKind, missionID, employeeID, sensitivity, author, state string) (bool, error) {
	if scopeKind == "employee" && b.employee != employeeID {
		return false, nil
	}
	if scopeKind == "mission" || missionID != "" {
		var taskMissionID string
		if err := tx.QueryRow(ctx, "SELECT mission_id FROM tasks WHERE company_id=$1 AND id=$2", b.scope.company, b.task).Scan(&taskMissionID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return false, nil
			}
			return false, err
		}
		if taskMissionID != missionID {
			return false, nil
		}
	}
	if b.employee == author {
		return true, nil
	}
	if state != "verified" {
		return false, nil
	}
	if scopeKind == "employee" && b.employee == employeeID && sensitivity != "restricted" {
		return true, nil
	}
	return sensitivity == "public" || sensitivity == "internal", nil
}

func memoryScopeAllowedTX(ctx context.Context, tx pgx.Tx, b Binding, scopeKind, missionID, employeeID string) error {
	if scopeKind == "company" {
		if missionID != "" || employeeID != "" {
			return core.Malformed
		}
		return nil
	}
	var taskMissionID string
	if err := tx.QueryRow(ctx, "SELECT mission_id FROM tasks WHERE company_id=$1 AND id=$2", b.scope.company, b.task).Scan(&taskMissionID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return core.OutOfScope
		}
		return err
	}
	if scopeKind == "mission" {
		if !core.ValidID(missionID) || employeeID != "" || taskMissionID != missionID {
			return core.OutOfScope
		}
		return nil
	}
	if scopeKind != "employee" || !core.ValidID(employeeID) || (missionID != "" && (!core.ValidID(missionID) || taskMissionID != missionID)) {
		return core.Malformed
	}
	var exists bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM employees WHERE company_id=$1 AND id=$2)", b.scope.company, employeeID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return core.OutOfScope
	}
	if employeeID != b.employee && b.employee != "emp-planning" {
		return core.Denied
	}
	return nil
}

func memoryReviewScopeAllowedTX(ctx context.Context, tx pgx.Tx, b Binding, scopeKind, missionID, employeeID string) error {
	if scopeKind != "employee" || employeeID == b.employee || b.employee != "emp-review" {
		return memoryScopeAllowedTX(ctx, tx, b, scopeKind, missionID, employeeID)
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM employees WHERE company_id=$1 AND id=$2)`, b.scope.company, employeeID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return core.OutOfScope
	}
	if missionID != "" {
		var taskMissionID string
		if err := tx.QueryRow(ctx, `SELECT mission_id FROM tasks WHERE company_id=$1 AND id=$2`, b.scope.company, b.task).Scan(&taskMissionID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return core.OutOfScope
			}
			return err
		}
		if taskMissionID != missionID {
			return core.OutOfScope
		}
	}
	return nil
}

func appendMemoryRevisionStateEventTX(ctx context.Context, tx pgx.Tx, scope Scope, recordID string, revision int64, state, actor, reason string) error {
	if err := appendEvent(ctx, tx, scope, "memory.revision."+state, map[string]any{
		"record_id": recordID, "revision": revision, "actor": actor, "reason": reason,
	}); err != nil {
		return err
	}
	seq, err := currentCompanySequenceTX(ctx, tx, scope.company)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO memory_revision_state_events(company_id,company_seq,record_id,revision,state,actor,reason)
VALUES($1,$2,$3,$4,$5,$6,$7)`, scope.company, seq, recordID, revision, state, actor, reason)
	return err
}

func invalidateMemoryDependenciesTX(ctx context.Context, tx pgx.Tx, b Binding, recordID string, revision int64, correctionID, reason string) error {
	type dependency struct {
		id, risk string
	}
	rows, err := tx.Query(ctx, `SELECT dependency_id,risk_level FROM memory_dependencies
WHERE company_id=$1 AND record_id=$2 AND record_revision=$3 ORDER BY dependency_id`, b.scope.company, recordID, revision)
	if err != nil {
		return err
	}
	dependencies := make([]dependency, 0)
	for rows.Next() {
		var d dependency
		if err = rows.Scan(&d.id, &d.risk); err != nil {
			rows.Close()
			return err
		}
		dependencies = append(dependencies, d)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, d := range dependencies {
		state := "needs_revalidation"
		if d.risk == "high" || d.risk == "critical" {
			state = "frozen"
		}
		if err = appendEvent(ctx, tx, b.scope, "memory.dependency."+state, map[string]any{
			"dependency_id": d.id, "record_id": recordID, "record_revision": revision,
			"correction_id": correctionID, "reason": reason,
		}); err != nil {
			return err
		}
		seq, seqErr := currentCompanySequenceTX(ctx, tx, b.scope.company)
		if seqErr != nil {
			return seqErr
		}
		if _, err = tx.Exec(ctx, `INSERT INTO memory_dependency_invalidation_events(company_id,company_seq,dependency_id,state,cause_kind,cause_id,actor,reason)
VALUES($1,$2,$3,$4,'memory_correction',$5,$6,$7)`, b.scope.company, seq, d.id, state, correctionID, b.employee, reason); err != nil {
			return err
		}
	}
	return nil
}

func validateMemorySourceTX(ctx context.Context, tx pgx.Tx, scope Scope, input MemoryRecordInput, source MemorySourceReference) (time.Time, error) {
	if !core.ValidID(source.ID) || source.Revision < 1 || !validSHA256(source.SHA256) {
		return time.Time{}, core.Malformed
	}
	var missionID, digest string
	var observedAt time.Time
	switch source.Kind {
	case "mission_input":
		var state string
		err := tx.QueryRow(ctx, `SELECT mission_id,content_digest,state,created_at FROM mission_inputs WHERE company_id=$1 AND input_id=$2 AND revision=$3`, scope.company, source.ID, source.Revision).Scan(&missionID, &digest, &state, &observedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return time.Time{}, core.OutOfScope
		}
		if err != nil {
			return time.Time{}, err
		}
		if state != "usable" && state != "partial" {
			return time.Time{}, core.Denied
		}
	case "artifact":
		if source.Revision != 1 {
			return time.Time{}, core.Malformed
		}
		var state, verdict string
		err := tx.QueryRow(ctx, `SELECT t.mission_id,a.digest,a.state,a.verdict,created.occurred_at
FROM artifacts a JOIN tasks t ON t.company_id=a.company_id AND t.id=a.task_id
JOIN LATERAL (
 SELECT (e.payload->>'occurred_at')::timestamptz AS occurred_at
 FROM events e WHERE e.company_id=a.company_id AND e.kind='artifact.submit' AND e.payload->>'id'=a.id
 ORDER BY e.company_seq DESC LIMIT 1
) created ON true
WHERE a.company_id=$1 AND a.id=$2`, scope.company, source.ID).Scan(&missionID, &digest, &state, &verdict, &observedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return time.Time{}, core.OutOfScope
		}
		if err != nil {
			return time.Time{}, err
		}
		if state != "ready" || verdict == "invalidated" {
			return time.Time{}, core.Denied
		}
	default:
		return time.Time{}, core.Malformed
	}
	if digest != source.SHA256 || (input.MissionID != "" && input.MissionID != missionID) {
		return time.Time{}, core.ConflictError{Reason: "memory source revision or scope changed", CurrentState: "source_stale"}
	}
	return observedAt, nil
}

func validateMemoryDependencyTargetTX(ctx context.Context, tx pgx.Tx, scope Scope, input MemoryDependencyInput) (string, error) {
	var missionID, digest string
	switch input.TargetKind {
	case "mission_input":
		var state string
		err := tx.QueryRow(ctx, `SELECT mission_id,content_digest,state FROM mission_inputs WHERE company_id=$1 AND input_id=$2 AND revision=$3`, scope.company, input.TargetID, input.TargetRevision).Scan(&missionID, &digest, &state)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", core.OutOfScope
		}
		if err != nil {
			return "", err
		}
		if state != "usable" && state != "partial" {
			return "", core.Denied
		}
	case "artifact":
		if input.TargetRevision != 1 {
			return "", core.Malformed
		}
		var state, verdict string
		err := tx.QueryRow(ctx, `SELECT t.mission_id,a.digest,a.state,a.verdict FROM artifacts a JOIN tasks t ON t.company_id=a.company_id AND t.id=a.task_id WHERE a.company_id=$1 AND a.id=$2`, scope.company, input.TargetID).Scan(&missionID, &digest, &state, &verdict)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", core.OutOfScope
		}
		if err != nil {
			return "", err
		}
		if state != "ready" || verdict == "invalidated" {
			return "", core.Denied
		}
	case "contract_revision":
		err := tx.QueryRow(ctx, `SELECT mission_id,digest FROM contract_revisions WHERE company_id=$1 AND id=$2 AND revision=$3`, scope.company, input.TargetID, input.TargetRevision).Scan(&missionID, &digest)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", core.OutOfScope
		}
		if err != nil {
			return "", err
		}
	case "task_revision":
		err := tx.QueryRow(ctx, `SELECT t.mission_id,r.digest FROM task_revisions r JOIN tasks t ON t.company_id=r.company_id AND t.id=r.task_id WHERE r.company_id=$1 AND r.id=$2 AND r.revision=$3`, scope.company, input.TargetID, input.TargetRevision).Scan(&missionID, &digest)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", core.OutOfScope
		}
		if err != nil {
			return "", err
		}
	default:
		return "", core.Malformed
	}
	if digest != input.TargetSHA256 {
		return "", core.ConflictError{Reason: "memory dependency target revision changed", CurrentState: "target_stale"}
	}
	return missionID, nil
}

func currentCompanySequenceTX(ctx context.Context, tx pgx.Tx, companyID string) (int64, error) {
	var seq int64
	err := tx.QueryRow(ctx, "SELECT company_seq FROM companies WHERE id=$1", companyID).Scan(&seq)
	return seq, err
}

func validMemoryRecordInput(b Binding, input MemoryRecordInput, key string) bool {
	if !core.ValidID(input.RecordID) || !core.ValidID(key) || !memoryKindAllowed(input.Kind) ||
		(input.Scope != "company" && input.Scope != "mission" && input.Scope != "employee") ||
		(input.Sensitivity != "public" && input.Sensitivity != "internal" && input.Sensitivity != "confidential" && input.Sensitivity != "restricted") ||
		len(input.Content) == 0 || len(input.Content) > maxMemoryRecordContentBytes || strings.TrimSpace(input.Content) == "" {
		return false
	}
	if input.Source == nil && (input.Kind == "verified_fact" || input.Kind == "decision") {
		return false
	}
	if input.Scope == "company" && (input.MissionID != "" || input.EmployeeID != "") {
		return false
	}
	if input.Scope == "mission" && (!core.ValidID(input.MissionID) || input.EmployeeID != "") {
		return false
	}
	if input.Scope == "employee" && !core.ValidID(input.EmployeeID) {
		return false
	}
	if input.Scope != "employee" && input.EmployeeID != "" {
		return false
	}
	if input.Source != nil && (!core.ValidID(input.Source.ID) || input.Source.Revision < 1 || !validSHA256(input.Source.SHA256) ||
		(input.Source.Kind != "mission_input" && input.Source.Kind != "artifact") || (input.Source.Kind == "artifact" && input.Source.Revision != 1)) {
		return false
	}
	return b.scope.company != ""
}

func validMemoryDependencyInput(input MemoryDependencyInput, key string) bool {
	return core.ValidID(key) && core.ValidID(input.RecordID) && input.RecordRevision > 0 && core.ValidID(input.TargetID) &&
		input.TargetRevision > 0 && validSHA256(input.TargetSHA256) &&
		(input.TargetKind == "mission_input" || input.TargetKind == "artifact" || input.TargetKind == "contract_revision" || input.TargetKind == "task_revision") &&
		(input.RiskLevel == "ordinary" || input.RiskLevel == "high" || input.RiskLevel == "critical")
}

func validMemoryCorrectionInput(input MemoryCorrectionInput, key string) bool {
	return core.ValidID(key) && core.ValidID(input.CorrectionID) && core.ValidID(input.RecordID) && input.BaseRevision > 0 &&
		len(input.Content) > 0 && len(input.Content) <= maxMemoryRecordContentBytes && strings.TrimSpace(input.Content) != "" &&
		validMemoryReason(input.Reason) && core.ValidID(input.Source.ID) && input.Source.Revision > 0 && validSHA256(input.Source.SHA256) &&
		(input.Source.Kind == "mission_input" || (input.Source.Kind == "artifact" && input.Source.Revision == 1))
}

func memoryKindAllowed(kind string) bool {
	switch kind {
	case "verified_fact", "decision", "hypothesis", "temporary_context":
		return true
	default:
		return false
	}
}

func memoryRoleMayReview(kind, employee string) bool {
	if employee == "emp-planning" {
		return true
	}
	if employee == "emp-review" {
		return kind != "role_profile" && kind != "responsibility"
	}
	return false
}

func validMemoryReason(reason string) bool {
	return len(strings.TrimSpace(reason)) > 0 && len(reason) <= 2048
}

func validSHA256(value string) bool {
	if len(value) != 64 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
