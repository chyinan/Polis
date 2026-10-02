// pattern: Imperative Shell
package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

type MemoryRecordRevocationInput struct {
	RecordID              string `json:"recordId"`
	ExpectedRevision      int64  `json:"expectedRevision"`
	ExpectedContentSHA256 string `json:"expectedContentSha256"`
	ReasonCode            string `json:"reasonCode"`
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

// MemoryCorrectionQueueItem is a bounded owner-facing metadata projection.
// Memory text is never returned without an employee-bound read identity;
// command decisions still go through employee-bound Kernel bindings.
type MemoryCorrectionQueueItem struct {
	CorrectionID    string                `json:"correctionId"`
	RecordID        string                `json:"recordId"`
	RecordKind      string                `json:"recordKind"`
	RecordScope     string                `json:"recordScope"`
	MissionID       string                `json:"missionId,omitempty"`
	Sensitivity     string                `json:"sensitivity"`
	BaseRevision    int64                 `json:"baseRevision"`
	CurrentRevision int64                 `json:"currentRevision"`
	CurrentState    string                `json:"currentState"`
	Source          MemorySourceReference `json:"source"`
	ObservedAt      string                `json:"observedAt"`
	ProposedBy      string                `json:"proposedBy"`
	ProposedAt      string                `json:"proposedAt"`
	State           string                `json:"state"`
	ReviewDecision  string                `json:"reviewDecision,omitempty"`
	ReviewActor     string                `json:"reviewActor,omitempty"`
	ReviewSequence  int64                 `json:"reviewSequence,omitempty"`
}

type MemoryCorrectionQueue struct {
	Items     []MemoryCorrectionQueueItem `json:"items"`
	Truncated bool                        `json:"truncated"`
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

type MemoryTaskImpact struct {
	DependencyID             string `json:"dependencyId"`
	RecordID                 string `json:"recordId"`
	RecordRevision           int64  `json:"recordRevision"`
	ReplacementRevision      int64  `json:"replacementRevision,omitempty"`
	ReplacementContent       string `json:"replacementContent,omitempty"`
	ReplacementContentSHA256 string `json:"replacementContentSha256,omitempty"`
	RiskLevel                string `json:"riskLevel"`
	State                    string `json:"state"`
	CorrectionID             string `json:"correctionId"`
	Reason                   string `json:"reason"`
}

type MemoryTaskStatus struct {
	State   string             `json:"state"`
	Impacts []MemoryTaskImpact `json:"impacts"`
}

type MemoryTaskDependencyContext struct {
	DependencyID   string                 `json:"dependencyId"`
	RecordID       string                 `json:"recordId"`
	RecordRevision int64                  `json:"recordRevision"`
	RiskLevel      string                 `json:"riskLevel"`
	TargetKind     string                 `json:"targetKind"`
	TargetID       string                 `json:"targetId"`
	TargetRevision int64                  `json:"targetRevision"`
	TargetSHA256   string                 `json:"targetSha256"`
	Content        string                 `json:"content"`
	ContentSHA256  string                 `json:"contentSha256"`
	ObservedAt     string                 `json:"observedAt"`
	Source         *MemorySourceReference `json:"source,omitempty"`
}

type MemoryTaskRevalidationInput struct {
	TaskID        string `json:"taskId"`
	DependencyID  string `json:"dependencyId"`
	CorrectionID  string `json:"correctionId"`
	ContextSHA256 string `json:"contextSha256"`
	Reason        string `json:"reason"`
}

type MemoryTaskRevalidationPreview struct {
	TaskID                   string                `json:"taskId"`
	TaskState                string                `json:"taskState"`
	TaskGeneration           int64                 `json:"taskGeneration"`
	TaskPlan                 json.RawMessage       `json:"taskPlan"`
	TaskPlanSHA256           string                `json:"taskPlanSha256"`
	MissionState             string                `json:"missionState"`
	DependencyID             string                `json:"dependencyId"`
	DependencyState          string                `json:"dependencyState"`
	CorrectionID             string                `json:"correctionId"`
	CorrectionSource         MemorySourceReference `json:"correctionSource"`
	CorrectionObservedAt     string                `json:"correctionObservedAt"`
	CorrectionProposerReason string                `json:"correctionProposerReason"`
	CorrectionReviewReason   string                `json:"correctionReviewReason"`
	PreviousRecordID         string                `json:"previousRecordId"`
	PreviousRevision         int64                 `json:"previousRevision"`
	ReplacementRevision      int64                 `json:"replacementRevision"`
	ReplacementContent       string                `json:"replacementContent"`
	ReplacementContentSHA256 string                `json:"replacementContentSha256"`
	RiskLevel                string                `json:"riskLevel"`
	TargetKind               string                `json:"targetKind"`
	TargetID                 string                `json:"targetId"`
	TargetRevision           int64                 `json:"targetRevision"`
	TargetSHA256             string                `json:"targetSha256"`
	StoppedSessionID         string                `json:"stoppedSessionId"`
	WorkspaceDigest          string                `json:"workspaceDigest"`
	WorkspaceRevision        int64                 `json:"workspaceRevision"`
	WorkspaceContent         string                `json:"workspaceContent"`
	OtherImpacts             []MemoryTaskImpact    `json:"otherImpacts,omitempty"`
	ContextSHA256            string                `json:"contextSha256"`
}

const (
	maxMemoryHandoverDependencies  = 16
	maxMemoryHandoverContentBytes  = 128 * 1024
	maxMemoryRevalidationPlanBytes = 64 * 1024
)

// TXCreateMemoryRecord stores an immutable, source-pinned first revision in
// proposed state. Ordinary Worker bindings cannot create verified facts.
func (k *Kernel) TXCreateMemoryRecord(ctx context.Context, b Binding, input MemoryRecordInput, key string) (MemoryRecordRevision, error) {
	if ctx == nil || !validMemoryRecordInput(b, input, key) {
		return MemoryRecordRevision{}, core.Malformed
	}
	contentSum := sha256.Sum256([]byte(input.Content))
	contentSHA := hex.EncodeToString(contentSum[:])
	_, err := k.TXWrite(ctx, b.scope, &b, key, "memory.record.proposed", input, func(tx pgx.Tx) (Receipt, error) {
		revoked, revokeErr := k.memoryRecordRevokedTX(ctx, tx, b.scope.company, input.RecordID)
		if revokeErr != nil {
			return Receipt{}, revokeErr
		}
		if revoked {
			return Receipt{}, core.Denied
		}
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
		if input.Source != nil {
			if err = appendMemoryCASRetentionPinTX(ctx, tx, b.scope, "memory_revision_source", input.RecordID, 1,
				input.Source.Kind, input.Source.ID, input.Source.Revision, input.Source.SHA256); err != nil {
				return Receipt{}, err
			}
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
		revoked, revokeErr := k.memoryRecordRevokedTX(ctx, tx, b.scope.company, input.RecordID)
		if revokeErr != nil {
			return Receipt{}, revokeErr
		}
		if revoked {
			return Receipt{}, core.Denied
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
		record, err := memoryRecordAccessTX(k, ctx, tx, b, input.RecordID, input.BaseRevision)
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
		if err = appendMemoryCASRetentionPinTX(ctx, tx, b.scope, "memory_correction_source", input.CorrectionID, 1,
			input.Source.Kind, input.Source.ID, input.Source.Revision, input.Source.SHA256); err != nil {
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
		revoked, revokeErr := k.memoryRecordRevokedTX(ctx, tx, b.scope.company, recordID)
		if revokeErr != nil {
			return Receipt{}, revokeErr
		}
		if revoked {
			return Receipt{}, core.Denied
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
			if err = appendMemoryCASRetentionPinTX(ctx, tx, b.scope, "memory_revision_source", recordID, resultRevision,
				sourceKind, sourceID, sourceRevision, sourceSHA); err != nil {
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

// ListMemoryCorrections returns the newest correction requests for a company.
// It is an owner-facing no-store projection, not a substitute for the
// employee-bound proposal and review commands.
func (k *Kernel) ListMemoryCorrections(ctx context.Context, scope Scope, limit int) (MemoryCorrectionQueue, error) {
	if k == nil || ctx == nil || !core.ValidID(scope.company) || limit < 1 || limit > 100 {
		return MemoryCorrectionQueue{}, core.Malformed
	}
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return MemoryCorrectionQueue{}, err
	}
	defer tx.Rollback(ctx)
	if err = k.checkRuntimeLease(ctx, tx); err != nil {
		return MemoryCorrectionQueue{}, err
	}
	rows, err := tx.Query(ctx, `SELECT c.correction_id,c.record_id,r.record_kind,r.scope_kind,COALESCE(r.mission_id,''),r.sensitivity,
c.base_revision,c.proposed_content,c.content_sha256,b.content,b.content_sha256,c.source_kind,c.source_id,c.source_revision,c.source_sha256,
c.observed_at::text,c.proposer_reason,c.proposed_by,c.created_at::text,
cur.revision,cur.state,COALESCE(rv.decision,''),COALESCE(rv.actor,''),COALESCE(rv.reason,''),COALESCE(rv.company_seq,0)
FROM memory_correction_requests c
JOIN memory_records r ON r.company_id=c.company_id AND r.record_id=c.record_id
JOIN memory_record_revisions b ON b.company_id=c.company_id AND b.record_id=c.record_id AND b.revision=c.base_revision
JOIN LATERAL (SELECT v.revision,e.state FROM memory_record_revisions v
 JOIN LATERAL (SELECT state FROM memory_revision_state_events se WHERE se.company_id=v.company_id AND se.record_id=v.record_id AND se.revision=v.revision ORDER BY se.company_seq DESC LIMIT 1) e ON true
 WHERE v.company_id=c.company_id AND v.record_id=c.record_id ORDER BY v.revision DESC LIMIT 1) cur ON true
LEFT JOIN LATERAL (SELECT decision,actor,reason,company_seq FROM memory_correction_review_events re
 WHERE re.company_id=c.company_id AND re.correction_id=c.correction_id LIMIT 1) rv ON true
WHERE c.company_id=$1 ORDER BY c.created_at DESC,c.correction_id DESC LIMIT $2`, scope.company, limit+1)
	if err != nil {
		return MemoryCorrectionQueue{}, err
	}
	defer rows.Close()
	result := MemoryCorrectionQueue{Items: make([]MemoryCorrectionQueueItem, 0, limit)}
	for rows.Next() {
		var item MemoryCorrectionQueueItem
		var proposedContent, proposedSHA, baseContent, baseSHA string
		var proposerReason, reviewReason string
		var sourceKind, sourceID, sourceSHA string
		var sourceRevision int64
		var reviewSequence int64
		if err = rows.Scan(&item.CorrectionID, &item.RecordID, &item.RecordKind, &item.RecordScope, &item.MissionID, &item.Sensitivity,
			&item.BaseRevision, &proposedContent, &proposedSHA, &baseContent, &baseSHA, &sourceKind, &sourceID, &sourceRevision, &sourceSHA,
			&item.ObservedAt, &proposerReason, &item.ProposedBy, &item.ProposedAt, &item.CurrentRevision, &item.CurrentState,
			&item.ReviewDecision, &item.ReviewActor, &reviewReason, &reviewSequence); err != nil {
			return MemoryCorrectionQueue{}, err
		}
		if len(result.Items) == limit {
			result.Truncated = true
			break
		}
		proposedSum := sha256.Sum256([]byte(proposedContent))
		baseSum := sha256.Sum256([]byte(baseContent))
		if hex.EncodeToString(proposedSum[:]) != proposedSHA || hex.EncodeToString(baseSum[:]) != baseSHA {
			return MemoryCorrectionQueue{}, core.Integrity
		}
		item.Source = MemorySourceReference{Kind: sourceKind, ID: sourceID, Revision: sourceRevision, SHA256: sourceSHA}
		item.ReviewSequence = reviewSequence
		revoked, revokeErr := k.memoryRecordRevokedTX(ctx, tx, scope.company, item.RecordID)
		if revokeErr != nil {
			return MemoryCorrectionQueue{}, revokeErr
		}
		switch {
		case revoked:
			item.State = "revoked"
		case item.ReviewDecision != "":
			item.State = item.ReviewDecision
		case item.CurrentRevision != item.BaseRevision || item.CurrentState != "verified":
			item.State = "stale"
		default:
			item.State = "proposed"
		}
		result.Items = append(result.Items, item)
	}
	if err = rows.Err(); err != nil {
		return MemoryCorrectionQueue{}, err
	}
	// A revocation may publish its durable overlay while this snapshot is being
	// read. Recheck immediately before returning the queue state.
	for index := range result.Items {
		if _, revoked := k.memoryRecordRevocation(scope.company, result.Items[index].RecordID); revoked {
			result.Items[index].State = "revoked"
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return MemoryCorrectionQueue{}, err
	}
	return result, nil
}

// TXCreateMemoryDependency pins one verified memory revision to an exact
// company-scoped source/target revision and its risk level.
func (k *Kernel) TXCreateMemoryDependency(ctx context.Context, b Binding, input MemoryDependencyInput, key string) (Receipt, error) {
	if ctx == nil || !validMemoryDependencyInput(input, key) {
		return Receipt{}, core.Malformed
	}
	if b.session == "" || b.task == "" {
		return Receipt{}, core.Denied
	}
	dependencyID := newID()
	return k.TXWrite(ctx, b.scope, &b, key, "memory.dependency.created", input, func(tx pgx.Tx) (Receipt, error) {
		record, err := memoryRecordAccessTX(k, ctx, tx, b, input.RecordID, input.RecordRevision)
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
			"target_sha256": input.TargetSHA256, "risk_level": input.RiskLevel, "task_id": b.task, "session_id": b.session,
		}); err != nil {
			return Receipt{}, err
		}
		seq, err := currentCompanySequenceTX(ctx, tx, b.scope.company)
		if err != nil {
			return Receipt{}, err
		}
		_, err = tx.Exec(ctx, `INSERT INTO memory_dependencies(company_id,dependency_id,record_id,record_revision,target_kind,target_id,target_revision,target_sha256,risk_level,company_seq,created_by,bound_task_id,worker_session_id)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, b.scope.company, dependencyID, input.RecordID, input.RecordRevision,
			input.TargetKind, input.TargetID, input.TargetRevision, input.TargetSHA256, input.RiskLevel, seq, b.employee, b.task, b.session)
		if isUniqueViolation(err) {
			return Receipt{}, core.Conflict
		}
		if err != nil {
			return Receipt{}, err
		}
		if input.TargetKind == "mission_input" || input.TargetKind == "artifact" {
			if err = appendMemoryCASRetentionPinTX(ctx, tx, b.scope, "memory_dependency_target", dependencyID, 1,
				input.TargetKind, input.TargetID, input.TargetRevision, input.TargetSHA256); err != nil {
				return Receipt{}, err
			}
		}
		return Receipt{ID: dependencyID, Status: "active", Revision: input.TargetRevision}, nil
	})
}

func (k *Kernel) memoryTaskStatusTX(ctx context.Context, tx pgx.Tx, companyID, taskID string) (MemoryTaskStatus, error) {
	status := MemoryTaskStatus{State: "clear", Impacts: []MemoryTaskImpact{}}
	rows, err := tx.Query(ctx, `SELECT latest.dependency_id,d.record_id,d.record_revision,d.risk_level,latest.state,latest.cause_id,latest.reason,
COALESCE(review.result_revision,0)
FROM (
 SELECT DISTINCT ON(dependency_id) dependency_id,state,cause_id,reason,company_seq
 FROM memory_task_state_events WHERE company_id=$1 AND task_id=$2
 ORDER BY dependency_id,company_seq DESC
) latest
JOIN memory_dependencies d ON d.company_id=$1 AND d.dependency_id=latest.dependency_id
LEFT JOIN memory_correction_review_events review ON review.company_id=$1 AND review.correction_id=latest.cause_id AND review.decision='approved'
WHERE latest.state IN ('dirty','frozen') ORDER BY latest.dependency_id`, companyID, taskID)
	if err != nil {
		return status, err
	}
	for rows.Next() {
		var impact MemoryTaskImpact
		if err = rows.Scan(&impact.DependencyID, &impact.RecordID, &impact.RecordRevision, &impact.RiskLevel,
			&impact.State, &impact.CorrectionID, &impact.Reason, &impact.ReplacementRevision); err != nil {
			return status, err
		}
		status.Impacts = append(status.Impacts, impact)
		if impact.State == "frozen" {
			status.State = "frozen"
		} else if status.State == "clear" {
			status.State = "dirty"
		}
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return status, err
	}
	rows.Close()
	revokedRows, err := tx.Query(ctx, `SELECT d.dependency_id,d.record_id,d.record_revision,d.risk_level,COALESCE(o.operation_id,'')
FROM memory_dependencies d LEFT JOIN memory_record_revocation_overlays o
 ON o.company_id=d.company_id AND o.record_id=d.record_id
WHERE d.company_id=$1 AND d.bound_task_id=$2 ORDER BY d.dependency_id`, companyID, taskID)
	if err != nil {
		return status, err
	}
	for revokedRows.Next() {
		var impact MemoryTaskImpact
		var operationID string
		if err = revokedRows.Scan(&impact.DependencyID, &impact.RecordID, &impact.RecordRevision, &impact.RiskLevel, &operationID); err != nil {
			revokedRows.Close()
			return status, err
		}
		if overlay, revoked := k.memoryRecordRevocation(companyID, impact.RecordID); revoked {
			operationID = overlay.OperationID
		}
		if operationID == "" {
			continue
		}
		impact.CorrectionID, impact.State = operationID, "frozen"
		impact.Reason = "memory record revoked; establish a new dependency before resuming work"
		found := false
		for index := range status.Impacts {
			if status.Impacts[index].DependencyID == impact.DependencyID {
				status.Impacts[index] = impact
				found = true
				break
			}
		}
		if !found {
			status.Impacts = append(status.Impacts, impact)
		}
		status.State = "frozen"
	}
	if err = revokedRows.Err(); err != nil {
		revokedRows.Close()
		return status, err
	}
	revokedRows.Close()
	return status, nil
}

// GetMemoryTaskStatus returns the durable dirty/frozen impacts recorded for a Task.
func (k *Kernel) GetMemoryTaskStatus(ctx context.Context, scope Scope, taskID string) (MemoryTaskStatus, error) {
	if ctx == nil || !core.ValidID(taskID) {
		return MemoryTaskStatus{}, core.Malformed
	}
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return MemoryTaskStatus{}, err
	}
	defer tx.Rollback(ctx)
	if err = k.checkRuntimeLease(ctx, tx); err != nil {
		return MemoryTaskStatus{}, err
	}
	var exists bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM tasks WHERE company_id=$1 AND id=$2)", scope.company, taskID).Scan(&exists); err != nil {
		return MemoryTaskStatus{}, err
	}
	if !exists {
		return MemoryTaskStatus{}, core.OutOfScope
	}
	status, err := k.memoryTaskStatusTX(ctx, tx, scope.company, taskID)
	if err != nil {
		return MemoryTaskStatus{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return MemoryTaskStatus{}, err
	}
	return status, nil
}

func (k *Kernel) requireMemoryTaskWritableTX(ctx context.Context, tx pgx.Tx, companyID, taskID string) error {
	status, err := k.memoryTaskStatusTX(ctx, tx, companyID, taskID)
	if err != nil {
		return err
	}
	if status.State == "frozen" {
		return core.ConflictError{Reason: "a high-risk memory dependency froze this Task", CurrentState: status.State}
	}
	return nil
}

func (k *Kernel) requireMemoryTaskCleanTX(ctx context.Context, tx pgx.Tx, companyID, taskID string) error {
	status, err := k.memoryTaskStatusTX(ctx, tx, companyID, taskID)
	if err != nil {
		return err
	}
	if status.State != "clear" {
		return core.ConflictError{Reason: "memory dependencies must be revalidated before Task finalization", CurrentState: status.State}
	}
	return nil
}

func (k *Kernel) memoryTaskImpactContextTX(ctx context.Context, tx pgx.Tx, b Binding, status MemoryTaskStatus) (MemoryTaskStatus, error) {
	for i := range status.Impacts {
		if status.Impacts[i].ReplacementRevision < 1 {
			continue
		}
		record, err := memoryRecordAccessTX(k, ctx, tx, b, status.Impacts[i].RecordID, status.Impacts[i].ReplacementRevision)
		if err != nil {
			return MemoryTaskStatus{}, err
		}
		status.Impacts[i].ReplacementContent = record.Content
		status.Impacts[i].ReplacementContentSHA256 = record.ContentSHA
	}
	return status, nil
}

func (k *Kernel) memoryTaskDependenciesTX(ctx context.Context, tx pgx.Tx, b Binding) ([]MemoryTaskDependencyContext, bool, error) {
	rows, err := tx.Query(ctx, `SELECT d.dependency_id,d.record_id,d.record_revision,d.risk_level,
d.target_kind,d.target_id,d.target_revision,d.target_sha256
FROM memory_dependencies d
JOIN memory_records r ON r.company_id=d.company_id AND r.record_id=d.record_id
JOIN LATERAL (SELECT state FROM memory_revision_state_events e
 WHERE e.company_id=d.company_id AND e.record_id=d.record_id AND e.revision=d.record_revision
 ORDER BY e.company_seq DESC LIMIT 1) rs ON rs.state='verified'
LEFT JOIN LATERAL (SELECT state FROM memory_dependency_invalidation_events e
 WHERE e.company_id=d.company_id AND e.dependency_id=d.dependency_id
 ORDER BY e.company_seq DESC LIMIT 1) inv ON true
WHERE d.company_id=$1 AND d.bound_task_id=$2 AND inv.state IS NULL
 AND d.record_revision=(SELECT max(current_revision.revision) FROM memory_record_revisions current_revision
  WHERE current_revision.company_id=d.company_id AND current_revision.record_id=d.record_id)
ORDER BY d.company_seq,d.dependency_id LIMIT $3`, b.scope.company, b.task, maxMemoryHandoverDependencies+1)
	if err != nil {
		return nil, false, err
	}
	type dependencyRef struct {
		id, recordID, risk, targetKind, targetID, targetSHA string
		revision, targetRevision                            int64
	}
	refs := make([]dependencyRef, 0)
	for rows.Next() {
		var ref dependencyRef
		if err = rows.Scan(&ref.id, &ref.recordID, &ref.revision, &ref.risk, &ref.targetKind, &ref.targetID, &ref.targetRevision, &ref.targetSHA); err != nil {
			rows.Close()
			return nil, false, err
		}
		refs = append(refs, ref)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, false, err
	}
	rows.Close()
	truncated := len(refs) > maxMemoryHandoverDependencies
	if truncated {
		return nil, true, core.ConflictError{Reason: "Task memory dependencies exceed the bounded Handover context", CurrentState: "memory_context_truncated"}
	}
	contexts := make([]MemoryTaskDependencyContext, 0, len(refs))
	contentBytes := 0
	for _, ref := range refs {
		record, accessErr := memoryRecordAccessTX(k, ctx, tx, b, ref.recordID, ref.revision)
		if accessErr != nil {
			return nil, false, accessErr
		}
		if contentBytes+len(record.Content) > maxMemoryHandoverContentBytes {
			return nil, true, core.ConflictError{Reason: "Task memory content exceeds the bounded Handover context", CurrentState: "memory_context_truncated"}
		}
		contentBytes += len(record.Content)
		contexts = append(contexts, MemoryTaskDependencyContext{
			DependencyID: ref.id, RecordID: ref.recordID, RecordRevision: ref.revision, RiskLevel: ref.risk,
			TargetKind: ref.targetKind, TargetID: ref.targetID, TargetRevision: ref.targetRevision, TargetSHA256: ref.targetSHA,
			Content: record.Content, ContentSHA256: record.ContentSHA, ObservedAt: record.ObservedAt, Source: record.Source,
		})
	}
	return contexts, truncated, nil
}

func (k *Kernel) memoryTaskRevalidationPreviewTX(ctx context.Context, tx pgx.Tx, scope Scope, taskID, dependencyID, correctionID string) (MemoryTaskRevalidationPreview, error) {
	var preview MemoryTaskRevalidationPreview
	preview.TaskID, preview.DependencyID, preview.CorrectionID = taskID, dependencyID, correctionID
	var companyState string
	var taskPlan string
	if err := tx.QueryRow(ctx, `SELECT t.state,t.generation,COALESCE(t.plan::text,'null'),ws.digest,ws.revision,m.state,c.state
FROM tasks t JOIN worker_workspaces ws ON ws.company_id=t.company_id AND ws.task_id=t.id
JOIN missions m ON m.company_id=t.company_id AND m.id=t.mission_id
JOIN companies c ON c.id=t.company_id
WHERE t.company_id=$1 AND t.id=$2`, scope.company, taskID).Scan(&preview.TaskState, &preview.TaskGeneration, &taskPlan, &preview.WorkspaceDigest, &preview.WorkspaceRevision, &preview.MissionState, &companyState); errors.Is(err, pgx.ErrNoRows) {
		return preview, core.OutOfScope
	} else if err != nil {
		return preview, err
	}
	if companyState != "active" || (preview.MissionState != "active" && preview.MissionState != "paused") {
		return preview, core.ConflictError{Reason: "memory revalidation requires an active Company and a nonterminal Mission", CurrentState: preview.MissionState}
	}
	planSum := sha256.Sum256([]byte(taskPlan))
	preview.TaskPlan = json.RawMessage(taskPlan)
	if len(preview.TaskPlan) > maxMemoryRevalidationPlanBytes {
		return preview, core.TooLarge
	}
	preview.TaskPlanSHA256 = hex.EncodeToString(planSum[:])
	if preview.TaskState != "ready" && preview.TaskState != "working" {
		return preview, core.ConflictError{Reason: "only an unfinished Task can be revalidated", CurrentState: preview.TaskState}
	}
	var createdBy, sessionID, sessionState string
	err := tx.QueryRow(ctx, `SELECT d.record_id,d.record_revision,d.target_kind,d.target_id,d.target_revision,d.target_sha256,d.risk_level,d.created_by,
COALESCE(d.worker_session_id,'')
FROM memory_dependencies d WHERE d.company_id=$1 AND d.dependency_id=$2 AND d.bound_task_id=$3`, scope.company, dependencyID, taskID).Scan(
		&preview.PreviousRecordID, &preview.PreviousRevision, &preview.TargetKind, &preview.TargetID, &preview.TargetRevision,
		&preview.TargetSHA256, &preview.RiskLevel, &createdBy, &sessionID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return preview, core.OutOfScope
	}
	if err != nil {
		return preview, err
	}
	revoked, err := k.memoryRecordRevokedTX(ctx, tx, scope.company, preview.PreviousRecordID)
	if err != nil {
		return preview, err
	}
	if revoked {
		return preview, core.Denied
	}
	status, err := k.memoryTaskStatusTX(ctx, tx, scope.company, taskID)
	if err != nil {
		return preview, err
	}
	impactFound := false
	for _, impact := range status.Impacts {
		if impact.DependencyID == dependencyID && impact.CorrectionID == correctionID {
			preview.DependencyState = impact.State
			impactFound = true
		} else {
			preview.OtherImpacts = append(preview.OtherImpacts, impact)
		}
	}
	if len(status.Impacts) > maxMemoryHandoverDependencies {
		return preview, core.TooLarge
	}
	if !impactFound {
		return preview, core.ConflictError{Reason: "this dependency has no current invalidation requiring revalidation", CurrentState: status.State}
	}
	if sessionID == "" {
		return preview, core.Integrity
	}
	preview.StoppedSessionID = sessionID
	if err = tx.QueryRow(ctx, "SELECT state FROM worker_sessions WHERE company_id=$1 AND id=$2 AND task_id=$3", scope.company, sessionID, taskID).Scan(&sessionState); errors.Is(err, pgx.ErrNoRows) {
		return preview, core.Integrity
	}
	if err != nil {
		return preview, err
	}
	if sessionState != "stopped" {
		return preview, core.ConflictError{Reason: "the WorkerSession that consumed this memory must be stopped", CurrentState: sessionState}
	}
	var liveSession bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM worker_sessions WHERE company_id=$1 AND task_id=$2 AND state!='stopped')", scope.company, taskID).Scan(&liveSession); err != nil {
		return preview, err
	}
	if liveSession {
		return preview, core.ConflictError{Reason: "all Task WorkerSessions must be stopped before clean-context revalidation", CurrentState: "worker_active"}
	}
	var baseRevision, sourceRevision int64
	var sourceKind, sourceID, sourceSHA, recordMissionID string
	err = tx.QueryRow(ctx, `SELECT c.base_revision,review.result_revision,v.content,v.content_sha256,
c.source_kind,c.source_id,c.source_revision,c.source_sha256,c.observed_at::text,c.proposer_reason,review.reason,r.mission_id
FROM memory_correction_requests c
JOIN memory_correction_review_events review ON review.company_id=c.company_id AND review.correction_id=c.correction_id AND review.decision='approved'
JOIN memory_record_revisions v ON v.company_id=c.company_id AND v.record_id=c.record_id AND v.revision=review.result_revision
JOIN memory_records r ON r.company_id=c.company_id AND r.record_id=c.record_id
WHERE c.company_id=$1 AND c.correction_id=$2 AND c.record_id=$3`, scope.company, correctionID, preview.PreviousRecordID).Scan(
		&baseRevision, &preview.ReplacementRevision, &preview.ReplacementContent, &preview.ReplacementContentSHA256,
		&sourceKind, &sourceID, &sourceRevision, &sourceSHA, &preview.CorrectionObservedAt, &preview.CorrectionProposerReason,
		&preview.CorrectionReviewReason, &recordMissionID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return preview, core.OutOfScope
	}
	if err != nil {
		return preview, err
	}
	if baseRevision != preview.PreviousRevision {
		return preview, core.Integrity
	}
	preview.CorrectionSource = MemorySourceReference{Kind: sourceKind, ID: sourceID, Revision: sourceRevision, SHA256: sourceSHA}
	if _, err = validateMemorySourceTX(ctx, tx, scope, MemoryRecordInput{MissionID: recordMissionID}, preview.CorrectionSource); err != nil {
		return preview, err
	}
	var currentRevision int64
	var replacementState string
	if err = tx.QueryRow(ctx, `SELECT v.revision,latest.state
FROM memory_record_revisions v
JOIN LATERAL (SELECT state FROM memory_revision_state_events state
 WHERE state.company_id=v.company_id AND state.record_id=v.record_id AND state.revision=v.revision
 ORDER BY state.company_seq DESC LIMIT 1) latest ON true
WHERE v.company_id=$1 AND v.record_id=$2 ORDER BY v.revision DESC LIMIT 1`, scope.company, preview.PreviousRecordID).Scan(&currentRevision, &replacementState); err != nil {
		return preview, err
	}
	if currentRevision != preview.ReplacementRevision || replacementState != "verified" {
		return preview, core.ConflictError{Reason: "the approved correction is no longer the current verified memory revision", CurrentState: "stale_correction"}
	}
	sum := sha256.Sum256([]byte(preview.ReplacementContent))
	if hex.EncodeToString(sum[:]) != preview.ReplacementContentSHA256 {
		return preview, core.Integrity
	}
	preview.ContextSHA256 = fingerprint(struct {
		TaskID, TaskState, TaskPlanSHA256, MissionState, WorkspaceDigest       string
		TaskGeneration, WorkspaceRevision                                      int64
		DependencyID, DependencyState                                          string
		CorrectionID, RecordID                                                 string
		PreviousRevision, ReplacementRevision                                  int64
		ReplacementContentSHA256                                               string
		CorrectionSource                                                       MemorySourceReference
		CorrectionObservedAt, CorrectionProposerReason, CorrectionReviewReason string
		RiskLevel, TargetKind, TargetID                                        string
		TargetRevision                                                         int64
		TargetSHA256, StoppedSessionID                                         string
		OtherImpacts                                                           []MemoryTaskImpact
	}{preview.TaskID, preview.TaskState, preview.TaskPlanSHA256, preview.MissionState, preview.WorkspaceDigest,
		preview.TaskGeneration, preview.WorkspaceRevision,
		preview.DependencyID, preview.DependencyState, preview.CorrectionID, preview.PreviousRecordID,
		preview.PreviousRevision, preview.ReplacementRevision, preview.ReplacementContentSHA256,
		preview.CorrectionSource, preview.CorrectionObservedAt, preview.CorrectionProposerReason, preview.CorrectionReviewReason,
		preview.RiskLevel, preview.TargetKind, preview.TargetID, preview.TargetRevision, preview.TargetSHA256,
		preview.StoppedSessionID, preview.OtherImpacts})
	return preview, nil
}

// GetMemoryTaskRevalidationPreview returns the exact, digest-bound context an
// operator must review before clearing one Task memory impact.
func (k *Kernel) GetMemoryTaskRevalidationPreview(ctx context.Context, scope Scope, taskID, dependencyID, correctionID string) (MemoryTaskRevalidationPreview, error) {
	if ctx == nil || !core.ValidID(taskID) || !core.ValidID(dependencyID) || !core.ValidID(correctionID) {
		return MemoryTaskRevalidationPreview{}, core.Malformed
	}
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return MemoryTaskRevalidationPreview{}, err
	}
	defer tx.Rollback(ctx)
	if err = k.checkRuntimeLease(ctx, tx); err != nil {
		return MemoryTaskRevalidationPreview{}, err
	}
	preview, err := k.memoryTaskRevalidationPreviewTX(ctx, tx, scope, taskID, dependencyID, correctionID)
	if err != nil {
		return MemoryTaskRevalidationPreview{}, err
	}
	workspaceContent, err := readBlobBounded(k.root, scope.company, preview.WorkspaceDigest, 128*1024)
	if err != nil {
		return MemoryTaskRevalidationPreview{}, core.Integrity
	}
	preview.WorkspaceContent = string(workspaceContent)
	if err = tx.Commit(ctx); err != nil {
		return MemoryTaskRevalidationPreview{}, err
	}
	return preview, nil
}

// TXRevalidateMemoryTask records an explicit local-owner decision over a
// stopped Task's exact workspace and approved replacement-memory revision.
func (k *Kernel) TXRevalidateMemoryTask(ctx context.Context, scope Scope, input MemoryTaskRevalidationInput, key string) (Receipt, error) {
	if ctx == nil || !core.ValidID(key) || !core.ValidID(input.TaskID) || !core.ValidID(input.DependencyID) ||
		!core.ValidID(input.CorrectionID) || !validSHA256(input.ContextSHA256) || !validMemoryReason(input.Reason) {
		return Receipt{}, core.Malformed
	}
	preview, err := k.GetMemoryTaskRevalidationPreview(ctx, scope, input.TaskID, input.DependencyID, input.CorrectionID)
	if err != nil {
		return Receipt{}, err
	}
	if preview.ContextSHA256 != input.ContextSHA256 {
		return Receipt{}, core.ConflictError{Reason: "the reviewed memory revalidation context is stale", CurrentState: "context_changed"}
	}
	if _, err = readBlobBounded(k.root, scope.company, preview.WorkspaceDigest, 128*1024); err != nil {
		return Receipt{}, core.Integrity
	}
	revalidationID, replacementDependencyID := newID(), newID()
	return k.TXWrite(ctx, scope, nil, key, "memory.task.revalidation.requested", input, func(tx pgx.Tx) (Receipt, error) {
		current, checkErr := k.memoryTaskRevalidationPreviewTX(ctx, tx, scope, input.TaskID, input.DependencyID, input.CorrectionID)
		if checkErr != nil {
			return Receipt{}, checkErr
		}
		if current.ContextSHA256 != input.ContextSHA256 {
			return Receipt{}, core.ConflictError{Reason: "the reviewed memory revalidation context is stale", CurrentState: "context_changed"}
		}
		if _, checkErr = readBlobBounded(k.root, scope.company, current.WorkspaceDigest, 128*1024); checkErr != nil {
			return Receipt{}, core.Integrity
		}
		target := MemoryDependencyInput{RecordID: current.PreviousRecordID, RecordRevision: current.ReplacementRevision,
			TargetKind: current.TargetKind, TargetID: current.TargetID, TargetRevision: current.TargetRevision,
			TargetSHA256: current.TargetSHA256, RiskLevel: current.RiskLevel}
		if _, checkErr = validateMemoryDependencyTargetTX(ctx, tx, scope, target); checkErr != nil {
			return Receipt{}, checkErr
		}
		var createdBy string
		if checkErr = tx.QueryRow(ctx, "SELECT created_by FROM memory_dependencies WHERE company_id=$1 AND dependency_id=$2", scope.company, input.DependencyID).Scan(&createdBy); checkErr != nil {
			return Receipt{}, checkErr
		}
		if checkErr = appendEvent(ctx, tx, scope, "memory.dependency.revalidated", map[string]any{
			"task_id": input.TaskID, "dependency_id": input.DependencyID, "replacement_dependency_id": replacementDependencyID,
			"correction_id": input.CorrectionID, "replacement_revision": current.ReplacementRevision,
		}); checkErr != nil {
			return Receipt{}, checkErr
		}
		seq, checkErr := currentCompanySequenceTX(ctx, tx, scope.company)
		if checkErr != nil {
			return Receipt{}, checkErr
		}
		if _, checkErr = tx.Exec(ctx, `INSERT INTO memory_dependencies(company_id,dependency_id,record_id,record_revision,target_kind,target_id,target_revision,target_sha256,risk_level,company_seq,created_by,bound_task_id,worker_session_id)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,NULL)`, scope.company, replacementDependencyID, target.RecordID, target.RecordRevision,
			target.TargetKind, target.TargetID, target.TargetRevision, target.TargetSHA256, target.RiskLevel, seq, createdBy, input.TaskID); checkErr != nil {
			return Receipt{}, checkErr
		}
		if target.TargetKind == "mission_input" || target.TargetKind == "artifact" {
			if checkErr = appendMemoryCASRetentionPinTX(ctx, tx, scope, "memory_dependency_target", replacementDependencyID, 1,
				target.TargetKind, target.TargetID, target.TargetRevision, target.TargetSHA256); checkErr != nil {
				return Receipt{}, checkErr
			}
		}
		if checkErr = appendEvent(ctx, tx, scope, "memory.dependency.revalidated", map[string]any{
			"dependency_id": input.DependencyID, "correction_id": input.CorrectionID, "revalidation_id": revalidationID,
		}); checkErr != nil {
			return Receipt{}, checkErr
		}
		seq, checkErr = currentCompanySequenceTX(ctx, tx, scope.company)
		if checkErr != nil {
			return Receipt{}, checkErr
		}
		if _, checkErr = tx.Exec(ctx, `INSERT INTO memory_dependency_invalidation_events(company_id,company_seq,dependency_id,state,cause_kind,cause_id,actor,reason)
VALUES($1,$2,$3,'revalidated','revalidation',$4,'local-owner',$5)`, scope.company, seq, input.DependencyID, revalidationID, strings.TrimSpace(input.Reason)); checkErr != nil {
			return Receipt{}, checkErr
		}
		if checkErr = appendEvent(ctx, tx, scope, "memory.task.revalidated", map[string]any{
			"task_id": input.TaskID, "dependency_id": input.DependencyID, "correction_id": input.CorrectionID,
			"replacement_dependency_id": replacementDependencyID, "context_sha256": input.ContextSHA256,
		}); checkErr != nil {
			return Receipt{}, checkErr
		}
		seq, checkErr = currentCompanySequenceTX(ctx, tx, scope.company)
		if checkErr != nil {
			return Receipt{}, checkErr
		}
		if _, checkErr = tx.Exec(ctx, `INSERT INTO memory_task_state_events(company_id,company_seq,task_id,dependency_id,state,risk_level,cause_kind,cause_id,actor,reason)
VALUES($1,$2,$3,$4,'revalidated',$5,'revalidation',$6,'local-owner',$7)`, scope.company, seq, input.TaskID, input.DependencyID,
			current.DependencyState, revalidationID, strings.TrimSpace(input.Reason)); checkErr != nil {
			return Receipt{}, checkErr
		}
		if checkErr = appendEvent(ctx, tx, scope, "memory.task.revalidation.recorded", map[string]any{
			"revalidation_id": revalidationID, "task_id": input.TaskID, "dependency_id": input.DependencyID,
			"replacement_dependency_id": replacementDependencyID, "correction_id": input.CorrectionID,
			"task_generation": current.TaskGeneration, "task_plan_sha256": current.TaskPlanSHA256,
			"previous_revision": current.PreviousRevision, "replacement_revision": current.ReplacementRevision,
			"target_sha256": current.TargetSHA256, "stopped_session_id": current.StoppedSessionID,
			"workspace_digest": current.WorkspaceDigest, "workspace_revision": current.WorkspaceRevision,
			"reviewed_context_sha256": input.ContextSHA256, "reason": strings.TrimSpace(input.Reason),
		}); checkErr != nil {
			return Receipt{}, checkErr
		}
		seq, checkErr = currentCompanySequenceTX(ctx, tx, scope.company)
		if checkErr != nil {
			return Receipt{}, checkErr
		}
		if _, checkErr = tx.Exec(ctx, `INSERT INTO memory_task_revalidation_events(company_id,company_seq,revalidation_id,task_id,task_generation,task_plan_sha256,dependency_id,replacement_dependency_id,correction_id,
previous_record_id,previous_revision,replacement_revision,target_kind,target_id,target_revision,target_sha256,stopped_session_id,workspace_digest,workspace_revision,reviewed_context_sha256,actor,reason)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,'local-owner',$21)`, scope.company, seq, revalidationID,
			input.TaskID, current.TaskGeneration, current.TaskPlanSHA256, input.DependencyID, replacementDependencyID, input.CorrectionID, current.PreviousRecordID, current.PreviousRevision,
			current.ReplacementRevision, current.TargetKind, current.TargetID, current.TargetRevision, current.TargetSHA256,
			current.StoppedSessionID, current.WorkspaceDigest, current.WorkspaceRevision, input.ContextSHA256, strings.TrimSpace(input.Reason)); checkErr != nil {
			return Receipt{}, checkErr
		}
		if checkErr = appendMemoryCASRetentionPinTX(ctx, tx, scope, "memory_revalidation_workspace", revalidationID, 1,
			"worker_workspace", input.TaskID, current.WorkspaceRevision, current.WorkspaceDigest); checkErr != nil {
			return Receipt{}, checkErr
		}
		taskUpdate, updateErr := tx.Exec(ctx, `UPDATE tasks SET state='ready',generation=generation+1
WHERE company_id=$1 AND id=$2 AND state IN ('ready','working')`, scope.company, input.TaskID)
		if updateErr != nil {
			return Receipt{}, updateErr
		}
		if taskUpdate.RowsAffected() != 1 {
			return Receipt{}, core.Conflict
		}
		if checkErr = appendEvent(ctx, tx, scope, "task.memory_revalidation_ready", map[string]any{
			"task_id": input.TaskID, "revalidation_id": revalidationID, "generation_advanced": true,
		}); checkErr != nil {
			return Receipt{}, checkErr
		}
		return Receipt{ID: revalidationID, Status: "revalidated", Revision: current.ReplacementRevision}, nil
	})
}

// TXRevokeMemoryRecord makes the external overlay durable before committing
// its database mirror. A failed SQL commit therefore remains fail-closed in
// this process and is replayed at the next Kernel.Open.
func (k *Kernel) TXRevokeMemoryRecord(ctx context.Context, scope Scope, input MemoryRecordRevocationInput, key string) (Receipt, error) {
	if k == nil || ctx == nil || !core.ValidID(scope.company) || !core.ValidID(input.RecordID) || !core.ValidID(key) ||
		input.ExpectedRevision < 1 || !validSHA256(input.ExpectedContentSHA256) ||
		(input.ReasonCode != "incorrect" && input.ReasonCode != "sensitive" && input.ReasonCode != "requested" && input.ReasonCode != "other") {
		return Receipt{}, core.Malformed
	}
	return k.TXWrite(ctx, scope, nil, key, "memory.record.revoked", input, func(tx pgx.Tx) (Receipt, error) {
		var revision int64
		var contentSHA string
		err := tx.QueryRow(ctx, `SELECT revision,content_sha256 FROM memory_record_revisions
WHERE company_id=$1 AND record_id=$2 ORDER BY revision DESC LIMIT 1`, scope.company, input.RecordID).Scan(&revision, &contentSHA)
		if errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		}
		if err != nil {
			return Receipt{}, err
		}
		if revision != input.ExpectedRevision || contentSHA != input.ExpectedContentSHA256 {
			return Receipt{}, core.Conflict
		}
		var liveWorker bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(
SELECT 1 FROM memory_dependencies d JOIN worker_sessions s
 ON s.company_id=d.company_id AND s.task_id=d.bound_task_id
WHERE d.company_id=$1 AND d.record_id=$2 AND s.state!='stopped')`, scope.company, input.RecordID).Scan(&liveWorker); err != nil {
			return Receipt{}, err
		}
		if liveWorker {
			return Receipt{}, core.ConflictError{Reason: "stop every WorkerSession that consumed this memory before revoking it", CurrentState: "worker_active"}
		}
		if existing, ok := k.memoryRecordRevocation(scope.company, input.RecordID); ok {
			if existing.OperationID != key || existing.ContentSHA256 != contentSHA || existing.SourceRevision != revision || existing.ReasonCode != input.ReasonCode {
				return Receipt{}, core.Conflict
			}
		} else {
			overlay := MemoryRecordRevocationOverlay{
				SchemaVersion: memoryRevocationOverlaySchema, CompanyID: scope.company, RecordID: input.RecordID,
				OperationID: key, ContentSHA256: contentSHA, SourceRevision: revision, ReasonCode: input.ReasonCode,
				CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
			}
			if err = k.persistMemoryRevocationOverlay(overlay); err != nil {
				return Receipt{}, err
			}
		}
		overlay, ok := k.memoryRecordRevocation(scope.company, input.RecordID)
		if !ok {
			return Receipt{}, core.Integrity
		}
		if err = k.applyMemoryRevocationOverlayTX(ctx, tx, overlay, false); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: key, Status: "revoked", Revision: revision}, nil
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
	record, err := memoryRecordAccessTX(k, ctx, tx, b, recordID, revision)
	if err != nil {
		return MemoryRecordRevision{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return MemoryRecordRevision{}, err
	}
	return record, nil
}

func memoryRecordAccessTX(k *Kernel, ctx context.Context, tx pgx.Tx, b Binding, recordID string, revision int64) (MemoryRecordRevision, error) {
	revoked, err := k.memoryRecordRevokedTX(ctx, tx, b.scope.company, recordID)
	if err != nil {
		return MemoryRecordRevision{}, err
	}
	if revoked {
		return MemoryRecordRevision{}, core.Denied
	}
	var record MemoryRecordRevision
	var source MemorySourceReference
	var sourceKind, sourceID, sourceSHA, missionID, employeeID string
	var sourceRevision *int64
	err = tx.QueryRow(ctx, `SELECT r.record_id,v.revision,r.record_kind,r.scope_kind,COALESCE(r.mission_id,''),COALESCE(r.employee_id,''),r.sensitivity,
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
	// A revoke can publish its external overlay while this read transaction is
	// in progress. Recheck immediately before returning content so the file
	// publication is the read path's linearization barrier.
	if _, revoked := k.memoryRecordRevocation(b.scope.company, recordID); revoked {
		return MemoryRecordRevision{}, core.Denied
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
		id, risk, taskID string
	}
	rows, err := tx.Query(ctx, `SELECT dependency_id,risk_level,COALESCE(bound_task_id,'') FROM memory_dependencies
WHERE company_id=$1 AND record_id=$2 AND record_revision=$3 ORDER BY dependency_id`, b.scope.company, recordID, revision)
	if err != nil {
		return err
	}
	dependencies := make([]dependency, 0)
	for rows.Next() {
		var d dependency
		if err = rows.Scan(&d.id, &d.risk, &d.taskID); err != nil {
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
		if d.taskID != "" {
			taskState := "dirty"
			if state == "frozen" {
				taskState = "frozen"
			}
			if err = appendEvent(ctx, tx, b.scope, "memory.task."+taskState, map[string]any{
				"task_id": d.taskID, "dependency_id": d.id, "record_id": recordID,
				"record_revision": revision, "correction_id": correctionID, "risk_level": d.risk, "reason": reason,
			}); err != nil {
				return err
			}
			seq, seqErr = currentCompanySequenceTX(ctx, tx, b.scope.company)
			if seqErr != nil {
				return seqErr
			}
			if _, err = tx.Exec(ctx, `INSERT INTO memory_task_state_events(company_id,company_seq,task_id,dependency_id,state,risk_level,cause_kind,cause_id,actor,reason)
VALUES($1,$2,$3,$4,$5,$6,'memory_correction',$7,$8,$9)`, b.scope.company, seq, d.taskID, d.id, taskState, d.risk, correctionID, b.employee, reason); err != nil {
				return err
			}
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

func appendMemoryCASRetentionPinTX(ctx context.Context, tx pgx.Tx, scope Scope, referenceKind, referenceID string, referenceRevision int64,
	objectKind, objectID string, objectRevision int64, digest string) error {
	if !core.ValidID(scope.company) || !core.ValidID(referenceID) || referenceRevision < 1 ||
		!core.ValidID(objectID) || objectRevision < 1 || !validSHA256(digest) {
		return core.Malformed
	}
	validReference := (referenceKind == "memory_revision_source" || referenceKind == "memory_correction_source" || referenceKind == "memory_dependency_target") &&
		(objectKind == "mission_input" || objectKind == "artifact") ||
		referenceKind == "memory_revalidation_workspace" && objectKind == "worker_workspace"
	if !validReference || (objectKind == "artifact" && objectRevision != 1) {
		return core.Malformed
	}
	tag, err := tx.Exec(ctx, `INSERT INTO memory_cas_retention_pins(company_id,reference_kind,reference_id,reference_revision,object_kind,object_id,object_revision,digest)
VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(company_id,reference_kind,reference_id,reference_revision) DO NOTHING`,
		scope.company, referenceKind, referenceID, referenceRevision, objectKind, objectID, objectRevision, digest)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var exact bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM memory_cas_retention_pins
WHERE company_id=$1 AND reference_kind=$2 AND reference_id=$3 AND reference_revision=$4
AND object_kind=$5 AND object_id=$6 AND object_revision=$7 AND digest=$8)`,
			scope.company, referenceKind, referenceID, referenceRevision, objectKind, objectID, objectRevision, digest).Scan(&exact); err != nil {
			return err
		}
		if !exact {
			return core.Integrity
		}
	}
	return nil
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
