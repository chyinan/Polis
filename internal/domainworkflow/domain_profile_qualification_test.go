package domainworkflow

import (
	"encoding/json"
	"testing"
)

func TestValidateDomainProfileQualificationEvidenceRequiresEveryProfileArea(t *testing.T) {
	for _, profile := range ReferenceWorkflows() {
		report := DomainProfileQualificationEvidence{
			SchemaVersion: DomainProfileQualificationEvidenceSchema,
			ProfileID:     profile.ID, ProfileRevision: profile.Revision,
			Areas: make([]DomainProfileQualificationAreaEvidence, 0, len(profile.RequiredEvidence)),
		}
		for index, area := range profile.RequiredEvidence {
			report.Areas = append(report.Areas, DomainProfileQualificationAreaEvidence{
				Area: area,
				Cases: []DomainProfileQualificationCaseReference{{
					RecordID: "domain-record-" + string(rune('a'+index)), EvidenceDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					AssessmentID: "domain-assessment-" + string(rune('a'+index)),
				}},
			})
		}
		content, err := json.Marshal(report)
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := ValidateDomainProfileQualificationEvidence(content, profile.ID, profile.Revision)
		if err != nil || len(parsed.Areas) != len(profile.RequiredEvidence) {
			t.Fatalf("valid qualification evidence for %s rejected: areas=%d err=%v", profile.ID, len(parsed.Areas), err)
		}
		if _, err = ValidateDomainProfileQualificationEvidence(content, profile.ID, "stale-profile-revision"); err == nil {
			t.Fatalf("stale profile revision accepted for %s", profile.ID)
		}
		parsed.Areas = parsed.Areas[:len(parsed.Areas)-1]
		incomplete, err := json.Marshal(parsed)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = ValidateDomainProfileQualificationEvidence(incomplete, profile.ID, profile.Revision); err == nil {
			t.Fatalf("missing required area accepted for %s", profile.ID)
		}
	}
}

func TestValidateDomainProfileQualificationEvidenceRejectsDuplicateAndEmptyReferences(t *testing.T) {
	profile := ReferenceWorkflows()[0]
	report := DomainProfileQualificationEvidence{
		SchemaVersion: DomainProfileQualificationEvidenceSchema,
		ProfileID:     profile.ID, ProfileRevision: profile.Revision,
		Areas: make([]DomainProfileQualificationAreaEvidence, 0, len(profile.RequiredEvidence)),
	}
	for index, area := range profile.RequiredEvidence {
		ref := DomainProfileQualificationCaseReference{
			RecordID: "domain-record-" + string(rune('a'+index)), EvidenceDigest: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			AssessmentID: "domain-assessment-" + string(rune('a'+index)),
		}
		report.Areas = append(report.Areas, DomainProfileQualificationAreaEvidence{Area: area, Cases: []DomainProfileQualificationCaseReference{ref}})
	}
	report.Areas[1].Cases = append(report.Areas[1].Cases, report.Areas[1].Cases[0])
	duplicate, _ := json.Marshal(report)
	if _, err := ValidateDomainProfileQualificationEvidence(duplicate, profile.ID, profile.Revision); err == nil {
		t.Fatal("duplicate case reference accepted")
	}
	report.Areas[1].Cases = nil
	empty, _ := json.Marshal(report)
	if _, err := ValidateDomainProfileQualificationEvidence(empty, profile.ID, profile.Revision); err == nil {
		t.Fatal("area without supporting cases accepted")
	}
	if _, err := ValidateDomainProfileQualificationEvidence([]byte(`{"schemaVersion":"r3-domain-profile-qualification@1","unexpected":true}`), profile.ID, profile.Revision); err == nil {
		t.Fatal("unknown report fields accepted")
	}
}
