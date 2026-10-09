// pattern: Imperative Shell
package kernel

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"polis/internal/core"
	"polis/internal/intake"
	"polis/internal/pgfixture"
	"polis/internal/taskvalidation"
)

func TestMissionInputUploadCreatesImmutableIdempotentRevisions(t *testing.T) {
	dsn := pgfixture.DSN(t, "POLIS_TEST_DSN")
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

	companyID := "input-" + newID()
	scope, err := createPGTestCompany(k, ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	missionID := "mission-" + newID()
	if err = createPGTestMission(k, ctx, scope, missionID); err != nil {
		t.Fatal(err)
	}

	firstBytes := []byte("# Requirements\nKeep the original source unchanged.\n")
	firstPrepared, err := intake.PrepareUpload(`C:\work\requirements.md`, "text/markdown", firstBytes)
	if err != nil {
		t.Fatal(err)
	}
	first, err := k.TXAddMissionInput(ctx, scope, missionID, "", "input-upload-1", firstPrepared, firstBytes)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := k.TXAddMissionInput(ctx, scope, missionID, "", "input-upload-1", firstPrepared, firstBytes)
	if err != nil {
		t.Fatal(err)
	}
	if replayed != first || first.Revision != 1 || first.State != string(intake.StateUsable) {
		t.Fatalf("idempotent upload mismatch: first=%+v replayed=%+v", first, replayed)
	}

	mission, err := k.MissionDetails(ctx, scope, missionID)
	if err != nil {
		t.Fatal(err)
	}
	if mission.State != "draft" {
		t.Fatalf("input upload started or changed the mission: %+v", mission)
	}
	conflictBytes := []byte("# Different payload\n")
	conflictPrepared, err := intake.PrepareUpload("different.md", "text/markdown", conflictBytes)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXAddMissionInput(ctx, scope, missionID, "", "input-upload-1", conflictPrepared, conflictBytes); !errors.Is(err, core.Conflict) {
		t.Fatalf("reused request id with changed content error = %v, want conflict", err)
	}
	if _, err = os.Stat(filepath.Join(root, companyID, conflictPrepared.ContentDigest)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("conflicting request left an unreferenced CAS blob: %v", err)
	}

	secondBytes := []byte("# Requirements\nApproved replacement revision.\n")
	secondPrepared, err := intake.PrepareUpload("requirements.md", "text/markdown", secondBytes)
	if err != nil {
		t.Fatal(err)
	}
	second, err := k.TXAddMissionInput(ctx, scope, missionID, first.InputID, "input-upload-2", secondPrepared, secondBytes)
	if err != nil {
		t.Fatal(err)
	}
	if second.InputID != first.InputID || second.Revision != 2 || second.ContentDigest == first.ContentDigest {
		t.Fatalf("replacement did not create a new immutable revision: first=%+v second=%+v", first, second)
	}

	otherScope, err := createPGTestCompany(k, ctx, "other-"+newID())
	if err != nil {
		t.Fatal(err)
	}
	otherMission := "mission-" + newID()
	if err = createPGTestMission(k, ctx, otherScope, otherMission); err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXAddMissionInput(ctx, otherScope, otherMission, first.InputID, "input-cross-company", secondPrepared, secondBytes); !errors.Is(err, core.OutOfScope) {
		t.Fatalf("cross-company input revision error = %v, want out-of-scope", err)
	}

	stored, err := readBlob(k.root, scope.company, second.ContentDigest)
	if err != nil {
		t.Fatal(err)
	}
	if string(stored) != string(secondBytes) {
		t.Fatal("CAS input bytes changed after revision publication")
	}
}

func TestMissionDirectorySnapshotIsStoredAsOneVerifiedRevision(t *testing.T) {
	dsn := pgfixture.DSN(t, "POLIS_TEST_DSN")
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
	companyID := "directory-input-" + newID()
	scope, err := createPGTestCompany(k, ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	missionID := "mission-" + newID()
	if err = createPGTestMission(k, ctx, scope, missionID); err != nil {
		t.Fatal(err)
	}
	prepared, err := intake.PrepareDirectorySnapshot([]intake.DirectoryInputFile{
		{RelativePath: "project/docs/goal.md", MediaType: "text/markdown", Content: []byte("# Goal\n")},
		{RelativePath: "project/src/main.go", MediaType: "text/plain", Content: []byte("package main\n")},
	})
	if err != nil {
		t.Fatal(err)
	}
	input, err := k.TXAddMissionInput(ctx, scope, missionID, "", "directory-upload-1", prepared.Upload, prepared.Archive)
	if err != nil {
		t.Fatal(err)
	}
	if input.SourceKind != "directory_snapshot" || input.DisplayName != "project" || input.State != string(intake.StateUsable) || input.ContentDigest != prepared.Upload.ContentDigest {
		t.Fatalf("directory input row = %+v", input)
	}
	stored, err := readBlob(k.root, companyID, input.ContentDigest)
	if err != nil {
		t.Fatal(err)
	}
	if string(stored) != string(prepared.Archive) {
		t.Fatal("stored directory archive differs from the validated snapshot")
	}
	if err = intake.VerifyPreparedUpload(prepared.Upload, stored); err != nil {
		t.Fatalf("stored directory archive failed boundary revalidation: %v", err)
	}
	mission, err := k.MissionDetails(ctx, scope, missionID)
	if err != nil {
		t.Fatal(err)
	}
	if mission.State != "draft" {
		t.Fatalf("directory upload changed Mission state to %q", mission.State)
	}
}

func TestProductTaskFreezesVersionedModelInputManifest(t *testing.T) {
	dsn := pgfixture.DSN(t, "POLIS_TEST_DSN")
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
	companyID := "manifest-input-" + newID()
	scope, err := createPGTestCompany(k, ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	mission, err := createPGTestMissionGoalWithAcceptance(k, ctx, scope, "Input manifest", "freeze exact accepted inputs for one product Task", &taskvalidation.AcceptanceContract{
		Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{"Mission ID: {{mission_id}}"},
	}, "manifest-mission-create")
	if err != nil {
		t.Fatal(err)
	}
	textContent := []byte("# Mission goal\n")
	textUpload, err := intake.PrepareUpload("goal.md", "text/markdown", textContent)
	if err != nil {
		t.Fatal(err)
	}
	textRevision, err := k.TXAddMissionInput(ctx, scope, mission.ID, "", "manifest-text-input", textUpload, textContent)
	if err != nil {
		t.Fatal(err)
	}
	imageContent := []byte{137, 80, 78, 71, 13, 10, 26, 10, 0, 0, 0, 13, 73, 72, 68, 82, 0, 0, 0, 1, 0, 0, 0, 1, 8, 6, 0, 0, 0, 31, 21, 196, 137, 0, 0, 0, 11, 73, 68, 65, 84, 120, 156, 99, 96, 0, 2, 0, 0, 5, 0, 1, 167, 38, 129, 36, 0, 0, 0, 0, 73, 69, 78, 68, 174, 66, 96, 130}
	imageUpload, err := intake.PrepareUpload("reference.png", "image/png", imageContent)
	if err != nil {
		t.Fatal(err)
	}
	imageRevision, err := k.TXAddMissionInput(ctx, scope, mission.ID, "", "manifest-image-input", imageUpload, imageContent)
	if err != nil {
		t.Fatal(err)
	}
	opaque := []byte{0, 1, 2}
	unsupported, err := intake.PrepareUpload("opaque.bin", "application/octet-stream", opaque)
	must(t, err)
	_, err = k.TXAddMissionInput(ctx, scope, mission.ID, "", "manifest-unsupported-input", unsupported, opaque)
	must(t, err)
	if _, err = k.TXStartMissionCommand(ctx, scope, mission.ID, "manifest-mission-start"); err != nil {
		t.Fatal(err)
	}
	task, err := k.TXPrepareProductTask(ctx, scope, mission.ID, "prepare task input manifest", "manifest-task-prepare")
	if err != nil {
		t.Fatal(err)
	}
	first, err := k.TaskInputManifest(ctx, scope, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if first.Manifest.DeliveryStatus != "not_delivered" || len(first.Manifest.CandidateInputs) != 2 || len(first.Manifest.ExcludedInputs) != 1 {
		t.Fatalf("manifest candidates/exclusions = %d/%d, status=%s", len(first.Manifest.CandidateInputs), len(first.Manifest.ExcludedInputs), first.Manifest.DeliveryStatus)
	}
	frozen := make(map[string]intake.ModelInputManifestEntry)
	for _, ref := range first.Manifest.CandidateInputs {
		frozen[ref.InputID] = ref
	}
	if frozen[textRevision.InputID].Revision != 1 || frozen[textRevision.InputID].ContentDigest != textRevision.ContentDigest || frozen[imageRevision.InputID].ContentDigest != imageRevision.ContentDigest || first.Manifest.ExcludedInputs[0].State != intake.StateUnsupported {
		t.Fatalf("unexpected frozen input refs: %+v", first.Manifest)
	}
	inputBinding, err := k.TXNewProductProviderWorkerWithToolBudget(ctx, scope, task.ID, "offline-model/medium", 16)
	if err != nil {
		t.Fatalf("product Worker binding creation failed for Task %+v: %v", task, err)
	}
	inputContext, err := k.ProductTaskInputContext(ctx, inputBinding)
	if err != nil {
		t.Fatal(err)
	}
	var textFound, imageFound bool
	for _, input := range inputContext.Payload.Inputs {
		textFound = textFound || input.Reference.ContentDigest == textRevision.ContentDigest && input.Text == string(textContent)
	}
	for _, image := range inputContext.Payload.Images {
		imageFound = imageFound || image.Reference.ContentDigest == imageRevision.ContentDigest && string(image.Content) == string(imageContent)
	}
	if len(inputContext.Payload.Inputs) != 1 || len(inputContext.Payload.Images) != 1 || !textFound || !imageFound || inputContext.ManifestDigest != first.Digest {
		t.Fatalf("provider input context is not bound to the frozen text revision: %+v", inputContext)
	}
	newContent := []byte("# Revised goal\n")
	newUpload, err := intake.PrepareUpload("goal.md", "text/markdown", newContent)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXAddMissionInput(ctx, scope, mission.ID, textRevision.InputID, "manifest-text-input-v2", newUpload, newContent); err != nil {
		t.Fatal(err)
	}
	second, err := k.TaskInputManifest(ctx, scope, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if second.Digest != first.Digest || second.Manifest.CandidateInputs[0].Revision != 1 {
		t.Fatalf("Task input binding changed after a later Mission revision: first=%+v second=%+v", first, second)
	}
	if _, err = k.pool.Exec(ctx, "UPDATE task_input_manifests SET delivery_status='delivered' WHERE company_id=$1 AND task_id=$2", companyID, task.ID); err == nil {
		t.Fatal("database allowed mutation of the immutable Task input manifest")
	}
}
