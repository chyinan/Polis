// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"polis/internal/core"
	"polis/internal/domainworkflow"
	"polis/internal/intake"
)

func TestDomainEvidencePreviewLoadsOnlyReferencedMissionInputAndVerifiesCAS(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	root := t.TempDir()
	k, err := Open(ctx, dsn, root)
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	companyID := fmt.Sprintf("domain-preview-%d", time.Now().UnixNano())
	scope, err := k.TXCreateCompany(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	mission, err := k.TXCreateMissionGoal(ctx, scope, "preview source", "bind preview to evidence reference", "domain-preview-mission")
	if err != nil {
		t.Fatal(err)
	}
	sourceBytes := []byte("quality report: all sampled facts were checked")
	prepared, stored, err := intake.PrepareMissionInput("quality.md", "text/markdown", sourceBytes)
	if err != nil {
		t.Fatal(err)
	}
	input, err := k.TXAddMissionInput(ctx, scope, mission.ID, "", "domain-preview-input", prepared, stored)
	if err != nil {
		t.Fatal(err)
	}
	profile := domainworkflow.ReferenceWorkflows()[0]
	submission := domainworkflow.DomainEvidenceSubmission{ProfileID: profile.ID, ProfileRevision: profile.Revision}
	for _, area := range profile.RequiredEvidence {
		submission.Evidence = append(submission.Evidence, domainworkflow.DomainEvidence{
			Area: area, Artifact: domainworkflow.DomainEvidenceArtifact{ID: input.InputID, Revision: input.Revision, SHA256: input.ContentDigest},
			MethodSHA256: strings.Repeat("b", 64), AssessedByEmployeeID: "emp-backend",
		})
	}
	record, err := k.TXRecordDomainEvidence(ctx, companyID, submission, "domain-preview-submit")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := k.ListDomainEvidenceArtifactPreviewEntries(ctx, companyID, record.RecordID, domainworkflow.DomainEvidenceQuality)
	if err != nil || manifest.InputID != input.InputID || manifest.InputRevision != input.Revision || manifest.SourceDigest != input.ContentDigest || len(manifest.Entries) != 1 || !manifest.Entries[0].Previewable || manifest.Entries[0].RelativePath != "quality.md" {
		t.Fatalf("direct-file preview manifest=(%+v,%v)", manifest, err)
	}
	preview, err := k.ReadDomainEvidenceArtifactPreview(ctx, companyID, record.RecordID, domainworkflow.DomainEvidenceQuality, "quality.md")
	if err != nil {
		t.Fatal(err)
	}
	if preview.CompanyID != companyID || preview.RecordID != record.RecordID || preview.Area != domainworkflow.DomainEvidenceQuality || preview.InputID != input.InputID || preview.InputRevision != input.Revision || preview.SourceDigest != input.ContentDigest || preview.FileName != "quality.md" || preview.MediaType != "text/markdown" || string(preview.Content) != string(sourceBytes) {
		t.Fatalf("evidence preview returned unrelated or changed content: %+v", preview)
	}
	if _, err = k.ReadDomainEvidenceArtifactPreview(ctx, companyID, record.RecordID, domainworkflow.DomainEvidenceArea("unknown"), "quality.md"); err != core.OutOfScope {
		t.Fatalf("unreferenced evidence area error=%v, want out-of-scope", err)
	}
	otherCompany := companyID + "-other"
	if _, err = k.TXCreateCompany(ctx, otherCompany); err != nil {
		t.Fatal(err)
	}
	if _, err = k.ReadDomainEvidenceArtifactPreview(ctx, otherCompany, record.RecordID, domainworkflow.DomainEvidenceQuality, "quality.md"); err != core.OutOfScope {
		t.Fatalf("cross-company evidence preview error=%v, want out-of-scope", err)
	}
	blobPath := filepath.Join(root, companyID, input.ContentDigest)
	if err = os.Remove(blobPath); err != nil {
		t.Fatal(err)
	}
	if _, err = k.ReadDomainEvidenceArtifactPreview(ctx, companyID, record.RecordID, domainworkflow.DomainEvidenceQuality, "quality.md"); err != core.Integrity {
		t.Fatalf("missing evidence CAS preview error=%v, want integrity failure", err)
	}

	directory, err := intake.PrepareDirectorySnapshot([]intake.DirectoryInputFile{
		{RelativePath: "pdf/source.md", MediaType: "text/markdown", Content: []byte("archive member")},
		{RelativePath: "pdf/vector.svg", MediaType: "image/svg+xml", Content: []byte("<svg></svg>")},
		{RelativePath: "pdf/extraction.json", MediaType: "application/json", Content: []byte(`{"source":"parser metadata"}`)},
		{RelativePath: "pdf/.polis-git-source.json", MediaType: "application/json", Content: []byte(`{"source":"git metadata"}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	archiveInput, err := k.TXAddMissionInput(ctx, scope, mission.ID, "", "domain-preview-archive-input", directory.Upload, directory.Archive)
	if err != nil {
		t.Fatal(err)
	}
	archiveSubmission := domainworkflow.DomainEvidenceSubmission{ProfileID: profile.ID, ProfileRevision: profile.Revision}
	for _, area := range profile.RequiredEvidence {
		archiveSubmission.Evidence = append(archiveSubmission.Evidence, domainworkflow.DomainEvidence{
			Area: area, Artifact: domainworkflow.DomainEvidenceArtifact{ID: archiveInput.InputID, Revision: archiveInput.Revision, SHA256: archiveInput.ContentDigest},
			MethodSHA256: strings.Repeat("d", 64), AssessedByEmployeeID: "emp-backend",
		})
	}
	archiveRecord, err := k.TXRecordDomainEvidence(ctx, companyID, archiveSubmission, "domain-preview-archive-submit")
	if err != nil {
		t.Fatal(err)
	}
	archiveManifest, err := k.ListDomainEvidenceArtifactPreviewEntries(ctx, companyID, archiveRecord.RecordID, domainworkflow.DomainEvidenceQuality)
	if err != nil || len(archiveManifest.Entries) != 4 {
		t.Fatalf("directory archive preview manifest=(%+v,%v)", archiveManifest, err)
	}
	if _, err = k.ReadDomainEvidenceArtifactPreview(ctx, companyID, archiveRecord.RecordID, domainworkflow.DomainEvidenceQuality, "pdf/source.md"); err != nil {
		t.Fatalf("verified directory text preview error=%v", err)
	}
	if _, err = k.ReadDomainEvidenceArtifactPreview(ctx, companyID, archiveRecord.RecordID, domainworkflow.DomainEvidenceQuality, "pdf/vector.svg"); !errors.Is(err, ErrDomainEvidencePreviewUnsupported) {
		t.Fatalf("unsafe SVG archive member preview error=%v, want unsupported media", err)
	}
	for _, relativePath := range []string{"pdf/extraction.json", "pdf/.polis-git-source.json"} {
		if _, err = k.ReadDomainEvidenceArtifactPreview(ctx, companyID, archiveRecord.RecordID, domainworkflow.DomainEvidenceQuality, relativePath); !errors.Is(err, ErrDomainEvidencePreviewUnsupported) {
			t.Fatalf("metadata-only directory member %q preview error=%v, want unsupported", relativePath, err)
		}
	}
}
