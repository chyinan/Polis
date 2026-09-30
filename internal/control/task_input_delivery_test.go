// pattern: Imperative Shell
package control

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/giraffesyo/pdf/pdftest"
	"github.com/jackc/pgx/v5/pgxpool"
	"polis/internal/codex"
	"polis/internal/core"
	"polis/internal/intake"
	"polis/internal/kernel"
	"polis/internal/provider"
	"polis/internal/taskvalidation"
)

type promptCaptureRuntime struct {
	*provider.FakeRuntime
	prompts chan string
	images  chan []codex.TurnImage
}

func (r *promptCaptureRuntime) Start(ctx context.Context, options provider.SessionStartOptions) (provider.Session, error) {
	session, err := r.FakeRuntime.Start(ctx, options)
	if err != nil {
		return nil, err
	}
	return &promptCaptureSession{Session: session, prompts: r.prompts, images: r.images}, nil
}

type promptCaptureSession struct {
	provider.Session
	prompts chan<- string
	images  chan<- []codex.TurnImage
}

func (s *promptCaptureSession) Turn(ctx context.Context, threadID, prompt string, options codex.TurnOptions, handler provider.ToolHandler) (provider.TurnResult, error) {
	s.prompts <- prompt
	if s.images != nil {
		images := make([]codex.TurnImage, len(options.Images))
		for index, image := range options.Images {
			images[index] = codex.TurnImage{MediaType: image.MediaType, Content: append([]byte(nil), image.Content...)}
		}
		s.images <- images
	}
	return s.Session.Turn(ctx, threadID, prompt, options, handler)
}

func TestProductWorkerReceivesFrozenImageBytesAsVisionInput(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	companyID := fmt.Sprintf("image-input-delivery-%d", time.Now().UnixNano())
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err = k.TXCreateCompany(ctx, companyID); err != nil {
		t.Fatal(err)
	}
	imageBytes := workerTestPNG(t)
	promptMarker := "Attached untrusted image"
	runtime := &promptCaptureRuntime{
		FakeRuntime: provider.NewFakeRuntime(provider.FakeRuntimeConfig{TurnDelay: time.Millisecond}),
		prompts:     make(chan string, 1), images: make(chan []codex.TurnImage, 1),
	}
	adapter, err := NewRealProviderWorkerAdapter(k, runtime)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(k, adapter)
	defer service.Close()
	created, err := service.CreateMission(ctx, companyID, CreateMissionRequest{
		Title: "Image input delivery test", Goal: "Inspect the attached screenshot and use its content as task context.", RequestID: "image-input-create",
		AcceptanceContract: &taskvalidation.AcceptanceContract{Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{"Mission ID: {{mission_id}}", "Task ID: {{task_id}}", "Acknowledgement:", "Task summary:"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := service.UploadMissionInput(ctx, companyID, UploadMissionInputRequest{MissionID: created.TargetID, RequestID: "image-input-upload"}, "screen.png", "image/png", imageBytes)
	if err != nil || prepared.State != string(intake.StatePartial) {
		t.Fatalf("image upload metadata=%+v error=%v", prepared, err)
	}
	if _, err = service.StartMission(ctx, companyID, MissionCommandRequest{MissionID: created.TargetID, RequestID: "image-input-start"}); err != nil {
		t.Fatal(err)
	}
	var taskID string
	if err = pool.QueryRow(ctx, "SELECT id FROM tasks WHERE company_id=$1 AND mission_id=$2 AND kind='compat'", companyID, created.TargetID).Scan(&taskID); err != nil {
		t.Fatal(err)
	}
	manifest, err := k.TaskInputManifest(ctx, k.LocalScope(companyID), taskID)
	if err != nil || len(manifest.Manifest.CandidateInputs) != 1 || manifest.Manifest.CandidateInputs[0].InputID != prepared.InputID {
		t.Fatalf("validated image was not frozen as a Task candidate: manifest=%+v error=%v", manifest.Manifest, err)
	}
	select {
	case prompt := <-runtime.prompts:
		for _, required := range []string{promptMarker, "screen.png", prepared.ContentDigest, manifest.Digest, "untrusted user input"} {
			if !strings.Contains(prompt, required) {
				t.Fatalf("image Worker prompt omitted %q: %q", required, prompt)
			}
		}
	case <-time.After(10 * time.Second):
		t.Fatal("offline Worker did not start a turn for image input")
	}
	select {
	case images := <-runtime.images:
		if len(images) != 1 || images[0].MediaType != "image/png" || !bytes.Equal(images[0].Content, imageBytes) {
			t.Fatalf("fake provider received image payloads=%+v", images)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("offline Worker did not pass image bytes to its turn input")
	}
	var sessionState, taskState string
	var artifacts, terminalEvents int
	waitForProviderTerminal(t, ctx, pool, companyID, &sessionState, &taskState, &artifacts, &terminalEvents)
	if sessionState != "stopped" || taskState != "candidate" || artifacts != 1 || terminalEvents != 1 {
		t.Fatalf("offline image Worker did not close cleanly: session=%s task=%s artifacts=%d terminal=%d", sessionState, taskState, artifacts, terminalEvents)
	}
	var outcome, deliveredManifest string
	var egress int
	var refsJSON []byte
	if err = pool.QueryRow(ctx, `SELECT outcome,manifest_digest,input_refs,provider_egress FROM task_input_delivery_attempts
WHERE company_id=$1 AND task_id=$2 AND phase='final' ORDER BY created_at DESC LIMIT 1`, companyID, taskID).Scan(&outcome, &deliveredManifest, &refsJSON, &egress); err != nil {
		t.Fatal(err)
	}
	var refs []intake.ModelInputDeliveryRef
	if err = json.Unmarshal(refsJSON, &refs); err != nil {
		t.Fatal(err)
	}
	if outcome != "local_context_loaded" || deliveredManifest != manifest.Digest || egress != 0 || len(refs) != 1 || refs[0].InputID != prepared.InputID || refs[0].MediaType != "image/png" || refs[0].ContentDigest != prepared.ContentDigest {
		t.Fatalf("image delivery receipt=%+v outcome=%s manifest=%s egress=%d", refs, outcome, deliveredManifest, egress)
	}
}

func workerTestPNG(t *testing.T) []byte {
	t.Helper()
	imageValue := image.NewRGBA(image.Rect(0, 0, 2, 2))
	imageValue.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, imageValue); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestProductWorkerLoadsOnlyApprovedBoundReadOnlySkill(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	companyID := fmt.Sprintf("offline-skill-worker-%d", time.Now().UTC().UnixNano())
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err = k.TXCreateCompany(ctx, companyID); err != nil {
		t.Fatal(err)
	}
	skill, err := k.TXImportReadOnlySkillPackage(ctx, companyID, kernel.ReadOnlySkillPackageInput{
		Revision: "1.0.0", Archive: workerTestReadOnlySkillZIP(t),
	}, "offline-skill-import")
	if err != nil {
		t.Fatal(err)
	}
	qualification, err := k.TXQualifyCapability(ctx, companyID, kernel.CapabilityQualificationInput{
		CapabilityKind: "skill", CapabilityID: skill.ID, RequestID: "offline-skill-qualification",
	})
	if err != nil || qualification.Status != "metadata_verified" {
		t.Fatalf("read-only Skill qualification=(%+v,%v)", qualification, err)
	}
	if _, err = k.TXDecideCapability(ctx, companyID, kernel.CapabilityDecisionInput{
		CapabilityKind: "skill", CapabilityID: skill.ID, QualificationID: qualification.QualificationID,
		Decision: "approved", Rationale: "reviewed static reference content", RequestID: "offline-skill-approval",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXBindEmployeeCapability(ctx, companyID, kernel.EmployeeCapabilityBindingInput{
		EmployeeID: core.EmployeeBackendID, CapabilityKind: "skill", CapabilityID: skill.ID, QualificationID: qualification.QualificationID,
		Reason: "bind the reviewed read-only reference to the product worker", RequestID: "offline-skill-bind",
	}); err != nil {
		t.Fatal(err)
	}
	runtime := provider.NewFakeRuntime(provider.FakeRuntimeConfig{ReadOnlySkillSurface: true})
	adapter, err := NewRealProviderWorkerAdapter(k, runtime)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(k, adapter)
	defer service.Close()
	created, err := service.CreateMission(ctx, companyID, CreateMissionRequest{
		Title: "Offline Skill worker flow", Goal: "complete the task with its approved reference Skill available",
		RequestID:          "offline-skill-mission-create",
		AcceptanceContract: &taskvalidation.AcceptanceContract{Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{"Mission ID: {{mission_id}}", "Task ID: {{task_id}}", "Acknowledgement:", "Task summary:"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.StartMission(ctx, companyID, MissionCommandRequest{MissionID: created.TargetID, RequestID: "offline-skill-mission-start"}); err != nil {
		t.Fatal(err)
	}
	var sessionState, taskState string
	var artifacts, terminalEvents int
	waitForProviderTerminal(t, ctx, pool, companyID, &sessionState, &taskState, &artifacts, &terminalEvents)
	if sessionState != "stopped" || taskState != "candidate" || artifacts != 1 || terminalEvents != 1 {
		t.Fatalf("offline Skill Worker did not close cleanly: session=%s task=%s artifacts=%d terminal=%d", sessionState, taskState, artifacts, terminalEvents)
	}
	var loadedCount int
	var loadedSkillID, loadedVersion, loadedPath string
	var leakedContent bool
	if err = pool.QueryRow(ctx, `SELECT count(*),COALESCE(min(payload->>'skillId'),''),COALESCE(min(payload->>'versionDigest'),''),COALESCE(min(payload->>'relativePath'),''),COALESCE(bool_or(payload ? 'content'),false)
FROM events WHERE company_id=$1 AND kind='capability.skill.loaded'`, companyID).Scan(&loadedCount, &loadedSkillID, &loadedVersion, &loadedPath, &leakedContent); err != nil {
		t.Fatal(err)
	}
	if loadedCount != 1 || loadedSkillID != skill.ID || loadedVersion != skill.ContentDigest || loadedPath != "SKILL.md" || leakedContent {
		t.Fatalf("offline Skill use evidence count=%d skill=%s version=%s path=%s leakedContent=%t", loadedCount, loadedSkillID, loadedVersion, loadedPath, leakedContent)
	}
	if runtime.Stats().ProviderEgress != 0 || runtime.Stats().Reservations != 0 {
		t.Fatalf("offline Skill Worker used provider egress: %+v", runtime.Stats())
	}
}

func workerTestReadOnlySkillZIP(t *testing.T) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	files := []struct{ path, content string }{
		{path: "SKILL.md", content: "---\nname: worker-reference-skill\ndescription: A fixed read-only project reference.\n---\nRead only the bound reference content; do not execute code.\n"},
		{path: "references/guide.md", content: "Approved offline reference material.\n"},
	}
	for _, file := range files {
		entry, err := writer.Create(file.path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = entry.Write([]byte(file.content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestProductWorkerReceivesFrozenGitSnapshotFromSelectedCommit(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	companyID := fmt.Sprintf("git-input-delivery-%d", time.Now().UnixNano())
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err = k.TXCreateCompany(ctx, companyID); err != nil {
		t.Fatal(err)
	}
	runtime := &promptCaptureRuntime{FakeRuntime: provider.NewFakeRuntime(provider.FakeRuntimeConfig{TurnDelay: time.Millisecond}), prompts: make(chan string, 1)}
	adapter, err := NewRealProviderWorkerAdapter(k, runtime)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(k, adapter)
	defer service.Close()
	created, err := service.CreateMission(ctx, companyID, CreateMissionRequest{
		Title: "Git snapshot delivery test", Goal: "Read the selected local commit and its source provenance.", RequestID: "git-input-create",
		AcceptanceContract: &taskvalidation.AcceptanceContract{Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{"Mission ID: {{mission_id}}", "Task ID: {{task_id}}", "Acknowledgement:", "Task summary:"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	repository := filepath.Join(t.TempDir(), "source")
	if err = os.MkdirAll(repository, 0o700); err != nil {
		t.Fatal(err)
	}
	controlGitFixture(t, repository, "init", "-q")
	controlGitFixture(t, repository, "config", "user.name", "Polis Fixture")
	controlGitFixture(t, repository, "config", "user.email", "polis-fixture@example.invalid")
	if err = os.WriteFile(filepath.Join(repository, "README.md"), []byte("POLIS_GIT_WORKER_COMMITTED_SENTINEL\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	controlGitFixture(t, repository, "add", "README.md")
	controlGitFixture(t, repository, "commit", "-q", "-m", "fixture")
	commitID := strings.TrimSpace(controlGitFixture(t, repository, "rev-parse", "HEAD"))
	if err = os.WriteFile(filepath.Join(repository, "README.md"), []byte("POLIS_GIT_WORKER_UNCOMMITTED_SENTINEL\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot, err := intake.ImportGitCommit(ctx, repository, commitID)
	if err != nil {
		t.Fatal(err)
	}
	input, err := k.TXAddMissionInput(ctx, k.LocalScope(companyID), created.TargetID, "", "git-input-import", snapshot.Upload, snapshot.Archive)
	if err != nil || input.State != string(intake.StatePartial) {
		t.Fatalf("Git snapshot input=%+v error=%v", input, err)
	}
	if _, err = service.StartMission(ctx, companyID, MissionCommandRequest{MissionID: created.TargetID, RequestID: "git-input-start"}); err != nil {
		t.Fatal(err)
	}
	var taskID string
	if err = pool.QueryRow(ctx, "SELECT id FROM tasks WHERE company_id=$1 AND mission_id=$2 AND kind='compat'", companyID, created.TargetID).Scan(&taskID); err != nil {
		t.Fatal(err)
	}
	manifest, err := k.TaskInputManifest(ctx, k.LocalScope(companyID), taskID)
	if err != nil || len(manifest.Manifest.CandidateInputs) != 1 || manifest.Manifest.CandidateInputs[0].SourceKind != "git_snapshot" {
		t.Fatalf("Git snapshot was not frozen into the Task manifest: manifest=%+v error=%v", manifest.Manifest, err)
	}
	select {
	case prompt := <-runtime.prompts:
		for _, required := range []string{"POLIS_GIT_WORKER_COMMITTED_SENTINEL", commitID, "not_included_by_policy", "separate directory snapshot", "networkFetch", "disabled", manifest.Digest} {
			if !strings.Contains(prompt, required) {
				t.Fatalf("Git Worker prompt omitted %q: %q", required, prompt)
			}
		}
		if strings.Contains(prompt, "POLIS_GIT_WORKER_UNCOMMITTED_SENTINEL") {
			t.Fatalf("uncommitted worktree bytes entered the frozen Worker prompt: %q", prompt)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("offline Worker did not start a turn for Git input")
	}
	var sessionState, taskState string
	var artifacts, terminalEvents int
	waitForProviderTerminal(t, ctx, pool, companyID, &sessionState, &taskState, &artifacts, &terminalEvents)
	if sessionState != "stopped" || taskState != "candidate" || artifacts != 1 || terminalEvents != 1 {
		t.Fatalf("offline Git input Worker did not close cleanly: session=%s task=%s artifacts=%d terminal=%d", sessionState, taskState, artifacts, terminalEvents)
	}
	var outcome, deliveredManifest string
	var egress int
	var refsJSON []byte
	if err = pool.QueryRow(ctx, `SELECT outcome,manifest_digest,input_refs,provider_egress FROM task_input_delivery_attempts
WHERE company_id=$1 AND task_id=$2 AND phase='final' ORDER BY created_at DESC LIMIT 1`, companyID, taskID).Scan(&outcome, &deliveredManifest, &refsJSON, &egress); err != nil {
		t.Fatal(err)
	}
	var refs []intake.ModelInputDeliveryRef
	if err = json.Unmarshal(refsJSON, &refs); err != nil {
		t.Fatal(err)
	}
	if outcome != "local_context_loaded" || deliveredManifest != manifest.Digest || egress != 0 || len(refs) != 2 {
		t.Fatalf("Git delivery receipt refs=%+v outcome=%s manifest=%s egress=%d", refs, outcome, deliveredManifest, egress)
	}
	gotReadme, gotSourceNote := false, false
	for _, ref := range refs {
		gotReadme = gotReadme || strings.HasSuffix(ref.RelativePath, "/README.md")
		gotSourceNote = gotSourceNote || strings.HasSuffix(ref.RelativePath, "/.polis-git-source.json")
	}
	if !gotReadme || !gotSourceNote {
		t.Fatalf("Git delivery receipt omitted source or provenance references: %+v", refs)
	}
}

func controlGitFixture(t *testing.T, repository string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", repository}, arguments...)...)
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("Git fixture command %q failed: %v: %s", strings.Join(arguments, " "), err, output)
	}
	return string(output)
}

func TestProductWorkerReceivesFrozenTextInputsInTurnPrompt(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	companyID := fmt.Sprintf("input-delivery-%d", time.Now().UnixNano())
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err = k.TXCreateCompany(ctx, companyID); err != nil {
		t.Fatal(err)
	}
	promptMarker := "POLIS_BOUND_INPUT_SENTINEL_2026"
	runtime := &promptCaptureRuntime{FakeRuntime: provider.NewFakeRuntime(provider.FakeRuntimeConfig{TurnDelay: time.Millisecond}), prompts: make(chan string, 1)}
	adapter, err := NewRealProviderWorkerAdapter(k, runtime)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(k, adapter)
	defer service.Close()
	created, err := service.CreateMission(ctx, companyID, CreateMissionRequest{
		Title: "Input delivery test", Goal: "Read the bound task inputs before writing the artifact.", RequestID: "input-delivery-create",
		AcceptanceContract: &taskvalidation.AcceptanceContract{Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{"Mission ID: {{mission_id}}", "Task ID: {{task_id}}", "Acknowledgement:", "Task summary:"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.UploadMissionInput(ctx, companyID, UploadMissionInputRequest{MissionID: created.TargetID, RequestID: "input-delivery-upload"}, "source.md", "text/markdown", []byte("Use this source fact: "+promptMarker)); err != nil {
		t.Fatal(err)
	}
	if _, err = service.StartMission(ctx, companyID, MissionCommandRequest{MissionID: created.TargetID, RequestID: "input-delivery-start"}); err != nil {
		t.Fatal(err)
	}
	var taskID string
	if err = pool.QueryRow(ctx, "SELECT id FROM tasks WHERE company_id=$1 AND mission_id=$2 AND kind='compat'", companyID, created.TargetID).Scan(&taskID); err != nil {
		t.Fatal(err)
	}
	scope := k.LocalScope(companyID)
	manifest, err := k.TaskInputManifest(ctx, scope, taskID)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case prompt := <-runtime.prompts:
		if !strings.Contains(prompt, promptMarker) {
			t.Fatalf("provider turn prompt omitted bound input text: %q", prompt)
		}
		if !strings.Contains(prompt, manifest.Digest) || !strings.Contains(prompt, "untrusted user input") {
			t.Fatalf("provider turn prompt omitted input provenance/trust boundary: %q", prompt)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("offline Worker did not start a turn")
	}
	var sessionState, taskState string
	var artifacts, terminalEvents int
	waitForProviderTerminal(t, ctx, pool, companyID, &sessionState, &taskState, &artifacts, &terminalEvents)
	if sessionState != "stopped" || taskState != "candidate" || artifacts != 1 || terminalEvents != 1 {
		t.Fatalf("offline input delivery Worker did not close cleanly: session=%s task=%s artifacts=%d terminal=%d", sessionState, taskState, artifacts, terminalEvents)
	}
	var status, manifestDigest string
	if err = pool.QueryRow(ctx, `SELECT outcome,manifest_digest FROM task_input_delivery_attempts
WHERE company_id=$1 AND task_id=$2 AND phase='final' ORDER BY created_at DESC LIMIT 1`, companyID, taskID).Scan(&status, &manifestDigest); err != nil {
		t.Fatal(err)
	}
	if status != "local_context_loaded" || manifestDigest != manifest.Digest {
		t.Fatalf("offline input delivery receipt = status %q manifest %q", status, manifestDigest)
	}
}

func TestProductWorkerReceivesFrozenImageBytesInTurnInput(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	companyID := fmt.Sprintf("image-input-delivery-%d", time.Now().UnixNano())
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err = k.TXCreateCompany(ctx, companyID); err != nil {
		t.Fatal(err)
	}
	imageBytes := workerTestPNG(t)
	runtime := &promptCaptureRuntime{
		FakeRuntime: provider.NewFakeRuntime(provider.FakeRuntimeConfig{TurnDelay: time.Millisecond}),
		prompts:     make(chan string, 1), images: make(chan []codex.TurnImage, 1),
	}
	adapter, err := NewRealProviderWorkerAdapter(k, runtime)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(k, adapter)
	defer service.Close()
	created, err := service.CreateMission(ctx, companyID, CreateMissionRequest{
		Title: "Image input delivery test", Goal: "Inspect the attached screenshot and use it as untrusted task context.", RequestID: "image-input-create",
		AcceptanceContract: &taskvalidation.AcceptanceContract{Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{"Mission ID: {{mission_id}}", "Task ID: {{task_id}}", "Acknowledgement:", "Task summary:"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := service.UploadMissionInput(ctx, companyID, UploadMissionInputRequest{MissionID: created.TargetID, RequestID: "image-input-upload"}, "screen.png", "image/png", imageBytes)
	if err != nil || prepared.State != string(intake.StatePartial) {
		t.Fatalf("image upload=%+v error=%v", prepared, err)
	}
	if _, err = service.StartMission(ctx, companyID, MissionCommandRequest{MissionID: created.TargetID, RequestID: "image-input-start"}); err != nil {
		t.Fatal(err)
	}
	var taskID string
	if err = pool.QueryRow(ctx, "SELECT id FROM tasks WHERE company_id=$1 AND mission_id=$2 AND kind='compat'", companyID, created.TargetID).Scan(&taskID); err != nil {
		t.Fatal(err)
	}
	manifest, err := k.TaskInputManifest(ctx, k.LocalScope(companyID), taskID)
	if err != nil || len(manifest.Manifest.CandidateInputs) != 1 || manifest.Manifest.CandidateInputs[0].InputID != prepared.InputID {
		t.Fatalf("validated image was not frozen as a Task candidate: manifest=%+v error=%v", manifest.Manifest, err)
	}
	select {
	case prompt := <-runtime.prompts:
		for _, required := range []string{"Attached untrusted image", "screen.png", prepared.ContentDigest, manifest.Digest, "untrusted user input"} {
			if !strings.Contains(prompt, required) {
				t.Fatalf("Worker prompt omitted image reference %q: %q", required, prompt)
			}
		}
	case <-time.After(10 * time.Second):
		t.Fatal("offline Worker did not start for the image input")
	}
	select {
	case images := <-runtime.images:
		if len(images) != 1 || images[0].MediaType != "image/png" || !bytes.Equal(images[0].Content, imageBytes) {
			t.Fatalf("fake provider did not receive the frozen image bytes: %+v", images)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Worker did not pass image bytes into the provider turn input")
	}
	var sessionState, taskState string
	var artifacts, terminalEvents int
	waitForProviderTerminal(t, ctx, pool, companyID, &sessionState, &taskState, &artifacts, &terminalEvents)
	if sessionState != "stopped" || taskState != "candidate" || artifacts != 1 || terminalEvents != 1 {
		t.Fatalf("offline image Worker did not close cleanly: session=%s task=%s artifacts=%d terminal=%d", sessionState, taskState, artifacts, terminalEvents)
	}
	var outcome, deliveredManifest string
	var egress int
	var refsJSON []byte
	if err = pool.QueryRow(ctx, `SELECT outcome,manifest_digest,input_refs,provider_egress FROM task_input_delivery_attempts
WHERE company_id=$1 AND task_id=$2 AND phase='final' ORDER BY created_at DESC LIMIT 1`, companyID, taskID).Scan(&outcome, &deliveredManifest, &refsJSON, &egress); err != nil {
		t.Fatal(err)
	}
	var refs []intake.ModelInputDeliveryRef
	if err = json.Unmarshal(refsJSON, &refs); err != nil {
		t.Fatal(err)
	}
	if outcome != "local_context_loaded" || deliveredManifest != manifest.Digest || egress != 0 || len(refs) != 1 || refs[0].InputID != prepared.InputID || refs[0].MediaType != "image/png" || refs[0].ContentDigest != prepared.ContentDigest {
		t.Fatalf("image delivery receipt=%+v outcome=%s manifest=%s egress=%d", refs, outcome, deliveredManifest, egress)
	}
}

func TestProductWorkerReceivesExtractedTextFromBoundPDFInput(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	companyID := fmt.Sprintf("pdf-input-delivery-%d", time.Now().UnixNano())
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err = k.TXCreateCompany(ctx, companyID); err != nil {
		t.Fatal(err)
	}
	sentinel := "POLIS_PDF_WORKER_INPUT_SENTINEL_2026"
	runtime := &promptCaptureRuntime{FakeRuntime: provider.NewFakeRuntime(provider.FakeRuntimeConfig{TurnDelay: time.Millisecond}), prompts: make(chan string, 1)}
	adapter, err := NewRealProviderWorkerAdapter(k, runtime)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(k, adapter)
	defer service.Close()
	created, err := service.CreateMission(ctx, companyID, CreateMissionRequest{
		Title: "PDF input delivery test", Goal: "Read the text extracted from the bound PDF source.", RequestID: "pdf-input-create",
		AcceptanceContract: &taskvalidation.AcceptanceContract{Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{"Mission ID: {{mission_id}}", "Task ID: {{task_id}}", "Acknowledgement:", "Task summary:"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := service.UploadMissionInput(ctx, companyID, UploadMissionInputRequest{MissionID: created.TargetID, RequestID: "pdf-input-upload"}, "brief.pdf", "application/pdf", workerTestTextPDF(sentinel))
	if err != nil {
		t.Fatal(err)
	}
	if prepared.SourceKind != "pdf_snapshot" || prepared.State != string(intake.StatePartial) {
		t.Fatalf("PDF input receipt = %+v", prepared)
	}
	if _, err = service.StartMission(ctx, companyID, MissionCommandRequest{MissionID: created.TargetID, RequestID: "pdf-input-start"}); err != nil {
		t.Fatal(err)
	}
	var taskID string
	if err = pool.QueryRow(ctx, "SELECT id FROM tasks WHERE company_id=$1 AND mission_id=$2 AND kind='compat'", companyID, created.TargetID).Scan(&taskID); err != nil {
		t.Fatal(err)
	}
	manifest, err := k.TaskInputManifest(ctx, k.LocalScope(companyID), taskID)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case prompt := <-runtime.prompts:
		for _, required := range []string{sentinel, "pdf/extracted.txt", manifest.Digest, "untrusted user input"} {
			if !strings.Contains(prompt, required) {
				t.Fatalf("PDF Worker prompt omitted %q", required)
			}
		}
		if strings.Contains(prompt, "%PDF-1.") || strings.Contains(prompt, "%%EOF") {
			t.Fatalf("binary PDF source was sent as model text: %q", prompt)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("offline Worker did not start a turn for PDF input")
	}
	var sessionState, taskState string
	var artifacts, terminalEvents int
	waitForProviderTerminal(t, ctx, pool, companyID, &sessionState, &taskState, &artifacts, &terminalEvents)
	if sessionState != "stopped" || taskState != "candidate" || artifacts != 1 || terminalEvents != 1 {
		t.Fatalf("offline PDF input Worker did not close cleanly: session=%s task=%s artifacts=%d terminal=%d", sessionState, taskState, artifacts, terminalEvents)
	}
	var outcome, deliveredManifest string
	var inputRefsJSON, inputExclusionsJSON []byte
	var providerEgress int
	if err = pool.QueryRow(ctx, `SELECT outcome,manifest_digest,input_refs,input_exclusions,provider_egress FROM task_input_delivery_attempts
WHERE company_id=$1 AND task_id=$2 AND phase='final' ORDER BY created_at DESC LIMIT 1`, companyID, taskID).Scan(&outcome, &deliveredManifest, &inputRefsJSON, &inputExclusionsJSON, &providerEgress); err != nil {
		t.Fatal(err)
	}
	var inputRefs []intake.ModelInputDeliveryRef
	var inputExclusions []intake.ModelInputExclusion
	if err = json.Unmarshal(inputRefsJSON, &inputRefs); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(inputExclusionsJSON, &inputExclusions); err != nil {
		t.Fatal(err)
	}
	if outcome != "local_context_loaded" || deliveredManifest != manifest.Digest || providerEgress != 0 || !hasPDFDeliveryPath(inputRefs, "pdf/extracted.txt") || !hasPDFDeliveryExclusion(inputExclusions, "pdf/original.pdf") {
		t.Fatalf("PDF local delivery receipt = outcome %q manifest %q egress %d refs %+v exclusions %+v", outcome, deliveredManifest, providerEgress, inputRefs, inputExclusions)
	}
}

func workerTestTextPDF(value string) []byte {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "(", "\\(")
	value = strings.ReplaceAll(value, ")", "\\)")
	content := fmt.Sprintf("BT /F1 12 Tf 72 720 Td (%s) Tj ET", value)
	return pdftest.Build(1,
		pdftest.Catalog(2),
		pdftest.Pages(3),
		pdftest.Page(2, 4, "<< /Font << /F1 5 0 R >> >>"),
		pdftest.Stream("", content),
		pdftest.Helvetica(),
	)
}

func hasPDFDeliveryPath(refs []intake.ModelInputDeliveryRef, path string) bool {
	for _, ref := range refs {
		if ref.RelativePath == path {
			return true
		}
	}
	return false
}

func hasPDFDeliveryExclusion(exclusions []intake.ModelInputExclusion, path string) bool {
	for _, exclusion := range exclusions {
		if exclusion.RelativePath == path && exclusion.Reason == "representation_not_supported" {
			return true
		}
	}
	return false
}

func TestProductWorkerReceivesTextFilesFromFrozenDirectoryInput(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	companyID := fmt.Sprintf("directory-worker-input-%d", time.Now().UnixNano())
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err = k.TXCreateCompany(ctx, companyID); err != nil {
		t.Fatal(err)
	}
	runtime := &promptCaptureRuntime{FakeRuntime: provider.NewFakeRuntime(provider.FakeRuntimeConfig{TurnDelay: time.Millisecond}), prompts: make(chan string, 1)}
	adapter, err := NewRealProviderWorkerAdapter(k, runtime)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(k, adapter)
	defer service.Close()
	created, err := service.CreateMission(ctx, companyID, CreateMissionRequest{
		Title: "Directory input delivery test", Goal: "Read supported files in the frozen directory snapshot.", RequestID: "directory-input-create",
		AcceptanceContract: &taskvalidation.AcceptanceContract{Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{"Mission ID: {{mission_id}}", "Task ID: {{task_id}}", "Acknowledgement:", "Task summary:"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	textMarker, excludedMarker := "POLIS_DIRECTORY_TEXT_SENTINEL_2026", "POLIS_DIRECTORY_BINARY_SENTINEL_2026"
	if _, err = service.UploadMissionDirectoryInput(ctx, companyID, UploadMissionInputRequest{MissionID: created.TargetID, RequestID: "directory-input-upload"}, []intake.DirectoryInputFile{
		{RelativePath: "project/README.md", MediaType: "text/markdown", Content: []byte("Read this directory fact: " + textMarker)},
		{RelativePath: "project/data.bin", MediaType: "application/octet-stream", Content: []byte(excludedMarker)},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = service.StartMission(ctx, companyID, MissionCommandRequest{MissionID: created.TargetID, RequestID: "directory-input-start"}); err != nil {
		t.Fatal(err)
	}
	var taskID string
	if err = pool.QueryRow(ctx, "SELECT id FROM tasks WHERE company_id=$1 AND mission_id=$2 AND kind='compat'", companyID, created.TargetID).Scan(&taskID); err != nil {
		t.Fatal(err)
	}
	manifest, err := k.TaskInputManifest(ctx, k.LocalScope(companyID), taskID)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case prompt := <-runtime.prompts:
		for _, required := range []string{textMarker, "project/README.md", manifest.Digest, "untrusted user input", "project/data.bin (representation_not_supported)"} {
			if !strings.Contains(prompt, required) {
				t.Fatalf("directory Worker prompt omitted %q: %q", required, prompt)
			}
		}
		if strings.Contains(prompt, excludedMarker) {
			t.Fatalf("unsupported binary sibling entered the Worker prompt: %q", prompt)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("offline Worker did not start a turn for directory input")
	}
	var sessionState, taskState string
	var artifacts, terminalEvents int
	waitForProviderTerminal(t, ctx, pool, companyID, &sessionState, &taskState, &artifacts, &terminalEvents)
	if sessionState != "stopped" || taskState != "candidate" || artifacts != 1 || terminalEvents != 1 {
		t.Fatalf("offline directory input Worker did not close cleanly: session=%s task=%s artifacts=%d terminal=%d", sessionState, taskState, artifacts, terminalEvents)
	}
	var deliveryOutcome, deliveredManifest string
	var providerEgress int
	if err = pool.QueryRow(ctx, `SELECT outcome,manifest_digest,provider_egress FROM task_input_delivery_attempts
WHERE company_id=$1 AND task_id=$2 AND phase='final' ORDER BY created_at DESC LIMIT 1`, companyID, taskID).Scan(&deliveryOutcome, &deliveredManifest, &providerEgress); err != nil {
		t.Fatal(err)
	}
	if deliveryOutcome != "local_context_loaded" || deliveredManifest != manifest.Digest || providerEgress != 0 {
		t.Fatalf("offline directory delivery receipt = outcome %q manifest %q provider_egress %d", deliveryOutcome, deliveredManifest, providerEgress)
	}
}

func TestProductWorkerReceivesFrozenCSVInputInTurnPrompt(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	companyID := fmt.Sprintf("csv-input-delivery-%d", time.Now().UnixNano())
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err = k.TXCreateCompany(ctx, companyID); err != nil {
		t.Fatal(err)
	}
	marker := "POLIS_CSV_WORKER_INPUT_SENTINEL_2026"
	csvBytes := []byte("name,value\nalpha," + marker + "\n")
	runtime := &promptCaptureRuntime{FakeRuntime: provider.NewFakeRuntime(provider.FakeRuntimeConfig{TurnDelay: time.Millisecond}), prompts: make(chan string, 1)}
	adapter, err := NewRealProviderWorkerAdapter(k, runtime)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(k, adapter)
	defer service.Close()
	created, err := service.CreateMission(ctx, companyID, CreateMissionRequest{
		Title: "CSV input delivery test", Goal: "Read the bound CSV source before writing the artifact.", RequestID: "csv-input-create",
		AcceptanceContract: &taskvalidation.AcceptanceContract{Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{"Mission ID: {{mission_id}}", "Task ID: {{task_id}}", "Acknowledgement:", "Task summary:"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := service.UploadMissionInput(ctx, companyID, UploadMissionInputRequest{MissionID: created.TargetID, RequestID: "csv-input-upload"}, "metrics.csv", "text/csv", csvBytes)
	if err != nil {
		t.Fatalf("upload CSV input: %v", err)
	}
	if _, err = service.StartMission(ctx, companyID, MissionCommandRequest{MissionID: created.TargetID, RequestID: "csv-input-start"}); err != nil {
		t.Fatal(err)
	}
	var taskID string
	if err = pool.QueryRow(ctx, "SELECT id FROM tasks WHERE company_id=$1 AND mission_id=$2 AND kind='compat'", companyID, created.TargetID).Scan(&taskID); err != nil {
		t.Fatal(err)
	}
	manifest, err := k.TaskInputManifest(ctx, k.LocalScope(companyID), taskID)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case prompt := <-runtime.prompts:
		for _, required := range []string{marker, "metrics.csv", prepared.ContentDigest, manifest.Digest, "untrusted user input"} {
			if !strings.Contains(prompt, required) {
				t.Fatalf("CSV Worker prompt omitted %q: %q", required, prompt)
			}
		}
	case <-time.After(10 * time.Second):
		t.Fatal("offline Worker did not start a turn for CSV input")
	}
	var sessionState, taskState string
	var artifacts, terminalEvents int
	waitForProviderTerminal(t, ctx, pool, companyID, &sessionState, &taskState, &artifacts, &terminalEvents)
	if sessionState != "stopped" || taskState != "candidate" || artifacts != 1 || terminalEvents != 1 {
		t.Fatalf("offline CSV input Worker did not close cleanly: session=%s task=%s artifacts=%d terminal=%d", sessionState, taskState, artifacts, terminalEvents)
	}
	var outcome, deliveredManifest string
	var egress int
	var refsJSON []byte
	if err = pool.QueryRow(ctx, `SELECT outcome,manifest_digest,input_refs,provider_egress FROM task_input_delivery_attempts
WHERE company_id=$1 AND task_id=$2 AND phase='final' ORDER BY created_at DESC LIMIT 1`, companyID, taskID).Scan(&outcome, &deliveredManifest, &refsJSON, &egress); err != nil {
		t.Fatal(err)
	}
	var refs []intake.ModelInputDeliveryRef
	if err = json.Unmarshal(refsJSON, &refs); err != nil {
		t.Fatal(err)
	}
	if outcome != "local_context_loaded" || deliveredManifest != manifest.Digest || egress != 0 || len(refs) != 1 || refs[0].InputID != prepared.InputID || refs[0].RelativePath != "" || refs[0].MediaType != "text/csv" || refs[0].ByteSize != prepared.ByteSize || refs[0].ContentDigest != prepared.ContentDigest || len(manifest.Manifest.CandidateInputs) != 1 || manifest.Manifest.CandidateInputs[0].DisplayName != "metrics.csv" {
		t.Fatalf("CSV Worker delivery receipt=%+v outcome=%s manifest=%s egress=%d", refs, outcome, deliveredManifest, egress)
	}
}

func TestProductWorkerReceivesOnlySupportedTextFromFrozenZIPInput(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	companyID := fmt.Sprintf("zip-input-delivery-%d", time.Now().UnixNano())
	k, err := kernel.Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err = k.TXCreateCompany(ctx, companyID); err != nil {
		t.Fatal(err)
	}
	textMarker, binaryMarker := "POLIS_ZIP_WORKER_TEXT_SENTINEL_2026", "POLIS_ZIP_WORKER_BINARY_SENTINEL_2026"
	archive := workerTestZIPArchive(t, []intake.DirectoryInputFile{
		{RelativePath: "repo/README.md", MediaType: "text/markdown", Content: []byte("Read this ZIP fact: " + textMarker)},
		{RelativePath: "repo/data.bin", MediaType: "application/octet-stream", Content: []byte(binaryMarker)},
	})
	runtime := &promptCaptureRuntime{FakeRuntime: provider.NewFakeRuntime(provider.FakeRuntimeConfig{TurnDelay: time.Millisecond}), prompts: make(chan string, 1)}
	adapter, err := NewRealProviderWorkerAdapter(k, runtime)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(k, adapter)
	defer service.Close()
	created, err := service.CreateMission(ctx, companyID, CreateMissionRequest{
		Title: "ZIP input delivery test", Goal: "Read supported files from the bound ZIP archive.", RequestID: "zip-input-create",
		AcceptanceContract: &taskvalidation.AcceptanceContract{Revision: taskvalidation.AcceptanceContractRevision, RequiredText: []string{"Mission ID: {{mission_id}}", "Task ID: {{task_id}}", "Acknowledgement:", "Task summary:"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := service.UploadMissionInput(ctx, companyID, UploadMissionInputRequest{MissionID: created.TargetID, RequestID: "zip-input-upload"}, "source.zip", "application/zip", archive)
	if err != nil || prepared.SourceKind != "zip_snapshot" {
		t.Fatalf("prepared ZIP input=%+v error=%v", prepared, err)
	}
	if _, err = service.StartMission(ctx, companyID, MissionCommandRequest{MissionID: created.TargetID, RequestID: "zip-input-start"}); err != nil {
		t.Fatal(err)
	}
	var taskID string
	if err = pool.QueryRow(ctx, "SELECT id FROM tasks WHERE company_id=$1 AND mission_id=$2 AND kind='compat'", companyID, created.TargetID).Scan(&taskID); err != nil {
		t.Fatal(err)
	}
	manifest, err := k.TaskInputManifest(ctx, k.LocalScope(companyID), taskID)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case prompt := <-runtime.prompts:
		for _, required := range []string{textMarker, "repo/README.md", "repo/data.bin (representation_not_supported)", manifest.Digest, "untrusted user input"} {
			if !strings.Contains(prompt, required) {
				t.Fatalf("ZIP Worker prompt omitted %q: %q", required, prompt)
			}
		}
		if strings.Contains(prompt, binaryMarker) {
			t.Fatalf("unsupported ZIP binary reached the Worker prompt: %q", prompt)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("offline Worker did not start a turn for ZIP input")
	}
	var sessionState, taskState string
	var artifacts, terminalEvents int
	waitForProviderTerminal(t, ctx, pool, companyID, &sessionState, &taskState, &artifacts, &terminalEvents)
	if sessionState != "stopped" || taskState != "candidate" || artifacts != 1 || terminalEvents != 1 {
		t.Fatalf("offline ZIP input Worker did not close cleanly: session=%s task=%s artifacts=%d terminal=%d", sessionState, taskState, artifacts, terminalEvents)
	}
	var outcome, deliveredManifest string
	var egress int
	var refsJSON, exclusionsJSON []byte
	if err = pool.QueryRow(ctx, `SELECT outcome,manifest_digest,input_refs,input_exclusions,provider_egress FROM task_input_delivery_attempts
WHERE company_id=$1 AND task_id=$2 AND phase='final' ORDER BY created_at DESC LIMIT 1`, companyID, taskID).Scan(&outcome, &deliveredManifest, &refsJSON, &exclusionsJSON, &egress); err != nil {
		t.Fatal(err)
	}
	var refs []intake.ModelInputDeliveryRef
	var exclusions []intake.ModelInputExclusion
	if err = json.Unmarshal(refsJSON, &refs); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(exclusionsJSON, &exclusions); err != nil {
		t.Fatal(err)
	}
	if outcome != "local_context_loaded" || deliveredManifest != manifest.Digest || egress != 0 || len(refs) != 1 || refs[0].RelativePath != "repo/README.md" || len(exclusions) != 1 || exclusions[0].RelativePath != "repo/data.bin" {
		t.Fatalf("ZIP Worker delivery receipt refs=%+v exclusions=%+v outcome=%s manifest=%s egress=%d", refs, exclusions, outcome, deliveredManifest, egress)
	}
}

func workerTestZIPArchive(t *testing.T, files []intake.DirectoryInputFile) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, file := range files {
		entry, err := writer.Create(file.RelativePath)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = entry.Write(file.Content); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
