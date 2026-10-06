// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
	"polis/internal/environment"
)

type DependencyChangeProposalRecord struct {
	CompanyID      string                               `json:"companyId"`
	ProposalID     string                               `json:"proposalId"`
	MissionID      string                               `json:"missionId"`
	BaseRevisionID string                               `json:"baseRevisionId"`
	Proposal       environment.DependencyChangeProposal `json:"proposal"`
	ProposalSHA256 string                               `json:"proposalSha256"`
	State          string                               `json:"state"`
	Actor          string                               `json:"actor"`
	Rationale      string                               `json:"rationale"`
	RequestID      string                               `json:"requestId"`
	CreatedAt      string                               `json:"createdAt"`
}

type DependencyChangeDecisionInput struct {
	ProposalID string
	Decision   string
	Rationale  string
	RequestID  string
}

func (k *Kernel) TXProposeDependencyChange(ctx context.Context, scope Scope, proposal environment.DependencyChangeProposal, requestID string) (DependencyChangeProposalRecord, error) {
	proposal.RequestID = requestID
	canonicalProposal, rawProposal, proposalDigest, err := environment.CanonicalizeDependencyChangeProposal(proposal)
	if k == nil {
		return DependencyChangeProposalRecord{}, core.StaleEpoch
	}
	if err != nil || !core.ValidID(scope.company) || !core.ValidID(requestID) {
		return DependencyChangeProposalRecord{}, core.Malformed
	}
	proposalID := "dependency-change-" + fingerprint([]string{scope.company, requestID})[:48]
	eventID := "dependency-change-event-" + fingerprint([]string{scope.company, requestID})[:40]
	_, err = k.TXWrite(ctx, scope, nil, requestID, "environment.dependency_change.propose", canonicalProposal, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureEnvironmentCompanyWritable(ctx, tx, scope.company); err != nil {
			return Receipt{}, err
		}
		var revisionMissionID, storedPolicyDigest string
		var policyManifest []byte
		if err := tx.QueryRow(ctx, `SELECT mission_id,policy_sha256,policy_manifest FROM project_environment_revisions WHERE company_id=$1 AND revision_id=$2 FOR SHARE`, scope.company, canonicalProposal.BaseRevisionID).Scan(&revisionMissionID, &storedPolicyDigest, &policyManifest); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if revisionMissionID != canonicalProposal.MissionID {
			return Receipt{}, core.OutOfScope
		}
		policy, _, canonicalPolicyDigest, policyErr := environment.ParseProjectEnvironmentPolicy(policyManifest)
		if policyErr != nil || canonicalPolicyDigest != storedPolicyDigest || !dependencyChangeHostsAllowed(policy.RegistryHosts, canonicalProposal.RegistryHosts) {
			return Receipt{}, core.Denied
		}
		if _, err := tx.Exec(ctx, `INSERT INTO environment_dependency_change_proposals(company_id,proposal_id,mission_id,base_revision_id,proposal,proposal_sha256,request_id)
VALUES($1,$2,$3,$4,$5,$6,$7)`, scope.company, proposalID, canonicalProposal.MissionID, canonicalProposal.BaseRevisionID, rawProposal, proposalDigest, requestID); err != nil {
			return Receipt{}, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO environment_dependency_change_events(company_id,event_id,proposal_id,state,actor,rationale,request_id)
VALUES($1,$2,$3,'proposed','local-owner',$4,$5)`, scope.company, eventID, proposalID, canonicalProposal.Rationale, requestID); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: proposalID, Status: "proposed"}, nil
	})
	if err != nil {
		return DependencyChangeProposalRecord{}, err
	}
	return k.DependencyChangeProposal(ctx, scope, proposalID)
}

func (k *Kernel) DecideDependencyChange(ctx context.Context, scope Scope, input DependencyChangeDecisionInput) (DependencyChangeProposalRecord, error) {
	if k == nil {
		return DependencyChangeProposalRecord{}, core.StaleEpoch
	}
	if !core.ValidID(scope.company) || !core.ValidID(input.ProposalID) || !core.ValidID(input.RequestID) || (input.Decision != "approved" && input.Decision != "rejected") {
		return DependencyChangeProposalRecord{}, core.Malformed
	}
	input.Rationale = strings.TrimSpace(input.Rationale)
	if input.Rationale == "" || len(input.Rationale) > 4096 {
		return DependencyChangeProposalRecord{}, core.Malformed
	}
	_, err := k.TXWrite(ctx, scope, nil, input.RequestID, "environment.dependency_change.decide", input, func(tx pgx.Tx) (Receipt, error) {
		var currentState string
		if err := tx.QueryRow(ctx, `SELECT state FROM environment_dependency_change_events WHERE company_id=$1 AND proposal_id=$2 ORDER BY event_seq DESC LIMIT 1 FOR UPDATE`, scope.company, input.ProposalID).Scan(&currentState); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if currentState != "proposed" && currentState != "blocked" {
			return Receipt{}, core.ConflictError{Reason: "dependency change proposal is already terminal", CurrentState: currentState}
		}
		eventID := "dependency-change-event-" + fingerprint([]string{scope.company, input.ProposalID, input.RequestID})[:40]
		if _, err := tx.Exec(ctx, `INSERT INTO environment_dependency_change_events(company_id,event_id,proposal_id,state,actor,rationale,request_id)
VALUES($1,$2,$3,$4,'local-owner',$5,$6)`, scope.company, eventID, input.ProposalID, input.Decision, input.Rationale, input.RequestID); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: input.ProposalID, Status: input.Decision}, nil
	})
	if err != nil {
		return DependencyChangeProposalRecord{}, err
	}
	return k.DependencyChangeProposal(ctx, scope, input.ProposalID)
}

func (k *Kernel) DependencyChangeProposal(ctx context.Context, scope Scope, proposalID string) (DependencyChangeProposalRecord, error) {
	if !core.ValidID(scope.company) || !core.ValidID(proposalID) {
		return DependencyChangeProposalRecord{}, core.Malformed
	}
	var record DependencyChangeProposalRecord
	var rawProposal []byte
	var createdAt time.Time
	if err := k.pool.QueryRow(ctx, `SELECT p.company_id,p.proposal_id,p.mission_id,p.base_revision_id,p.proposal,p.proposal_sha256,e.state,e.actor,e.rationale,e.request_id,p.created_at
FROM environment_dependency_change_proposals p
JOIN LATERAL (SELECT state,actor,rationale,request_id,created_at FROM environment_dependency_change_events WHERE company_id=p.company_id AND proposal_id=p.proposal_id ORDER BY event_seq DESC LIMIT 1) e ON true
WHERE p.company_id=$1 AND p.proposal_id=$2`, scope.company, proposalID).Scan(&record.CompanyID, &record.ProposalID, &record.MissionID, &record.BaseRevisionID, &rawProposal, &record.ProposalSHA256, &record.State, &record.Actor, &record.Rationale, &record.RequestID, &createdAt); errors.Is(err, pgx.ErrNoRows) {
		return DependencyChangeProposalRecord{}, core.OutOfScope
	} else if err != nil {
		return DependencyChangeProposalRecord{}, err
	}
	if err := json.Unmarshal(rawProposal, &record.Proposal); err != nil {
		return DependencyChangeProposalRecord{}, core.Integrity
	}
	canonicalProposal, _, canonicalDigest, canonicalErr := environment.CanonicalizeDependencyChangeProposal(record.Proposal)
	if canonicalErr != nil || record.ProposalSHA256 != canonicalDigest || canonicalProposal.MissionID != record.MissionID || canonicalProposal.BaseRevisionID != record.BaseRevisionID {
		return DependencyChangeProposalRecord{}, core.Integrity
	}
	record.Proposal = canonicalProposal
	record.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	return record, nil
}

func dependencyChangeHostsAllowed(policyHosts, proposalHosts []string) bool {
	allowed := make(map[string]struct{}, len(policyHosts))
	for _, host := range policyHosts {
		allowed[host] = struct{}{}
	}
	for _, host := range proposalHosts {
		if _, ok := allowed[host]; !ok {
			return false
		}
	}
	return true
}
