// pattern: Imperative Shell
package kernel

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"polis/internal/core"
	"polis/internal/environment"
	"polis/internal/intake"
)

func TestEnvironmentPreparationPersistsPolicyAndIsolationBlocksIdempotently(t *testing.T) {
	dsn := os.Getenv("POLIS_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	k, err := Open(ctx, dsn, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	companyID := fmt.Sprintf("environment-lifecycle-%d", time.Now().UnixNano())
	scope, err := k.TXCreateCompany(ctx, companyID)
	if err != nil {
		t.Fatal(err)
	}
	missionID := "environment-source-" + fmt.Sprint(time.Now().UnixNano())
	if err = k.TXCreateMission(ctx, scope, missionID); err != nil {
		t.Fatal(err)
	}
	directory, err := intake.PrepareDirectorySnapshot([]intake.DirectoryInputFile{
		{RelativePath: "repo/package.json", MediaType: "application/json", Content: []byte(`{"name":"demo","version":"1.0.0"}`)},
		{RelativePath: "repo/package-lock.json", MediaType: "application/json", Content: []byte(`{"name":"demo","version":"1.0.0","lockfileVersion":3,"packages":{"":{"name":"demo","version":"1.0.0"}}}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	source, err := k.TXAddMissionInput(ctx, scope, missionID, "", "environment-source-input", directory.Upload, directory.Archive)
	if err != nil {
		t.Fatal(err)
	}
	_, policyJSON, _, err := environment.BuildWindowsNodeEnvironmentPolicy([]string{"registry.npmjs.org"}, 180_000, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = k.TXRegisterProjectEnvironmentRevision(ctx, companyID, ProjectEnvironmentRevisionInput{
		ProfileID: environment.WindowsNodeNPMProfile, MissionID: missionID, SourceInputID: source.InputID, SourceInputRevision: source.Revision,
		SourceRevisionSHA256: strings.Repeat("a", 64), PolicyManifest: json.RawMessage(policyJSON), ToolchainSHA256: strings.Repeat("e", 64),
	}, "environment-revision-forged-source"); !errors.Is(err, core.Integrity) {
		t.Fatalf("caller-supplied source digest error = %v, want %s", err, core.Integrity)
	}
	revision, err := k.TXRegisterProjectEnvironmentRevision(ctx, companyID, ProjectEnvironmentRevisionInput{
		ProfileID: environment.WindowsNodeNPMProfile, MissionID: missionID, SourceInputID: source.InputID, SourceInputRevision: source.Revision,
		PolicyManifest: json.RawMessage(policyJSON), ToolchainSHA256: strings.Repeat("e", 64),
	}, "environment-revision-register")
	if err != nil {
		t.Fatal(err)
	}
	if revision.RevisionID == "" || revision.ProfileID != environment.WindowsNodeNPMProfile {
		t.Fatalf("environment revision projection = %+v", revision)
	}
	if revision.MissionID != missionID || revision.SourceInputID != source.InputID || revision.SourceInputRevision != "1" || revision.ProjectRootRelative != "repo" || revision.SourceRevisionSHA256 != source.ContentDigest {
		t.Fatalf("environment revision is not bound to the exact directory CAS revision: %+v", revision)
	}
	executionSnapshot, err := k.GetProjectEnvironmentExecutionSnapshot(ctx, companyID, revision.RevisionID)
	if err != nil {
		t.Fatalf("load verified environment execution snapshot: %v", err)
	}
	if executionSnapshot.Revision.RevisionID != revision.RevisionID || executionSnapshot.Plan.PackageJSONSHA256 != revision.PackageJSONSHA256 || executionSnapshot.Plan.LockfileSHA256 != revision.LockfileSHA256 || len(executionSnapshot.Files) != 2 {
		t.Fatalf("execution snapshot lost its approved source binding: %+v files=%d", executionSnapshot.Revision, len(executionSnapshot.Files))
	}
	_, linuxPolicyJSON, _, err := environment.BuildLinuxNodeEnvironmentPolicy([]string{"registry.npmjs.org"}, 180_000, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	linuxRevision, err := k.TXRegisterProjectEnvironmentRevision(ctx, companyID, ProjectEnvironmentRevisionInput{
		ProfileID: environment.LinuxNodeNPMProfile, MissionID: missionID, SourceInputID: source.InputID, SourceInputRevision: source.Revision,
		PolicyManifest: json.RawMessage(linuxPolicyJSON), ToolchainSHA256: strings.Repeat("d", 64),
	}, "environment-linux-profile-register")
	if err != nil {
		t.Fatalf("register independent Linux Node profile: %v", err)
	}
	linuxSnapshot, err := k.GetProjectEnvironmentExecutionSnapshot(ctx, companyID, linuxRevision.RevisionID)
	if err != nil || linuxSnapshot.Revision.ProfileID != environment.LinuxNodeNPMProfile || linuxSnapshot.Policy.NetworkPolicy != "deny_all" || linuxSnapshot.Plan.ProjectRoot != "repo" {
		t.Fatalf("Linux execution snapshot=%+v error=%v", linuxSnapshot.Revision, err)
	}
	if _, err = k.TXRecordEnvironmentPolicyDecision(ctx, companyID, EnvironmentPolicyDecisionInput{RevisionID: linuxRevision.RevisionID, Decision: "approved", Rationale: "approve isolated offline Linux profile", RequestID: "environment-linux-policy-approve"}); err != nil {
		t.Fatal(err)
	}
	linuxUnqualified, err := k.TXRequestEnvironmentPreparation(ctx, companyID, linuxRevision.RevisionID, "environment-linux-preparation")
	if err != nil || linuxUnqualified.State != string(environment.PreparationBlockedUnqualified) {
		t.Fatalf("Linux profile without its own qualification=%+v error=%v", linuxUnqualified, err)
	}
	zipBytes := zipProjectForEnvironmentTest(t)
	preparedZIP, err := intake.PrepareUpload("project.zip", "application/zip", zipBytes)
	if err != nil {
		t.Fatal(err)
	}
	zipSource, err := k.TXAddMissionInput(ctx, scope, missionID, "", "environment-zip-source", preparedZIP, zipBytes)
	if err != nil {
		t.Fatal(err)
	}
	zipRevision, err := k.TXRegisterProjectEnvironmentRevision(ctx, companyID, ProjectEnvironmentRevisionInput{
		ProfileID: environment.WindowsNodeNPMProfile, MissionID: missionID, SourceInputID: zipSource.InputID, SourceInputRevision: zipSource.Revision,
		PolicyManifest: json.RawMessage(policyJSON), ToolchainSHA256: strings.Repeat("f", 64),
	}, "environment-zip-register")
	if err != nil {
		t.Fatal(err)
	}
	if zipRevision.SourceInputID != zipSource.InputID || zipRevision.ProjectRootRelative != "." || zipRevision.PackageJSONSHA256 == "" || zipRevision.LockfileSHA256 == "" {
		t.Fatalf("ZIP project environment lost its CAS/source manifest binding: %+v", zipRevision)
	}
	if _, err = k.pool.Exec(ctx, `UPDATE project_environment_revisions SET lockfile_sha256=$3 WHERE company_id=$1 AND revision_id=$2`, companyID, revision.RevisionID, strings.Repeat("f", 64)); err == nil {
		t.Fatal("immutable environment revision was modified")
	}
	blockedPolicy, err := k.TXRequestEnvironmentPreparation(ctx, companyID, revision.RevisionID, "environment-prepare-before-policy")
	if err != nil {
		t.Fatal(err)
	}
	if blockedPolicy.State != string(environment.PreparationBlockedPolicy) || blockedPolicy.ReasonCode != "environment_policy_not_approved" {
		t.Fatalf("unapproved environment request = %+v", blockedPolicy)
	}
	replayed, err := k.TXRequestEnvironmentPreparation(ctx, companyID, revision.RevisionID, "environment-prepare-before-policy")
	if err != nil || replayed.RunID != blockedPolicy.RunID || replayed.State != blockedPolicy.State {
		t.Fatalf("environment request replay = %+v, %v; original %+v", replayed, err, blockedPolicy)
	}
	if _, err = k.TXRecordEnvironmentPolicyDecision(ctx, companyID, EnvironmentPolicyDecisionInput{
		RevisionID: revision.RevisionID, Decision: "approved", Rationale: "approve only the fixed lockfile/source policy", RequestID: "environment-policy-approve",
	}); err != nil {
		t.Fatal(err)
	}
	blockedIsolation, err := k.TXRequestEnvironmentPreparation(ctx, companyID, revision.RevisionID, "environment-prepare-after-policy")
	if err != nil {
		t.Fatal(err)
	}
	if blockedIsolation.State != string(environment.PreparationBlockedUnqualified) || blockedIsolation.ReasonCode != "environment_executor_not_qualified" {
		t.Fatalf("unqualified Windows executor was not held: %+v", blockedIsolation)
	}
	if _, err = k.TXRecordEnvironmentPreparationEvent(ctx, companyID, EnvironmentPreparationEventInput{
		RunID: blockedIsolation.RunID, State: string(environment.PreparationAccepted), ReasonCode: "operator_requested",
		RequestID: "environment-preparation-bypass",
	}); !errors.Is(err, core.Denied) {
		t.Fatalf("unqualified environment advancement error = %v, want %s", err, core.Denied)
	}
	jobInput := JobRunInput{
		TaskID: "task-not-yet-created", SessionID: "session-not-yet-created", EnvironmentRevisionID: revision.RevisionID,
		Kind: "batch", SourceRevisionSHA256: revision.SourceRevisionSHA256, ArgvSHA256: strings.Repeat("1", 64),
		WorkingDirectorySHA256: strings.Repeat("2", 64), NetworkPolicySHA256: strings.Repeat("3", 64),
		EnvironmentAllowlist: []string{"PATH", "SystemRoot"}, TimeoutMS: 1000, OutputLimitBytes: 4096, RequestID: "job-run-unqualified",
	}
	if _, err = k.TXCreateJobRun(ctx, companyID, jobInput); !errors.Is(err, core.Denied) {
		t.Fatalf("JobRun bypassed the unqualified environment gate: %v", err)
	}
}

func zipProjectForEnvironmentTest(t *testing.T) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, file := range []struct{ name, content string }{
		{"package.json", `{"name":"demo","version":"1.0.0"}`},
		{"package-lock.json", `{"name":"demo","version":"1.0.0","lockfileVersion":3,"packages":{"":{"name":"demo","version":"1.0.0"}}}`},
	} {
		entry, err := writer.Create(file.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = io.WriteString(entry, file.content); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
