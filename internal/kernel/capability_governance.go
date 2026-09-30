// pattern: Imperative Shell
package kernel

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"polis/internal/core"
	"polis/internal/mcptransport"
)

type CapabilityQualification struct {
	CompanyID       string `json:"companyId"`
	QualificationID string `json:"qualificationId"`
	CapabilityKind  string `json:"capabilityKind"`
	CapabilityID    string `json:"capabilityId"`
	VersionDigest   string `json:"versionDigest"`
	Profile         string `json:"profile"`
	Status          string `json:"status"`
	EvidenceDigest  string `json:"evidenceDigest"`
	CreatedAt       string `json:"createdAt"`
}

type EmployeeCapabilityBinding struct {
	CompanyID       string `json:"companyId"`
	EventID         string `json:"eventId"`
	EmployeeID      string `json:"employeeId"`
	CapabilityKind  string `json:"capabilityKind"`
	CapabilityID    string `json:"capabilityId"`
	VersionDigest   string `json:"versionDigest"`
	QualificationID string `json:"qualificationId"`
	Qualification   string `json:"qualificationStatus"`
	State           string `json:"state"`
	ExecutionStatus string `json:"executionStatus"`
	Reason          string `json:"reason"`
	CreatedAt       string `json:"createdAt"`
}

type CapabilityDecisionRecord struct {
	CompanyID       string `json:"companyId"`
	DecisionID      string `json:"decisionId"`
	CapabilityKind  string `json:"capabilityKind"`
	CapabilityID    string `json:"capabilityId"`
	VersionDigest   string `json:"versionDigest"`
	QualificationID string `json:"qualificationId"`
	Decision        string `json:"decision"`
	Rationale       string `json:"rationale"`
	Actor           string `json:"actor"`
	CreatedAt       string `json:"createdAt"`
}

type CapabilityQualificationInput struct {
	CapabilityKind string
	CapabilityID   string
	RequestID      string
}

type CapabilityDecisionInput struct {
	CapabilityKind  string
	CapabilityID    string
	QualificationID string
	Decision        string
	Rationale       string
	RequestID       string
}

type EmployeeCapabilityBindingInput struct {
	EmployeeID      string
	CapabilityKind  string
	CapabilityID    string
	QualificationID string
	Reason          string
	RequestID       string
}

func (k *Kernel) TXQualifyCapability(ctx context.Context, companyID string, input CapabilityQualificationInput) (CapabilityQualification, error) {
	if !core.ValidID(companyID) || !core.ValidID(input.CapabilityID) || !validCapabilityKind(input.CapabilityKind) || !core.ValidID(input.RequestID) {
		return CapabilityQualification{}, core.Malformed
	}
	skillSourceVerified := false
	if input.CapabilityKind == "skill" {
		var verifyErr error
		skillSourceVerified, verifyErr = k.verifyStoredReadOnlySkillPackage(ctx, companyID, input.CapabilityID)
		if verifyErr != nil {
			return CapabilityQualification{}, verifyErr
		}
	}
	qualificationID := stableCapabilityID("qual", companyID, input.RequestID)
	scope := k.LocalScope(companyID)
	_, err := k.TXWrite(ctx, scope, nil, input.RequestID, "capability.qualification.record", struct {
		CapabilityKind string
		CapabilityID   string
	}{input.CapabilityKind, input.CapabilityID}, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureCapabilityCatalogCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		qualification, evidence, err := qualifyCapabilityMetadata(ctx, tx, companyID, qualificationID, input, skillSourceVerified)
		if err != nil {
			return Receipt{}, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO capability_qualification_records(company_id,qualification_id,capability_kind,capability_id,version_digest,profile,status,evidence_digest,evidence,request_id)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, companyID, qualification.QualificationID, qualification.CapabilityKind, qualification.CapabilityID, qualification.VersionDigest, qualification.Profile, qualification.Status, qualification.EvidenceDigest, evidence, input.RequestID); err != nil {
			if isUniqueViolation(err) {
				return Receipt{}, core.Conflict
			}
			return Receipt{}, err
		}
		return Receipt{ID: qualification.QualificationID, Status: qualification.Status}, nil
	})
	if err != nil {
		return CapabilityQualification{}, err
	}
	catalog, err := k.ListCapabilityCatalog(ctx, companyID)
	if err != nil {
		return CapabilityQualification{}, err
	}
	for _, item := range catalog.Qualifications {
		if item.QualificationID == qualificationID {
			return item, nil
		}
	}
	return CapabilityQualification{}, core.Integrity
}

func (k *Kernel) TXDecideCapability(ctx context.Context, companyID string, input CapabilityDecisionInput) (Receipt, error) {
	if !core.ValidID(companyID) || !core.ValidID(input.CapabilityID) || !core.ValidID(input.QualificationID) || !validCapabilityKind(input.CapabilityKind) || !core.ValidID(input.RequestID) || (input.Decision != "approved" && input.Decision != "revoked") || strings.TrimSpace(input.Rationale) == "" || len(input.Rationale) > 512 {
		return Receipt{}, core.Malformed
	}
	decisionID := stableCapabilityID("dec", companyID, input.RequestID)
	scope := k.LocalScope(companyID)
	return k.TXWrite(ctx, scope, nil, input.RequestID, "capability.decision.record", input, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureCapabilityCatalogCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		if input.CapabilityKind == "mcp" {
			if err := lockCapabilityMCPServerInTransaction(ctx, tx, companyID, input.CapabilityID); err != nil {
				return Receipt{}, err
			}
		}
		versionDigest, status, err := readCapabilityVersion(ctx, tx, companyID, input.CapabilityKind, input.CapabilityID)
		if err != nil {
			return Receipt{}, err
		}
		var qualifiedVersion, qualificationStatus string
		if err = tx.QueryRow(ctx, `SELECT version_digest,status FROM capability_qualification_records
WHERE company_id=$1 AND qualification_id=$2 AND capability_kind=$3 AND capability_id=$4`, companyID, input.QualificationID, input.CapabilityKind, input.CapabilityID).Scan(&qualifiedVersion, &qualificationStatus); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if versionDigest != qualifiedVersion || (input.Decision == "approved" && qualificationStatus != "metadata_verified") {
			return Receipt{}, core.Denied
		}
		if input.Decision == "approved" && status != "candidate" && status != "unverified" {
			return Receipt{}, core.ConflictError{Reason: "capability is not awaiting manual approval", CurrentState: status}
		}
		if input.Decision == "revoked" && status == "revoked" {
			return Receipt{}, core.ConflictError{Reason: "capability is already revoked", CurrentState: status}
		}
		if err = updateCapabilityStatus(ctx, tx, companyID, input.CapabilityKind, input.CapabilityID, versionDigest, input.Decision); err != nil {
			return Receipt{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO capability_decisions(company_id,decision_id,capability_kind,capability_id,version_digest,qualification_id,decision,rationale,actor,request_id)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,'local-owner',$9)`, companyID, decisionID, input.CapabilityKind, input.CapabilityID, versionDigest, input.QualificationID, input.Decision, strings.TrimSpace(input.Rationale), input.RequestID); err != nil {
			return Receipt{}, err
		}
		if input.Decision == "revoked" {
			if err = revokeCapabilityBindings(ctx, tx, companyID, input, versionDigest, decisionID); err != nil {
				return Receipt{}, err
			}
			if input.CapabilityKind == "mcp" {
				if err = revokeStdioMCPRuntimeQualifications(ctx, tx, companyID, input.CapabilityID, versionDigest, decisionID, "capability approval revoked"); err != nil {
					return Receipt{}, err
				}
			}
		}
		return Receipt{ID: decisionID, Status: input.Decision}, nil
	})
}

func (k *Kernel) TXBindEmployeeCapability(ctx context.Context, companyID string, input EmployeeCapabilityBindingInput) (Receipt, error) {
	if !core.ValidID(companyID) || !core.ValidID(input.EmployeeID) || !core.ValidID(input.CapabilityID) || !core.ValidID(input.QualificationID) || !core.ValidID(input.RequestID) || !validCapabilityKind(input.CapabilityKind) || strings.TrimSpace(input.Reason) == "" || len(input.Reason) > 512 {
		return Receipt{}, core.Malformed
	}
	eventID := stableCapabilityID("bind", companyID, input.RequestID)
	scope := k.LocalScope(companyID)
	return k.TXWrite(ctx, scope, nil, input.RequestID, "capability.employee.bind", input, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureCapabilityCatalogCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		if input.CapabilityKind == "mcp" {
			if err := lockCapabilityMCPServerInTransaction(ctx, tx, companyID, input.CapabilityID); err != nil {
				return Receipt{}, err
			}
		}
		var employeeExists bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM employees WHERE company_id=$1 AND id=$2)", companyID, input.EmployeeID).Scan(&employeeExists); err != nil {
			return Receipt{}, err
		}
		if !employeeExists {
			return Receipt{}, core.OutOfScope
		}
		versionDigest, capabilityStatus, err := readCapabilityVersion(ctx, tx, companyID, input.CapabilityKind, input.CapabilityID)
		if err != nil {
			return Receipt{}, err
		}
		if capabilityStatus != "approved" {
			return Receipt{}, core.Denied
		}
		var qualificationStatus string
		if err = tx.QueryRow(ctx, `SELECT status FROM capability_qualification_records WHERE company_id=$1 AND qualification_id=$2 AND capability_kind=$3 AND capability_id=$4 AND version_digest=$5`, companyID, input.QualificationID, input.CapabilityKind, input.CapabilityID, versionDigest).Scan(&qualificationStatus); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if qualificationStatus != "metadata_verified" {
			return Receipt{}, core.Denied
		}
		var currentEventID, currentVersion, currentEvent string
		err = tx.QueryRow(ctx, `SELECT event_id,version_digest,event FROM employee_capability_events
WHERE company_id=$1 AND employee_id=$2 AND capability_kind=$3 AND capability_id=$4
ORDER BY event_seq DESC LIMIT 1`, companyID, input.EmployeeID, input.CapabilityKind, input.CapabilityID).Scan(&currentEventID, &currentVersion, &currentEvent)
		if err == nil && currentEvent == "bound" {
			if currentVersion == versionDigest {
				return Receipt{ID: currentEventID, Status: "bound_unqualified"}, nil
			}
			return Receipt{}, core.ConflictError{Reason: "employee already has a different pinned capability version", CurrentState: currentVersion}
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO employee_capability_events(company_id,event_id,employee_id,capability_kind,capability_id,version_digest,qualification_id,event,reason,request_id)
VALUES($1,$2,$3,$4,$5,$6,$7,'bound',$8,$9)`, companyID, eventID, input.EmployeeID, input.CapabilityKind, input.CapabilityID, versionDigest, input.QualificationID, strings.TrimSpace(input.Reason), input.RequestID); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: eventID, Status: "bound_unqualified"}, nil
	})
}

func (k *Kernel) TXRevokeEmployeeCapability(ctx context.Context, companyID string, input EmployeeCapabilityBindingInput) (Receipt, error) {
	if !core.ValidID(companyID) || !core.ValidID(input.EmployeeID) || !core.ValidID(input.CapabilityID) || !validCapabilityKind(input.CapabilityKind) || !core.ValidID(input.RequestID) || strings.TrimSpace(input.Reason) == "" || len(input.Reason) > 512 {
		return Receipt{}, core.Malformed
	}
	eventID := stableCapabilityID("bind-revoke", companyID, input.RequestID)
	scope := k.LocalScope(companyID)
	return k.TXWrite(ctx, scope, nil, input.RequestID, "capability.employee.revoke", input, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureCapabilityCatalogCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		if input.CapabilityKind == "mcp" {
			if err := lockCapabilityMCPServerInTransaction(ctx, tx, companyID, input.CapabilityID); err != nil {
				return Receipt{}, err
			}
		}
		var versionDigest, qualificationID, currentEvent string
		if err := tx.QueryRow(ctx, `SELECT version_digest,qualification_id,event FROM employee_capability_events
WHERE company_id=$1 AND employee_id=$2 AND capability_kind=$3 AND capability_id=$4
ORDER BY event_seq DESC LIMIT 1`, companyID, input.EmployeeID, input.CapabilityKind, input.CapabilityID).Scan(&versionDigest, &qualificationID, &currentEvent); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if currentEvent != "bound" {
			return Receipt{}, core.ConflictError{Reason: "capability is not currently bound", CurrentState: currentEvent}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO employee_capability_events(company_id,event_id,employee_id,capability_kind,capability_id,version_digest,qualification_id,event,reason,request_id)
VALUES($1,$2,$3,$4,$5,$6,$7,'revoked',$8,$9)`, companyID, eventID, input.EmployeeID, input.CapabilityKind, input.CapabilityID, versionDigest, qualificationID, strings.TrimSpace(input.Reason), input.RequestID); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: eventID, Status: "revoked"}, nil
	})
}

func listCapabilityGovernance(ctx context.Context, tx pgx.Tx, companyID string) ([]CapabilityQualification, []EmployeeCapabilityBinding, []CapabilityDecisionRecord, error) {
	qualifications := make([]CapabilityQualification, 0)
	rows, err := tx.Query(ctx, `SELECT qualification_id,capability_kind,capability_id,version_digest,profile,status,evidence_digest,created_at::text
FROM capability_qualification_records WHERE company_id=$1 ORDER BY created_at DESC,qualification_id`, companyID)
	if err != nil {
		return nil, nil, nil, err
	}
	for rows.Next() {
		var item CapabilityQualification
		if err = rows.Scan(&item.QualificationID, &item.CapabilityKind, &item.CapabilityID, &item.VersionDigest, &item.Profile, &item.Status, &item.EvidenceDigest, &item.CreatedAt); err != nil {
			rows.Close()
			return nil, nil, nil, err
		}
		item.CompanyID = companyID
		qualifications = append(qualifications, item)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, nil, nil, err
	}
	bindings := make([]EmployeeCapabilityBinding, 0)
	rows, err = tx.Query(ctx, `SELECT DISTINCT ON (e.employee_id,e.capability_kind,e.capability_id)
e.event_id,e.employee_id,e.capability_kind,e.capability_id,e.version_digest,e.qualification_id,qualification.status,e.event,e.reason,e.created_at::text
FROM employee_capability_events e
JOIN capability_qualification_records qualification ON qualification.company_id=e.company_id AND qualification.qualification_id=e.qualification_id
WHERE e.company_id=$1
ORDER BY e.employee_id,e.capability_kind,e.capability_id,e.event_seq DESC`, companyID)
	if err != nil {
		return nil, nil, nil, err
	}
	for rows.Next() {
		var item EmployeeCapabilityBinding
		if err = rows.Scan(&item.EventID, &item.EmployeeID, &item.CapabilityKind, &item.CapabilityID, &item.VersionDigest, &item.QualificationID, &item.Qualification, &item.State, &item.Reason, &item.CreatedAt); err != nil {
			rows.Close()
			return nil, nil, nil, err
		}
		item.CompanyID = companyID
		item.ExecutionStatus = "runtime_unqualified"
		bindings = append(bindings, item)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, nil, nil, err
	}
	for index := range bindings {
		item := &bindings[index]
		if item.CapabilityKind != "mcp" || item.State != "bound" {
			continue
		}
		var runtimeStatus string
		lookupErr := tx.QueryRow(ctx, `SELECT latest.status
FROM mcp_runtime_qualification_records runtime
JOIN mcp_server_definitions capability ON capability.company_id=runtime.company_id
 AND capability.id=runtime.capability_id AND capability.status='approved'
 AND capability.descriptor_digest=runtime.version_digest
JOIN LATERAL (SELECT status FROM mcp_runtime_qualification_events event
 WHERE event.company_id=runtime.company_id AND event.runtime_qualification_id=runtime.runtime_qualification_id
 ORDER BY event.event_seq DESC LIMIT 1) latest ON true
WHERE runtime.company_id=$1 AND runtime.capability_id=$2 AND runtime.version_digest=$3
 AND runtime.capability_qualification_id=$4
ORDER BY runtime.created_at DESC,runtime.runtime_qualification_id DESC LIMIT 1`, companyID, item.CapabilityID, item.VersionDigest, item.QualificationID).Scan(&runtimeStatus)
		if lookupErr != nil && !errors.Is(lookupErr, pgx.ErrNoRows) {
			return nil, nil, nil, lookupErr
		}
		if runtimeStatus == "qualified" {
			item.ExecutionStatus = "runtime_qualified_dispatch_unavailable"
		}
	}
	decisions := make([]CapabilityDecisionRecord, 0)
	rows, err = tx.Query(ctx, `SELECT decision_id,capability_kind,capability_id,version_digest,qualification_id,decision,rationale,actor,created_at::text
FROM capability_decisions WHERE company_id=$1 ORDER BY created_at DESC,decision_id`, companyID)
	if err != nil {
		return nil, nil, nil, err
	}
	for rows.Next() {
		var item CapabilityDecisionRecord
		if err = rows.Scan(&item.DecisionID, &item.CapabilityKind, &item.CapabilityID, &item.VersionDigest, &item.QualificationID, &item.Decision, &item.Rationale, &item.Actor, &item.CreatedAt); err != nil {
			rows.Close()
			return nil, nil, nil, err
		}
		item.CompanyID = companyID
		decisions = append(decisions, item)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, nil, nil, err
	}
	return qualifications, bindings, decisions, nil
}

func qualifyCapabilityMetadata(ctx context.Context, tx pgx.Tx, companyID, qualificationID string, input CapabilityQualificationInput, skillSourceVerified bool) (CapabilityQualification, []byte, error) {
	var item CapabilityQualification
	var evidence []byte
	item = CapabilityQualification{CompanyID: companyID, QualificationID: qualificationID, CapabilityKind: input.CapabilityKind, CapabilityID: input.CapabilityID}
	switch input.CapabilityKind {
	case "skill":
		var manifest []byte
		var state string
		if err := tx.QueryRow(ctx, `SELECT content_digest,manifest,status FROM skill_revisions WHERE company_id=$1 AND id=$2`, companyID, input.CapabilityID).Scan(&item.VersionDigest, &manifest, &state); errors.Is(err, pgx.ErrNoRows) {
			return item, nil, core.OutOfScope
		} else if err != nil {
			return item, nil, err
		}
		if state == "revoked" || !validCapabilityDigest(item.VersionDigest) {
			return item, nil, core.Denied
		}
		var descriptor map[string]json.RawMessage
		if err := json.Unmarshal(manifest, &descriptor); err != nil {
			return item, nil, core.Integrity
		}
		var declaredReadOnly bool
		_ = json.Unmarshal(descriptor["readOnlyDeclared"], &declaredReadOnly)
		var importedReadOnly bool
		_ = json.Unmarshal(descriptor["readOnly"], &importedReadOnly)
		item.Profile = "read_only_skill@1"
		item.Status = "needs_external_qualification"
		if skillSourceVerified {
			item.Status = "metadata_verified"
		}
		evidence, _ = json.Marshal(struct {
			Profile               string `json:"profile"`
			ReadOnlyDeclared      bool   `json:"readOnlyDeclared"`
			SourcePackageVerified bool   `json:"sourcePackageVerified"`
			ContentDigest         string `json:"contentDigest"`
			Execution             string `json:"execution"`
		}{item.Profile, declaredReadOnly || importedReadOnly, skillSourceVerified, item.VersionDigest, "not_run"})
	case "mcp":
		var name, transport, descriptorDigest, state string
		var endpoint, command *string
		var args []byte
		if err := tx.QueryRow(ctx, `SELECT name,transport,endpoint,command,args,descriptor_digest,status FROM mcp_server_definitions WHERE company_id=$1 AND id=$2`, companyID, input.CapabilityID).Scan(&name, &transport, &endpoint, &command, &args, &descriptorDigest, &state); errors.Is(err, pgx.ErrNoRows) {
			return item, nil, core.OutOfScope
		} else if err != nil {
			return item, nil, err
		}
		if state == "revoked" || !validCapabilityDigest(descriptorDigest) {
			return item, nil, core.Denied
		}
		item.VersionDigest = descriptorDigest
		switch transport {
		case "stdio":
			if command == nil || strings.TrimSpace(*command) == "" {
				return item, nil, core.Denied
			}
			item.Profile = "stdio_mcp@1"
			item.Status = "failed"
			configDigest, digestErr := canonicalMCPDescriptorDigest(name, transport, *command, args)
			if digestErr != nil {
				return item, nil, core.Integrity
			}
			if configDigest == descriptorDigest {
				item.Status = "metadata_verified"
			}
			evidence, _ = json.Marshal(struct {
				Profile           string `json:"profile"`
				Transport         string `json:"transport"`
				DescriptorMatches bool   `json:"descriptorMatches"`
				ProcessStarted    bool   `json:"processStarted"`
			}{item.Profile, transport, configDigest == descriptorDigest, false})
		case "streamable_http":
			var argv []string
			if json.Unmarshal(args, &argv) != nil || endpoint == nil || command != nil || len(argv) != 0 {
				return item, nil, core.Denied
			}
			canonicalEndpoint, configDigest, digestErr := canonicalMCPStreamableHTTPDescriptorDigest(name, *endpoint)
			if digestErr != nil {
				return item, nil, core.Integrity
			}
			item.Profile = "streamable_http_mcp_2026_07_28@1"
			item.Status = "failed"
			if canonicalEndpoint == *endpoint && configDigest == descriptorDigest {
				item.Status = "metadata_verified"
			}
			evidence, _ = json.Marshal(struct {
				Profile            string `json:"profile"`
				Transport          string `json:"transport"`
				ProtocolVersion    string `json:"protocolVersion"`
				Endpoint           string `json:"endpoint"`
				DescriptorMatches  bool   `json:"descriptorMatches"`
				NetworkProbe       string `json:"networkProbe"`
				ToolSchemaObserved bool   `json:"toolSchemaObserved"`
				RuntimeInvoked     bool   `json:"runtimeInvoked"`
			}{item.Profile, transport, mcptransport.ProtocolVersion20260728, canonicalEndpoint, configDigest == descriptorDigest && canonicalEndpoint == *endpoint, "not_requested", false, false})
		default:
			return item, nil, core.Denied
		}
	}
	item.EvidenceDigest = digestCapabilityBytes(evidence)
	return item, evidence, nil
}

func readCapabilityVersion(ctx context.Context, tx pgx.Tx, companyID, kind, capabilityID string) (string, string, error) {
	var digest, status string
	var err error
	if kind == "skill" {
		err = tx.QueryRow(ctx, "SELECT content_digest,status FROM skill_revisions WHERE company_id=$1 AND id=$2", companyID, capabilityID).Scan(&digest, &status)
	} else {
		err = tx.QueryRow(ctx, "SELECT descriptor_digest,status FROM mcp_server_definitions WHERE company_id=$1 AND id=$2", companyID, capabilityID).Scan(&digest, &status)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", core.OutOfScope
	}
	if err != nil {
		return "", "", err
	}
	if !validCapabilityDigest(digest) {
		return "", "", core.Integrity
	}
	return digest, status, nil
}

func updateCapabilityStatus(ctx context.Context, tx pgx.Tx, companyID, kind, capabilityID, digest, status string) error {
	var commandTag pgconn.CommandTag
	var err error
	if kind == "skill" {
		commandTag, err = tx.Exec(ctx, "UPDATE skill_revisions SET status=$1 WHERE company_id=$2 AND id=$3 AND content_digest=$4", status, companyID, capabilityID, digest)
	} else {
		commandTag, err = tx.Exec(ctx, "UPDATE mcp_server_definitions SET status=$1 WHERE company_id=$2 AND id=$3 AND descriptor_digest=$4", status, companyID, capabilityID, digest)
	}
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() != 1 {
		return core.Conflict
	}
	return nil
}

func revokeCapabilityBindings(ctx context.Context, tx pgx.Tx, companyID string, input CapabilityDecisionInput, versionDigest, decisionID string) error {
	rows, err := tx.Query(ctx, `SELECT DISTINCT ON (employee_id) employee_id,version_digest,qualification_id,event
FROM employee_capability_events WHERE company_id=$1 AND capability_kind=$2 AND capability_id=$3
ORDER BY employee_id,event_seq DESC`, companyID, input.CapabilityKind, input.CapabilityID)
	if err != nil {
		return err
	}
	type currentBinding struct{ employeeID, versionDigest, qualificationID, event string }
	active := make([]currentBinding, 0)
	for rows.Next() {
		var item currentBinding
		if err = rows.Scan(&item.employeeID, &item.versionDigest, &item.qualificationID, &item.event); err != nil {
			rows.Close()
			return err
		}
		if item.event == "bound" && item.versionDigest == versionDigest {
			active = append(active, item)
		}
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	for _, item := range active {
		requestID := stableCapabilityID("revoke", decisionID, item.employeeID)
		eventID := stableCapabilityID("ev", decisionID, item.employeeID)
		if _, err = tx.Exec(ctx, `INSERT INTO employee_capability_events(company_id,event_id,employee_id,capability_kind,capability_id,version_digest,qualification_id,event,reason,request_id)
VALUES($1,$2,$3,$4,$5,$6,$7,'revoked',$8,$9)`, companyID, eventID, item.employeeID, input.CapabilityKind, input.CapabilityID, versionDigest, item.qualificationID, "capability_revoked", requestID); err != nil {
			return err
		}
	}
	return nil
}

func validCapabilityKind(kind string) bool { return kind == "skill" || kind == "mcp" }

func validCapabilityDigest(value string) bool { return validTaskInputDigest(value) }

func canonicalMCPDescriptorDigest(name, transport, command string, args []byte) (string, error) {
	descriptor := struct {
		Name      string          `json:"name"`
		Transport string          `json:"transport"`
		Command   string          `json:"command"`
		Args      json.RawMessage `json:"args"`
	}{name, transport, command, json.RawMessage(args)}
	var canonicalBuffer bytes.Buffer
	encoder := json.NewEncoder(&canonicalBuffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(descriptor); err != nil {
		return "", err
	}
	return digestCapabilityBytes(bytes.TrimSuffix(canonicalBuffer.Bytes(), []byte("\n"))), nil
}

func canonicalMCPStreamableHTTPDescriptorDigest(name, endpoint string) (string, string, error) {
	canonicalEndpoint, err := mcptransport.CanonicalEndpoint(endpoint)
	if err != nil || strings.TrimSpace(name) == "" {
		return "", "", errors.Join(err, core.Malformed)
	}
	descriptor := struct {
		SchemaVersion   string `json:"schemaVersion"`
		Name            string `json:"name"`
		Transport       string `json:"transport"`
		Endpoint        string `json:"endpoint"`
		ProtocolVersion string `json:"protocolVersion"`
	}{"mcp-streamable-http-descriptor@1", strings.TrimSpace(name), "streamable_http", canonicalEndpoint, mcptransport.ProtocolVersion20260728}
	var canonical bytes.Buffer
	encoder := json.NewEncoder(&canonical)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(descriptor); err != nil {
		return "", "", err
	}
	return canonicalEndpoint, digestCapabilityBytes(bytes.TrimSuffix(canonical.Bytes(), []byte("\n"))), nil
}

func digestCapabilityBytes(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func stableCapabilityID(prefix string, values ...string) string {
	digest := digestCapabilityBytes([]byte(strings.Join(values, "\x00")))
	return fmt.Sprintf("%s-%s", prefix, digest[:32])
}
