package domainworkflow

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

const (
	DomainProfileQualificationEvidenceSchema   = "r3-domain-profile-qualification@1"
	MaxDomainProfileQualificationEvidenceBytes = 64 << 10
	maxDomainProfileQualificationCases         = 100
)

var ErrInvalidDomainProfileQualificationEvidence = errors.New("invalid R3 domain profile qualification evidence")

type DomainProfileQualificationCaseReference struct {
	RecordID       string `json:"recordId"`
	EvidenceDigest string `json:"evidenceDigest"`
	AssessmentID   string `json:"assessmentId"`
}

type DomainProfileQualificationAreaEvidence struct {
	Area  DomainEvidenceArea                        `json:"area"`
	Cases []DomainProfileQualificationCaseReference `json:"cases"`
}

// DomainProfileQualificationEvidence is a company-scoped aggregate report.
// The Kernel separately re-reads every referenced submission and accepted
// substantive assessment before recording an owner qualification decision.
type DomainProfileQualificationEvidence struct {
	SchemaVersion   string                                   `json:"schemaVersion"`
	ProfileID       string                                   `json:"profileId"`
	ProfileRevision string                                   `json:"profileRevision"`
	Areas           []DomainProfileQualificationAreaEvidence `json:"areas"`
}

func ValidateDomainProfileQualificationEvidence(content []byte, profileID, profileRevision string) (DomainProfileQualificationEvidence, error) {
	if len(content) == 0 || len(content) > MaxDomainProfileQualificationEvidenceBytes || !validEntityID(profileID) || strings.TrimSpace(profileRevision) == "" {
		return DomainProfileQualificationEvidence{}, ErrInvalidDomainProfileQualificationEvidence
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var report DomainProfileQualificationEvidence
	if err := decoder.Decode(&report); err != nil {
		return DomainProfileQualificationEvidence{}, errors.Join(ErrInvalidDomainProfileQualificationEvidence, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return DomainProfileQualificationEvidence{}, ErrInvalidDomainProfileQualificationEvidence
	}
	if report.SchemaVersion != DomainProfileQualificationEvidenceSchema || report.ProfileID != profileID || report.ProfileRevision != profileRevision {
		return DomainProfileQualificationEvidence{}, ErrInvalidDomainProfileQualificationEvidence
	}
	var profile *WorkflowProfile
	for _, candidate := range ReferenceWorkflows() {
		if candidate.ID == report.ProfileID && candidate.Revision == report.ProfileRevision {
			profile = &candidate
			break
		}
	}
	if profile == nil || len(report.Areas) != len(profile.RequiredEvidence) {
		return DomainProfileQualificationEvidence{}, ErrInvalidDomainProfileQualificationEvidence
	}
	required := make(map[DomainEvidenceArea]struct{}, len(profile.RequiredEvidence))
	for _, area := range profile.RequiredEvidence {
		required[area] = struct{}{}
	}
	seenAreas := make(map[DomainEvidenceArea]struct{}, len(report.Areas))
	totalCases := 0
	for _, areaEvidence := range report.Areas {
		if _, ok := required[areaEvidence.Area]; !ok || len(areaEvidence.Cases) == 0 {
			return DomainProfileQualificationEvidence{}, ErrInvalidDomainProfileQualificationEvidence
		}
		if _, duplicate := seenAreas[areaEvidence.Area]; duplicate {
			return DomainProfileQualificationEvidence{}, ErrInvalidDomainProfileQualificationEvidence
		}
		seenAreas[areaEvidence.Area] = struct{}{}
		totalCases += len(areaEvidence.Cases)
		if totalCases > maxDomainProfileQualificationCases {
			return DomainProfileQualificationEvidence{}, ErrInvalidDomainProfileQualificationEvidence
		}
		seenRecords := make(map[string]struct{}, len(areaEvidence.Cases))
		for _, reference := range areaEvidence.Cases {
			if !validEntityID(reference.RecordID) || !validEntityID(reference.AssessmentID) || !validDigest(reference.EvidenceDigest) {
				return DomainProfileQualificationEvidence{}, ErrInvalidDomainProfileQualificationEvidence
			}
			if _, duplicate := seenRecords[reference.RecordID]; duplicate {
				return DomainProfileQualificationEvidence{}, ErrInvalidDomainProfileQualificationEvidence
			}
			seenRecords[reference.RecordID] = struct{}{}
		}
	}
	return report, nil
}
