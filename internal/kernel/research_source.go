// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
)

type ResearchSourceRegistrationInput struct {
	Registration ResearchSourceRegistration
	Rationale    string
	RequestID    string
}

type ResearchSourceRecord struct {
	CompanyID             string `json:"companyId"`
	SourceID              string `json:"sourceId"`
	MissionID             string `json:"missionId"`
	Origin                string `json:"origin"`
	SearchEndpoint        string `json:"searchEndpoint"`
	SearchCredentialRef   string `json:"searchCredentialRef"`
	SearchRankingRevision string `json:"searchRankingRevision"`
	ProfileRevision       string `json:"profileRevision"`
	IdentitySHA256        string `json:"identitySha256"`
	DataSHA256            string `json:"dataSha256"`
	RegistrationSHA       string `json:"registrationSha256"`
	State                 string `json:"state"`
	Rationale             string `json:"rationale"`
	Actor                 string `json:"actor"`
	RequestID             string `json:"requestId"`
	CreatedAt             string `json:"createdAt"`
}

func (k *Kernel) TXRegisterResearchSource(ctx context.Context, companyID string, input ResearchSourceRegistrationInput) (ResearchSourceRecord, error) {
	if k == nil || !core.ValidID(companyID) || !core.ValidID(input.RequestID) || strings.TrimSpace(input.Rationale) == "" || len(strings.TrimSpace(input.Rationale)) > 512 {
		return ResearchSourceRecord{}, core.Malformed
	}
	normalized, err := normalizeResearchSourceRegistration(input.Registration)
	if err != nil {
		return ResearchSourceRecord{}, err
	}
	input.Rationale = strings.TrimSpace(input.Rationale)
	input.Registration = normalized
	registrationSHA, err := researchSourceRegistrationDigest(normalized)
	if err != nil {
		return ResearchSourceRecord{}, err
	}
	if prior, err := k.GetResearchSourceByRequest(ctx, companyID, input.RequestID); err == nil {
		if prior.SourceID != normalized.SourceID || prior.MissionID != normalized.MissionID || prior.Origin != normalized.Origin || prior.RegistrationSHA != registrationSHA || prior.State != "authorized" {
			return ResearchSourceRecord{}, core.Conflict
		}
		return prior, nil
	} else if !errors.Is(err, core.OutOfScope) {
		return ResearchSourceRecord{}, err
	}
	eventID := stableCapabilityID("research-source-event", companyID, input.RequestID)
	_, err = k.TXWrite(ctx, k.LocalScope(companyID), nil, input.RequestID, "research.source.register", input, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureEnvironmentCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		var missionState string
		if err := tx.QueryRow(ctx, `SELECT state FROM missions WHERE company_id=$1 AND id=$2`, companyID, normalized.MissionID).Scan(&missionState); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		searchEndpointAvailable, searchAuthAvailable, err := researchSourceOptionalColumnsAvailable(ctx, tx)
		if err != nil {
			return Receipt{}, err
		}
		if normalized.SearchEndpoint != "" && !searchEndpointAvailable {
			return Receipt{}, core.Denied
		}
		if (normalized.SearchCredentialRef != "" || normalized.SearchRankingRevision != "") && !searchAuthAvailable {
			return Receipt{}, core.Denied
		}
		var insertErr error
		if searchEndpointAvailable && searchAuthAvailable {
			_, insertErr = tx.Exec(ctx, `INSERT INTO research_source_bindings(company_id,source_id,mission_id,origin,search_endpoint,search_credential_ref,search_ranking_revision,profile_revision,identity_sha256,data_sha256,registration_sha256,request_id)
VALUES($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,''),NULLIF($7,''),$8,$9,$10,$11,$12)`, companyID, normalized.SourceID, normalized.MissionID, normalized.Origin, normalized.SearchEndpoint, normalized.SearchCredentialRef, normalized.SearchRankingRevision, normalized.ProfileRevision, normalized.IdentitySHA256, normalized.DataSHA256, registrationSHA, input.RequestID)
		} else if searchEndpointAvailable {
			_, insertErr = tx.Exec(ctx, `INSERT INTO research_source_bindings(company_id,source_id,mission_id,origin,search_endpoint,profile_revision,identity_sha256,data_sha256,registration_sha256,request_id)
VALUES($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8,$9,$10)`, companyID, normalized.SourceID, normalized.MissionID, normalized.Origin, normalized.SearchEndpoint, normalized.ProfileRevision, normalized.IdentitySHA256, normalized.DataSHA256, registrationSHA, input.RequestID)
		} else {
			_, insertErr = tx.Exec(ctx, `INSERT INTO research_source_bindings(company_id,source_id,mission_id,origin,profile_revision,identity_sha256,data_sha256,registration_sha256,request_id)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, companyID, normalized.SourceID, normalized.MissionID, normalized.Origin, normalized.ProfileRevision, normalized.IdentitySHA256, normalized.DataSHA256, registrationSHA, input.RequestID)
		}
		if insertErr != nil {
			if isUniqueViolation(insertErr) {
				return Receipt{}, core.Conflict
			}
			return Receipt{}, insertErr
		}
		if _, err := tx.Exec(ctx, `INSERT INTO research_source_events(company_id,event_id,source_id,state,rationale,actor,request_id)
VALUES($1,$2,$3,'authorized',$4,'local-owner',$5)`, companyID, eventID, normalized.SourceID, input.Rationale, input.RequestID); err != nil {
			if isUniqueViolation(err) {
				return Receipt{}, core.Conflict
			}
			return Receipt{}, err
		}
		return Receipt{ID: eventID, Status: "authorized"}, nil
	})
	if err != nil {
		return ResearchSourceRecord{}, err
	}
	return k.GetResearchSourceByRequest(ctx, companyID, input.RequestID)
}

func (k *Kernel) TXRevokeResearchSource(ctx context.Context, companyID, sourceID, rationale, requestID string) (ResearchSourceRecord, error) {
	if k == nil || !core.ValidID(companyID) || !core.ValidID(sourceID) || !core.ValidID(requestID) || strings.TrimSpace(rationale) == "" || len(strings.TrimSpace(rationale)) > 512 {
		return ResearchSourceRecord{}, core.Malformed
	}
	rationale = strings.TrimSpace(rationale)
	if prior, err := k.GetResearchSourceByRequest(ctx, companyID, requestID); err == nil {
		if prior.SourceID != sourceID || prior.State != "revoked" || prior.Rationale != rationale {
			return ResearchSourceRecord{}, core.Conflict
		}
		return prior, nil
	} else if !errors.Is(err, core.OutOfScope) {
		return ResearchSourceRecord{}, err
	}
	eventID := stableCapabilityID("research-source-event", companyID, requestID)
	_, err := k.TXWrite(ctx, k.LocalScope(companyID), nil, requestID, "research.source.revoke", []string{sourceID, rationale}, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureEnvironmentCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		var state string
		if err := tx.QueryRow(ctx, `SELECT state FROM research_source_events WHERE company_id=$1 AND source_id=$2 ORDER BY event_seq DESC LIMIT 1 FOR UPDATE`, companyID, sourceID).Scan(&state); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if state != "authorized" {
			return Receipt{}, core.Conflict
		}
		if _, err := tx.Exec(ctx, `INSERT INTO research_source_events(company_id,event_id,source_id,state,rationale,actor,request_id)
VALUES($1,$2,$3,'revoked',$4,'local-owner',$5)`, companyID, eventID, sourceID, rationale, requestID); err != nil {
			if isUniqueViolation(err) {
				return Receipt{}, core.Conflict
			}
			return Receipt{}, err
		}
		return Receipt{ID: eventID, Status: "revoked"}, nil
	})
	if err != nil {
		return ResearchSourceRecord{}, err
	}
	return k.GetResearchSourceByRequest(ctx, companyID, requestID)
}

func (k *Kernel) GetResearchSourceByRequest(ctx context.Context, companyID, requestID string) (ResearchSourceRecord, error) {
	if k == nil || !core.ValidID(companyID) || !core.ValidID(requestID) {
		return ResearchSourceRecord{}, core.Malformed
	}
	searchEndpointAvailable, searchAuthAvailable, err := researchSourceOptionalColumnsAvailable(ctx, k.pool)
	if err != nil {
		return ResearchSourceRecord{}, err
	}
	var record ResearchSourceRecord
	var createdAt time.Time
	var query string
	if searchEndpointAvailable && searchAuthAvailable {
		query = `SELECT b.company_id,b.source_id,b.mission_id,b.origin,COALESCE(b.search_endpoint,''),COALESCE(b.search_credential_ref,''),COALESCE(b.search_ranking_revision,''),b.profile_revision,b.identity_sha256,b.data_sha256,b.registration_sha256,e.state,e.rationale,e.actor,e.request_id,e.created_at
FROM research_source_bindings b JOIN research_source_events e ON e.company_id=b.company_id AND e.source_id=b.source_id
WHERE e.company_id=$1 AND e.request_id=$2`
	} else if searchEndpointAvailable {
		query = `SELECT b.company_id,b.source_id,b.mission_id,b.origin,COALESCE(b.search_endpoint,''),'' AS search_credential_ref,'' AS search_ranking_revision,b.profile_revision,b.identity_sha256,b.data_sha256,b.registration_sha256,e.state,e.rationale,e.actor,e.request_id,e.created_at
FROM research_source_bindings b JOIN research_source_events e ON e.company_id=b.company_id AND e.source_id=b.source_id
WHERE e.company_id=$1 AND e.request_id=$2`
	} else {
		query = `SELECT b.company_id,b.source_id,b.mission_id,b.origin,'' AS search_endpoint,'' AS search_credential_ref,'' AS search_ranking_revision,b.profile_revision,b.identity_sha256,b.data_sha256,b.registration_sha256,e.state,e.rationale,e.actor,e.request_id,e.created_at
FROM research_source_bindings b JOIN research_source_events e ON e.company_id=b.company_id AND e.source_id=b.source_id
WHERE e.company_id=$1 AND e.request_id=$2`
	}
	err = k.pool.QueryRow(ctx, query, companyID, requestID).Scan(
		&record.CompanyID, &record.SourceID, &record.MissionID, &record.Origin, &record.SearchEndpoint, &record.SearchCredentialRef, &record.SearchRankingRevision, &record.ProfileRevision, &record.IdentitySHA256, &record.DataSHA256, &record.RegistrationSHA, &record.State, &record.Rationale, &record.Actor, &record.RequestID, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ResearchSourceRecord{}, core.OutOfScope
	}
	if err != nil {
		return ResearchSourceRecord{}, err
	}
	record.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	return record, nil
}

func (k *Kernel) GetAuthorizedResearchSource(ctx context.Context, companyID, missionID, sourceID string) (ResearchSourceRegistration, error) {
	if k == nil || !core.ValidID(companyID) || !core.ValidID(missionID) || !core.ValidID(sourceID) {
		return ResearchSourceRegistration{}, core.Malformed
	}
	searchEndpointAvailable, searchAuthAvailable, err := researchSourceOptionalColumnsAvailable(ctx, k.pool)
	if err != nil {
		return ResearchSourceRegistration{}, err
	}
	var registration ResearchSourceRegistration
	var state string
	query := `SELECT b.source_id,b.mission_id,b.origin,'' AS search_endpoint,'' AS search_credential_ref,'' AS search_ranking_revision,b.profile_revision,b.identity_sha256,b.data_sha256,e.state
FROM research_source_bindings b JOIN LATERAL (SELECT state FROM research_source_events WHERE company_id=b.company_id AND source_id=b.source_id ORDER BY event_seq DESC LIMIT 1) e ON true
WHERE b.company_id=$1 AND b.mission_id=$2 AND b.source_id=$3`
	if searchEndpointAvailable && searchAuthAvailable {
		query = `SELECT b.source_id,b.mission_id,b.origin,COALESCE(b.search_endpoint,''),COALESCE(b.search_credential_ref,''),COALESCE(b.search_ranking_revision,''),b.profile_revision,b.identity_sha256,b.data_sha256,e.state
FROM research_source_bindings b JOIN LATERAL (SELECT state FROM research_source_events WHERE company_id=b.company_id AND source_id=b.source_id ORDER BY event_seq DESC LIMIT 1) e ON true
WHERE b.company_id=$1 AND b.mission_id=$2 AND b.source_id=$3`
	} else if searchEndpointAvailable {
		query = `SELECT b.source_id,b.mission_id,b.origin,COALESCE(b.search_endpoint,''),'' AS search_credential_ref,'' AS search_ranking_revision,b.profile_revision,b.identity_sha256,b.data_sha256,e.state
FROM research_source_bindings b JOIN LATERAL (SELECT state FROM research_source_events WHERE company_id=b.company_id AND source_id=b.source_id ORDER BY event_seq DESC LIMIT 1) e ON true
WHERE b.company_id=$1 AND b.mission_id=$2 AND b.source_id=$3`
	}
	err = k.pool.QueryRow(ctx, query, companyID, missionID, sourceID).Scan(&registration.SourceID, &registration.MissionID, &registration.Origin, &registration.SearchEndpoint, &registration.SearchCredentialRef, &registration.SearchRankingRevision, &registration.ProfileRevision, &registration.IdentitySHA256, &registration.DataSHA256, &state)
	if errors.Is(err, pgx.ErrNoRows) {
		return ResearchSourceRegistration{}, core.OutOfScope
	}
	if err != nil {
		return ResearchSourceRegistration{}, err
	}
	if state != "authorized" {
		return ResearchSourceRegistration{}, core.Denied
	}
	return normalizeResearchSourceRegistration(registration)
}

type researchSourceSchemaQueryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func researchSourceOptionalColumnsAvailable(ctx context.Context, queryer researchSourceSchemaQueryer) (bool, bool, error) {
	var endpoint, auth bool
	err := queryer.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='research_source_bindings' AND column_name='search_endpoint'), EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='research_source_bindings' AND column_name='search_credential_ref' AND EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='research_source_bindings' AND column_name='search_ranking_revision'))`).Scan(&endpoint, &auth)
	return endpoint, auth, err
}
