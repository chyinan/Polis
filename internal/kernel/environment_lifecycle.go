// pattern: Imperative Shell
package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
	"polis/internal/environment"
	"polis/internal/intake"
)

type ProjectEnvironmentRevisionInput struct {
	RevisionID           string
	ProfileID            string
	MissionID            string
	SourceInputID        string
	SourceInputRevision  int64
	SourceRevisionSHA256 string
	PackageJSONSHA256    string
	LockfileSHA256       string
	PolicySHA256         string
	PolicyManifest       json.RawMessage
	ToolchainSHA256      string
}

type ProjectEnvironmentRevision struct {
	CompanyID            string          `json:"companyId"`
	RevisionID           string          `json:"revisionId"`
	MissionID            string          `json:"missionId"`
	SourceInputID        string          `json:"sourceInputId"`
	SourceInputRevision  string          `json:"sourceInputRevision"`
	ProjectRootRelative  string          `json:"projectRootRelative"`
	ProfileID            string          `json:"profileId"`
	SourceRevisionSHA256 string          `json:"sourceRevisionSha256"`
	PackageJSONSHA256    string          `json:"packageJsonSha256"`
	LockfileSHA256       string          `json:"lockfileSha256"`
	PolicySHA256         string          `json:"policySha256"`
	PolicyManifest       json.RawMessage `json:"policyManifest"`
	ToolchainSHA256      string          `json:"toolchainSha256"`
	CreatedAt            string          `json:"createdAt"`
}

type ProjectEnvironmentExecutionSnapshot struct {
	Revision                        ProjectEnvironmentRevision
	Policy                          environment.ProjectEnvironmentPolicyManifest
	Plan                            environment.NodeNPMProjectPlan
	Files                           []environment.ProjectSourceFile
	AuthorizedIsolationPolicySHA256 string
}

type EnvironmentPolicyDecisionInput struct {
	RevisionID string
	Decision   string
	Rationale  string
	RequestID  string
}

type EnvironmentPreparationRun struct {
	CompanyID  string `json:"companyId"`
	RunID      string `json:"runId"`
	RevisionID string `json:"revisionId"`
	State      string `json:"state"`
	ReasonCode string `json:"reasonCode"`
	RequestID  string `json:"requestId"`
	CreatedAt  string `json:"createdAt"`
}

type EnvironmentPreparationEventInput struct {
	RunID          string
	State          string
	ReasonCode     string
	EvidenceSHA256 string
	LogSHA256      string
	LogBytes       int
	LogTruncated   bool
	LogGap         bool
	RequestID      string
}

type JobRunInput struct {
	JobID                  string
	TaskID                 string
	SessionID              string
	EnvironmentRevisionID  string
	HandoverID             string
	Kind                   string
	ServiceID              string
	SourceRevisionSHA256   string
	ArgvSHA256             string
	WorkingDirectorySHA256 string
	NetworkPolicySHA256    string
	EnvironmentAllowlist   []string
	TimeoutMS              int
	OutputLimitBytes       int
	RequestID              string
}

type JobRunEventInput struct {
	JobID             string
	State             string
	Readiness         string
	ExitCode          *int
	ReasonCode        string
	StdoutOffset      int64
	StderrOffset      int64
	StdoutBytes       int
	StderrBytes       int
	LogsTruncated     bool
	LogGap            bool
	LogManifestSHA256 string
	RequestID         string
}

func (k *Kernel) TXRegisterProjectEnvironmentRevision(ctx context.Context, companyID string, input ProjectEnvironmentRevisionInput, requestID string) (ProjectEnvironmentRevision, error) {
	if !core.ValidID(companyID) || !core.ValidID(requestID) || (input.RevisionID != "" && !core.ValidID(input.RevisionID)) || (input.ProfileID != environment.WindowsNodeNPMProfile && input.ProfileID != environment.LinuxNodeNPMProfile) || !core.ValidID(input.MissionID) || !core.ValidID(input.SourceInputID) || input.SourceInputRevision <= 0 || !environment.ValidNodeToolchainSHA256(input.ToolchainSHA256) {
		return ProjectEnvironmentRevision{}, core.Malformed
	}
	policy, canonicalPolicy, policyDigest, err := environment.ParseProjectEnvironmentPolicy(input.PolicyManifest)
	if err != nil || policy.ProfileID != input.ProfileID || (input.PolicySHA256 != "" && policyDigest != input.PolicySHA256) {
		return ProjectEnvironmentRevision{}, core.Malformed
	}
	input.PolicyManifest = canonicalPolicy
	_, projectPlan, sourceDigest, err := k.inspectProjectEnvironmentInput(ctx, companyID, input.MissionID, input.SourceInputID, input.SourceInputRevision, input.ProfileID, policy.RegistryHosts)
	if err != nil {
		return ProjectEnvironmentRevision{}, err
	}
	if input.ProfileID != projectPlan.ProfileID || (input.PackageJSONSHA256 != "" && input.PackageJSONSHA256 != projectPlan.PackageJSONSHA256) || (input.LockfileSHA256 != "" && input.LockfileSHA256 != projectPlan.LockfileSHA256) || (input.SourceRevisionSHA256 != "" && input.SourceRevisionSHA256 != sourceDigest) {
		return ProjectEnvironmentRevision{}, core.Integrity
	}
	input.SourceRevisionSHA256 = sourceDigest
	input.PackageJSONSHA256 = projectPlan.PackageJSONSHA256
	input.LockfileSHA256 = projectPlan.LockfileSHA256
	input.PolicySHA256 = policyDigest
	if input.RevisionID == "" {
		input.RevisionID = stableCapabilityID("env", companyID, requestID)
	}
	scope := k.LocalScope(companyID)
	_, err = k.TXWrite(ctx, scope, nil, requestID, "environment.revision.register", input, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureEnvironmentCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO project_environment_revisions(company_id,revision_id,mission_id,source_input_id,source_input_revision,project_root_relative,profile_id,source_revision_sha256,package_json_sha256,lockfile_sha256,policy_sha256,policy_manifest,toolchain_sha256,created_by)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,'local-owner')`, companyID, input.RevisionID, input.MissionID, input.SourceInputID, input.SourceInputRevision, projectPlan.ProjectRoot, input.ProfileID, input.SourceRevisionSHA256, input.PackageJSONSHA256, input.LockfileSHA256, input.PolicySHA256, []byte(input.PolicyManifest), input.ToolchainSHA256); err != nil {
			if isUniqueViolation(err) {
				return Receipt{}, core.Conflict
			}
			return Receipt{}, err
		}
		return Receipt{ID: input.RevisionID, Status: "unprepared"}, nil
	})
	if err != nil {
		return ProjectEnvironmentRevision{}, err
	}
	return k.GetProjectEnvironmentRevision(ctx, companyID, input.RevisionID)
}

func (k *Kernel) GetProjectEnvironmentRevision(ctx context.Context, companyID, revisionID string) (ProjectEnvironmentRevision, error) {
	if !core.ValidID(companyID) || !core.ValidID(revisionID) {
		return ProjectEnvironmentRevision{}, core.Malformed
	}
	var revision ProjectEnvironmentRevision
	var sourceInputRevision int64
	err := k.pool.QueryRow(ctx, `SELECT company_id,revision_id,COALESCE(mission_id,''),COALESCE(source_input_id,''),COALESCE(source_input_revision,0),COALESCE(project_root_relative,''),profile_id,source_revision_sha256,package_json_sha256,lockfile_sha256,policy_sha256,policy_manifest,toolchain_sha256,created_at::text
FROM project_environment_revisions WHERE company_id=$1 AND revision_id=$2`, companyID, revisionID).Scan(&revision.CompanyID, &revision.RevisionID, &revision.MissionID, &revision.SourceInputID, &sourceInputRevision, &revision.ProjectRootRelative, &revision.ProfileID, &revision.SourceRevisionSHA256, &revision.PackageJSONSHA256, &revision.LockfileSHA256, &revision.PolicySHA256, &revision.PolicyManifest, &revision.ToolchainSHA256, &revision.CreatedAt)
	revision.SourceInputRevision = strconv.FormatInt(sourceInputRevision, 10)
	if errors.Is(err, pgx.ErrNoRows) {
		return ProjectEnvironmentRevision{}, core.OutOfScope
	}
	return revision, err
}

// GetProjectEnvironmentExecutionSnapshot re-reads and verifies the immutable
// source CAS revision before returning bytes to a local environment executor.
func (k *Kernel) GetProjectEnvironmentExecutionSnapshot(ctx context.Context, companyID, revisionID string) (ProjectEnvironmentExecutionSnapshot, error) {
	revision, err := k.GetProjectEnvironmentRevision(ctx, companyID, revisionID)
	if err != nil {
		return ProjectEnvironmentExecutionSnapshot{}, err
	}
	policy, _, policyDigest, err := environment.ParseProjectEnvironmentPolicy(revision.PolicyManifest)
	if err != nil || policyDigest != revision.PolicySHA256 || policy.ProfileID != revision.ProfileID || !environment.ValidNodeToolchainSHA256(revision.ToolchainSHA256) {
		return ProjectEnvironmentExecutionSnapshot{}, core.Integrity
	}
	inputRevision, err := strconv.ParseInt(revision.SourceInputRevision, 10, 64)
	if err != nil || inputRevision <= 0 {
		return ProjectEnvironmentExecutionSnapshot{}, core.Integrity
	}
	source, plan, sourceDigest, err := k.inspectProjectEnvironmentInput(ctx, companyID, revision.MissionID, revision.SourceInputID, inputRevision, revision.ProfileID, policy.RegistryHosts)
	if err != nil {
		return ProjectEnvironmentExecutionSnapshot{}, err
	}
	if sourceDigest != revision.SourceRevisionSHA256 || plan.ProjectRoot != revision.ProjectRootRelative || plan.PackageJSONSHA256 != revision.PackageJSONSHA256 || plan.LockfileSHA256 != revision.LockfileSHA256 {
		return ProjectEnvironmentExecutionSnapshot{}, core.Integrity
	}
	archive, err := readBlob(k.root, companyID, source.ContentDigest)
	if err != nil || int64(len(archive)) != source.ByteSize {
		return ProjectEnvironmentExecutionSnapshot{}, core.Integrity
	}
	verifiedFiles, err := intake.ExtractVerifiedInputArchive(source.SourceKind, archive)
	if err != nil {
		return ProjectEnvironmentExecutionSnapshot{}, core.Integrity
	}
	files := make([]environment.ProjectSourceFile, len(verifiedFiles))
	for index, file := range verifiedFiles {
		files[index] = environment.ProjectSourceFile{RelativePath: file.RelativePath, MediaType: file.MediaType, Content: append([]byte(nil), file.Content...)}
	}
	return ProjectEnvironmentExecutionSnapshot{Revision: revision, Policy: policy, Plan: plan, Files: files}, nil
}

func (k *Kernel) inspectProjectEnvironmentInput(ctx context.Context, companyID, missionID, inputID string, revision int64, profileID string, registryHosts []string) (MissionInputRevision, environment.NodeNPMProjectPlan, string, error) {
	if !core.ValidID(companyID) || !core.ValidID(missionID) || !core.ValidID(inputID) || revision <= 0 {
		return MissionInputRevision{}, environment.NodeNPMProjectPlan{}, "", core.Malformed
	}
	var source MissionInputRevision
	err := k.pool.QueryRow(ctx, `SELECT company_id,input_id,mission_id,revision,request_id,source_kind,display_name,media_type,byte_size,content_digest,state,image_width,image_height
FROM mission_inputs WHERE company_id=$1 AND mission_id=$2 AND input_id=$3 AND revision=$4`, companyID, missionID, inputID, revision).Scan(&source.CompanyID, &source.InputID, &source.MissionID, &source.Revision, &source.RequestID, &source.SourceKind, &source.DisplayName, &source.MediaType, &source.ByteSize, &source.ContentDigest, &source.State, &source.ImageWidth, &source.ImageHeight)
	if errors.Is(err, pgx.ErrNoRows) {
		return MissionInputRevision{}, environment.NodeNPMProjectPlan{}, "", core.OutOfScope
	}
	if err != nil {
		return MissionInputRevision{}, environment.NodeNPMProjectPlan{}, "", err
	}
	if !intake.IsInputArchiveSource(source.SourceKind) || (source.State != string(intake.StateUsable) && source.State != string(intake.StatePartial)) || source.ByteSize < 1 || source.ByteSize > intake.MaxUploadBytes || !validEnvironmentSHA256(source.ContentDigest) {
		return MissionInputRevision{}, environment.NodeNPMProjectPlan{}, "", core.Denied
	}
	archive, err := readBlob(k.root, companyID, source.ContentDigest)
	if err != nil || int64(len(archive)) != source.ByteSize {
		return MissionInputRevision{}, environment.NodeNPMProjectPlan{}, "", core.Integrity
	}
	verifiedFiles, err := intake.ExtractVerifiedInputArchive(source.SourceKind, archive)
	if err != nil {
		return MissionInputRevision{}, environment.NodeNPMProjectPlan{}, "", core.Integrity
	}
	files := make([]environment.ProjectSourceFile, len(verifiedFiles))
	for index, file := range verifiedFiles {
		files[index] = environment.ProjectSourceFile{RelativePath: file.RelativePath, MediaType: file.MediaType, Content: file.Content}
	}
	plan, err := environment.InspectNodeNPMProjectFilesForProfile(files, registryHosts, profileID)
	if err != nil {
		return MissionInputRevision{}, environment.NodeNPMProjectPlan{}, "", core.Malformed
	}
	return source, plan, source.ContentDigest, nil
}

func (k *Kernel) verifyProjectEnvironmentSource(ctx context.Context, companyID, revisionID string) (bool, error) {
	if !core.ValidID(companyID) || !core.ValidID(revisionID) {
		return false, core.Malformed
	}
	var missionID, inputID, projectRoot, sourceDigest, packageDigest, lockDigest, policyDigest, profileID string
	var inputRevision int64
	var policyManifest []byte
	err := k.pool.QueryRow(ctx, `SELECT COALESCE(mission_id,''),COALESCE(source_input_id,''),COALESCE(source_input_revision,0),COALESCE(project_root_relative,''),source_revision_sha256,package_json_sha256,lockfile_sha256,policy_sha256,policy_manifest,profile_id
FROM project_environment_revisions WHERE company_id=$1 AND revision_id=$2`, companyID, revisionID).Scan(&missionID, &inputID, &inputRevision, &projectRoot, &sourceDigest, &packageDigest, &lockDigest, &policyDigest, &policyManifest, &profileID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, core.OutOfScope
	}
	if err != nil {
		return false, err
	}
	if missionID == "" || inputID == "" || inputRevision <= 0 || projectRoot == "" {
		return false, nil
	}
	policy, _, digest, err := environment.ParseProjectEnvironmentPolicy(policyManifest)
	if err != nil || digest != policyDigest || profileID != policy.ProfileID {
		return false, nil
	}
	_, plan, contentDigest, err := k.inspectProjectEnvironmentInput(ctx, companyID, missionID, inputID, inputRevision, profileID, policy.RegistryHosts)
	if err != nil {
		return false, err
	}
	if contentDigest != sourceDigest || plan.ProjectRoot != projectRoot || plan.PackageJSONSHA256 != packageDigest || plan.LockfileSHA256 != lockDigest {
		return false, core.Integrity
	}
	return true, nil
}

func (k *Kernel) TXRecordEnvironmentPolicyDecision(ctx context.Context, companyID string, input EnvironmentPolicyDecisionInput) (Receipt, error) {
	if !core.ValidID(companyID) || !core.ValidID(input.RevisionID) || !core.ValidID(input.RequestID) || (input.Decision != "approved" && input.Decision != "revoked") || strings.TrimSpace(input.Rationale) == "" || len(input.Rationale) > 512 {
		return Receipt{}, core.Malformed
	}
	if input.Decision == "approved" {
		verified, err := k.verifyProjectEnvironmentSource(ctx, companyID, input.RevisionID)
		if err != nil {
			return Receipt{}, err
		}
		if !verified {
			return Receipt{}, core.Denied
		}
	}
	eventID := stableCapabilityID("env-policy", companyID, input.RequestID)
	scope := k.LocalScope(companyID)
	return k.TXWrite(ctx, scope, nil, input.RequestID, "environment.policy."+input.Decision, input, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureEnvironmentCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		var policyDigest string
		var policyManifest []byte
		if err := tx.QueryRow(ctx, `SELECT policy_sha256,policy_manifest FROM project_environment_revisions WHERE company_id=$1 AND revision_id=$2 FOR UPDATE`, companyID, input.RevisionID).Scan(&policyDigest, &policyManifest); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		_, digest, manifestErr := canonicalEnvironmentPolicyManifest(policyManifest)
		policyManifestVerified := manifestErr == nil && digest == policyDigest
		currentDecision, currentDigest, err := latestEnvironmentPolicyDecision(ctx, tx, companyID, input.RevisionID)
		if err != nil {
			return Receipt{}, err
		}
		if input.Decision == "approved" && !policyManifestVerified {
			return Receipt{}, core.Denied
		}
		if input.Decision == "approved" && currentDecision == "approved" && currentDigest == policyDigest {
			return Receipt{}, core.ConflictError{Reason: "environment policy is already approved", CurrentState: currentDecision}
		}
		if input.Decision == "revoked" && (currentDecision != "approved" || currentDigest != policyDigest) {
			return Receipt{}, core.ConflictError{Reason: "current environment policy is not approved", CurrentState: currentDecision}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO environment_policy_events(company_id,event_id,revision_id,policy_sha256,decision,rationale,actor,request_id)
VALUES($1,$2,$3,$4,$5,$6,'local-owner',$7)`, companyID, eventID, input.RevisionID, policyDigest, input.Decision, strings.TrimSpace(input.Rationale), input.RequestID); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: eventID, Status: input.Decision}, nil
	})
}

func (k *Kernel) TXRequestEnvironmentPreparation(ctx context.Context, companyID, revisionID, requestID string) (EnvironmentPreparationRun, error) {
	if !core.ValidID(companyID) || !core.ValidID(revisionID) || !core.ValidID(requestID) {
		return EnvironmentPreparationRun{}, core.Malformed
	}
	sourceVerified, err := k.verifyProjectEnvironmentSource(ctx, companyID, revisionID)
	if err != nil {
		return EnvironmentPreparationRun{}, err
	}
	runID := stableCapabilityID("env-run", companyID, requestID)
	eventID := stableCapabilityID("env-event", companyID, requestID)
	scope := k.LocalScope(companyID)
	receipt, err := k.TXWrite(ctx, scope, nil, requestID, "environment.preparation.request", []string{revisionID}, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureEnvironmentCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		var profileID, policyDigest, toolchainDigest string
		var policyManifest []byte
		if err := tx.QueryRow(ctx, `SELECT profile_id,policy_sha256,toolchain_sha256,policy_manifest FROM project_environment_revisions WHERE company_id=$1 AND revision_id=$2 FOR UPDATE`, companyID, revisionID).Scan(&profileID, &policyDigest, &toolchainDigest, &policyManifest); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		state := string(environment.PreparationAccepted)
		reason := "preparation_authorized"
		decision, decisionDigest, err := latestEnvironmentPolicyDecision(ctx, tx, companyID, revisionID)
		if err != nil {
			return Receipt{}, err
		}
		_, canonicalPolicyDigest, policyErr := canonicalEnvironmentPolicyManifest(policyManifest)
		policyManifestVerified := policyErr == nil && canonicalPolicyDigest == policyDigest
		policyApproved := policyManifestVerified && decision == "approved" && decisionDigest == policyDigest
		executorQualified := false
		if !sourceVerified {
			state, reason = string(environment.PreparationBlockedSourceUnverified), "environment_source_binding_unverified"
		} else if !policyManifestVerified {
			state, reason = string(environment.PreparationBlockedPolicy), "environment_policy_manifest_unverified"
		} else if !policyApproved {
			state, reason = string(environment.PreparationBlockedPolicy), "environment_policy_not_approved"
			if !policyManifestVerified {
				reason = "environment_policy_manifest_unverified"
			}
		} else {
			qualified, err := k.environmentExecutorQualified(ctx, tx, companyID, profileID, toolchainDigest)
			if err != nil {
				return Receipt{}, err
			}
			executorQualified = qualified
			if !executorQualified {
				state, reason = string(environment.PreparationBlockedUnqualified), "environment_executor_not_qualified"
			}
		}
		if policyApproved && executorQualified {
			existingRunID, existingState, err := latestEnvironmentPreparationForRevision(ctx, tx, companyID, revisionID)
			if err != nil {
				return Receipt{}, err
			}
			if existingRunID != "" && (existingState == string(environment.PreparationReady) || existingState == string(environment.PreparationAccepted) || existingState == string(environment.PreparationStarting) || existingState == string(environment.PreparationRunning)) {
				return Receipt{ID: existingRunID, Status: existingState}, nil
			}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO environment_preparation_runs(company_id,run_id,revision_id,request_id,created_by) VALUES($1,$2,$3,$4,'local-owner')`, companyID, runID, revisionID, requestID); err != nil {
			return Receipt{}, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO environment_preparation_events(company_id,event_id,run_id,state,reason_code) VALUES($1,$2,$3,$4,$5)`, companyID, eventID, runID, state, reason); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: runID, Status: state}, nil
	})
	if err != nil {
		return EnvironmentPreparationRun{}, err
	}
	return k.GetEnvironmentPreparationRun(ctx, companyID, receipt.ID)
}

func latestEnvironmentPreparationForRevision(ctx context.Context, tx pgx.Tx, companyID, revisionID string) (string, string, error) {
	var runID, state string
	err := tx.QueryRow(ctx, `SELECT run.run_id,event.state
FROM environment_preparation_runs run
JOIN LATERAL (SELECT state FROM environment_preparation_events WHERE company_id=run.company_id AND run_id=run.run_id ORDER BY event_seq DESC LIMIT 1) event ON true
WHERE run.company_id=$1 AND run.revision_id=$2 ORDER BY run.created_at DESC,run.run_id LIMIT 1`, companyID, revisionID).Scan(&runID, &state)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", nil
	}
	return runID, state, err
}

func (k *Kernel) GetEnvironmentPreparationRun(ctx context.Context, companyID, runID string) (EnvironmentPreparationRun, error) {
	if !core.ValidID(companyID) || !core.ValidID(runID) {
		return EnvironmentPreparationRun{}, core.Malformed
	}
	var run EnvironmentPreparationRun
	err := k.pool.QueryRow(ctx, `SELECT r.company_id,r.run_id,r.revision_id,e.state,e.reason_code,r.request_id,r.created_at::text
FROM environment_preparation_runs r
JOIN LATERAL (SELECT state,reason_code FROM environment_preparation_events WHERE company_id=r.company_id AND run_id=r.run_id ORDER BY event_seq DESC LIMIT 1) e ON true
WHERE r.company_id=$1 AND r.run_id=$2`, companyID, runID).Scan(&run.CompanyID, &run.RunID, &run.RevisionID, &run.State, &run.ReasonCode, &run.RequestID, &run.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return EnvironmentPreparationRun{}, core.OutOfScope
	}
	return run, err
}

// TXMarkUnrestoredEnvironmentPreparationsUnknown invalidates environment runs
// that may have held in-memory profiles or active processes in a prior control
// process. Source and policy revisions stay immutable; a later ensure creates
// a fresh preparation run.
func (k *Kernel) TXMarkUnrestoredEnvironmentPreparationsUnknown(ctx context.Context, recoveryID string) (int, error) {
	if !core.ValidID(recoveryID) {
		return 0, core.Malformed
	}
	rows, err := k.pool.Query(ctx, `SELECT r.company_id,r.run_id
FROM environment_preparation_runs r
JOIN companies c ON c.id=r.company_id
JOIN LATERAL (
 SELECT state FROM environment_preparation_events
 WHERE company_id=r.company_id AND run_id=r.run_id
 ORDER BY event_seq DESC LIMIT 1
) latest ON true
WHERE c.state='active' AND latest.state=ANY($1::text[])
ORDER BY r.company_id,r.run_id`, []string{
		string(environment.PreparationAccepted), string(environment.PreparationStarting),
		string(environment.PreparationRunning), string(environment.PreparationReady),
	})
	if err != nil {
		return 0, err
	}
	type readyRun struct{ companyID, runID string }
	ready := make([]readyRun, 0)
	for rows.Next() {
		var run readyRun
		if err = rows.Scan(&run.companyID, &run.runID); err != nil {
			rows.Close()
			return 0, err
		}
		ready = append(ready, run)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	marked := 0
	for _, run := range ready {
		requestID := stableCapabilityID("env-restart", run.companyID, recoveryID+"-"+run.runID)
		if _, err = k.TXRecordEnvironmentPreparationEvent(ctx, run.companyID, EnvironmentPreparationEventInput{
			RunID: run.runID, State: string(environment.PreparationOutcomeUnknown), ReasonCode: "prepared_environment_not_restored", RequestID: requestID,
		}); err != nil {
			return marked, fmt.Errorf("record restart state for company %q preparation %q: %w", run.companyID, run.runID, err)
		}
		marked++
	}
	return marked, nil
}

func (k *Kernel) StoreEnvironmentPreparationArtifact(ctx context.Context, companyID, runID string, content []byte) (string, error) {
	if !core.ValidID(companyID) || !core.ValidID(runID) || len(content) == 0 || len(content) > 65536 {
		return "", core.Malformed
	}
	if _, err := k.GetEnvironmentPreparationRun(ctx, companyID, runID); err != nil {
		return "", err
	}
	return k.putBlobWithClaim(ctx, companyID, content)
}

func (k *Kernel) TXRecordEnvironmentPreparationEvent(ctx context.Context, companyID string, input EnvironmentPreparationEventInput) (Receipt, error) {
	if !core.ValidID(companyID) || !core.ValidID(input.RunID) || !core.ValidID(input.RequestID) || !validEnvironmentPreparationState(input.State) || input.LogBytes < 0 || input.LogBytes > 65536 || (input.EvidenceSHA256 != "" && !validEnvironmentSHA256(input.EvidenceSHA256)) || (input.LogSHA256 != "" && !validEnvironmentSHA256(input.LogSHA256)) || (input.LogBytes > 0 && input.LogSHA256 == "") {
		return Receipt{}, core.Malformed
	}
	if input.State == string(environment.PreparationAccepted) || input.State == string(environment.PreparationStarting) || input.State == string(environment.PreparationRunning) || input.State == string(environment.PreparationReady) {
		run, err := k.GetEnvironmentPreparationRun(ctx, companyID, input.RunID)
		if err != nil {
			return Receipt{}, err
		}
		verified, err := k.verifyProjectEnvironmentSource(ctx, companyID, run.RevisionID)
		if err != nil {
			return Receipt{}, err
		}
		if !verified {
			return Receipt{}, core.Denied
		}
	}
	eventID := stableCapabilityID("env-state", companyID, input.RequestID)
	return k.TXWrite(ctx, k.LocalScope(companyID), nil, input.RequestID, "environment.preparation.state", input, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureEnvironmentCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		var revisionID, profileID, policyDigest, toolchainDigest string
		var policyManifest []byte
		if err := tx.QueryRow(ctx, `SELECT r.revision_id,p.profile_id,p.policy_sha256,p.toolchain_sha256,p.policy_manifest FROM environment_preparation_runs r JOIN project_environment_revisions p ON p.company_id=r.company_id AND p.revision_id=r.revision_id WHERE r.company_id=$1 AND r.run_id=$2 FOR UPDATE`, companyID, input.RunID).Scan(&revisionID, &profileID, &policyDigest, &toolchainDigest, &policyManifest); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		current, err := latestEnvironmentPreparationState(ctx, tx, companyID, input.RunID)
		if err != nil {
			return Receipt{}, err
		}
		if !environment.CanTransitionPreparation(current, input.State) {
			return Receipt{}, core.ConflictError{Reason: "environment preparation state transition is invalid", CurrentState: current}
		}
		if input.State == string(environment.PreparationAccepted) || input.State == string(environment.PreparationStarting) || input.State == string(environment.PreparationRunning) || input.State == string(environment.PreparationReady) {
			_, canonicalPolicyDigest, policyErr := canonicalEnvironmentPolicyManifest(policyManifest)
			decision, digest, err := latestEnvironmentPolicyDecision(ctx, tx, companyID, revisionID)
			if err != nil {
				return Receipt{}, err
			}
			qualified, err := k.environmentExecutorQualified(ctx, tx, companyID, profileID, toolchainDigest)
			if err != nil {
				return Receipt{}, err
			}
			if policyErr != nil || canonicalPolicyDigest != policyDigest || decision != "approved" || digest != policyDigest || !qualified {
				return Receipt{}, core.Denied
			}
		}
		if input.State == string(environment.PreparationReady) && input.EvidenceSHA256 == "" {
			return Receipt{}, core.Malformed
		}
		if _, err := tx.Exec(ctx, `INSERT INTO environment_preparation_events(company_id,event_id,run_id,state,reason_code,evidence_sha256,log_sha256,log_bytes,log_truncated,log_gap)
VALUES($1,$2,$3,$4,$5,NULLIF($6,''),NULLIF($7,''),$8,$9,$10)`, companyID, eventID, input.RunID, input.State, safeLifecycleReason(input.ReasonCode), input.EvidenceSHA256, input.LogSHA256, input.LogBytes, input.LogTruncated, input.LogGap); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: eventID, Status: input.State}, nil
	})
}

func (k *Kernel) TXCreateJobRun(ctx context.Context, companyID string, input JobRunInput) (Receipt, error) {
	if !core.ValidID(companyID) || !core.ValidID(input.TaskID) || (input.SessionID != "" && !core.ValidID(input.SessionID)) || !core.ValidID(input.EnvironmentRevisionID) || (input.HandoverID != "" && !core.ValidID(input.HandoverID)) || !core.ValidID(input.RequestID) || (input.JobID != "" && !core.ValidID(input.JobID)) || !validJobKind(input.Kind) || !validEnvironmentSHA256(input.SourceRevisionSHA256) || !validEnvironmentSHA256(input.ArgvSHA256) || !validEnvironmentSHA256(input.WorkingDirectorySHA256) || !validEnvironmentSHA256(input.NetworkPolicySHA256) || input.TimeoutMS < 1 || input.TimeoutMS > 3600000 || input.OutputLimitBytes < 1 || input.OutputLimitBytes > 1048576 {
		return Receipt{}, core.Malformed
	}
	if (input.Kind == "service" && !environment.ValidProjectServiceID(input.ServiceID)) || (input.Kind != "service" && input.ServiceID != "") {
		return Receipt{}, core.Malformed
	}
	allowlist, err := environment.NormalizeEnvironmentAllowlist(input.EnvironmentAllowlist)
	if err != nil {
		return Receipt{}, core.Malformed
	}
	if input.JobID == "" {
		input.JobID = stableCapabilityID("job", companyID, input.RequestID)
	}
	input.EnvironmentAllowlist = allowlist
	sourceVerified, err := k.verifyProjectEnvironmentSource(ctx, companyID, input.EnvironmentRevisionID)
	if err != nil {
		return Receipt{}, err
	}
	if !sourceVerified {
		return Receipt{}, core.Denied
	}
	return k.TXWrite(ctx, k.LocalScope(companyID), nil, input.RequestID, "job.run.accept", input, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureEnvironmentCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		var sourceRevision, profileID, policyDigest, toolchainDigest string
		var policyManifest []byte
		if err := tx.QueryRow(ctx, `SELECT source_revision_sha256,profile_id,policy_sha256,toolchain_sha256,policy_manifest FROM project_environment_revisions WHERE company_id=$1 AND revision_id=$2`, companyID, input.EnvironmentRevisionID).Scan(&sourceRevision, &profileID, &policyDigest, &toolchainDigest, &policyManifest); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if sourceRevision != input.SourceRevisionSHA256 {
			return Receipt{}, core.Conflict
		}
		_, canonicalPolicyDigest, policyErr := canonicalEnvironmentPolicyManifest(policyManifest)
		decision, digest, err := latestEnvironmentPolicyDecision(ctx, tx, companyID, input.EnvironmentRevisionID)
		if err != nil {
			return Receipt{}, err
		}
		qualified, err := k.environmentExecutorQualified(ctx, tx, companyID, profileID, toolchainDigest)
		if err != nil {
			return Receipt{}, err
		}
		prepState, err := latestReadyEnvironmentRevisionState(ctx, tx, companyID, input.EnvironmentRevisionID)
		if err != nil {
			return Receipt{}, err
		}
		if policyErr != nil || canonicalPolicyDigest != policyDigest || decision != "approved" || digest != policyDigest || !qualified || prepState != string(environment.PreparationReady) {
			return Receipt{}, core.Denied
		}
		if input.SessionID == "" {
			input.SessionID, err = k.txCreateProjectJobSession(ctx, tx, companyID, input.TaskID, profileID, input.RequestID)
			if err != nil {
				return Receipt{}, err
			}
		}
		var taskState, taskKind, taskOwner, sessionState, sessionTask, missionState, executionMode string
		if err := tx.QueryRow(ctx, `SELECT t.state,t.kind,t.owner,s.state,s.task_id,m.state,s.execution_mode
FROM tasks t JOIN worker_sessions s ON s.company_id=t.company_id AND s.task_id=t.id
JOIN missions m ON m.company_id=t.company_id AND m.id=t.mission_id
WHERE t.company_id=$1 AND t.id=$2 AND s.id=$3`, companyID, input.TaskID, input.SessionID).Scan(&taskState, &taskKind, &taskOwner, &sessionState, &sessionTask, &missionState, &executionMode); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if taskState != "working" || sessionState != "active" || sessionTask != input.TaskID || missionState != "active" {
			return Receipt{}, core.Denied
		}
		if taskKind != string(core.TaskKindCompat) || taskOwner != core.EmployeeBackendID {
			return Receipt{}, core.Denied
		}
		if executionMode == "project_job" {
			var sessionUsed bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM job_runs WHERE company_id=$1 AND session_id=$2)`, companyID, input.SessionID).Scan(&sessionUsed); err != nil {
				return Receipt{}, err
			}
			if sessionUsed {
				return Receipt{}, core.Denied
			}
		}
		if err := validateJobRunBackendTransition(ctx, tx, companyID, input, profileID, input.SessionID); err != nil {
			return Receipt{}, err
		}
		allowlistJSON, err := json.Marshal(input.EnvironmentAllowlist)
		if err != nil {
			return Receipt{}, core.Malformed
		}
		if _, err := tx.Exec(ctx, `INSERT INTO job_runs(company_id,job_id,task_id,session_id,environment_revision_id,kind,service_id,source_revision_sha256,argv_sha256,working_directory_sha256,network_policy_sha256,environment_allowlist,timeout_ms,output_limit_bytes,request_id,handover_id)
VALUES($1,$2,$3,$4,$5,$6,NULLIF($7,''),$8,$9,$10,$11,$12,$13,$14,$15,NULLIF($16,''))`, companyID, input.JobID, input.TaskID, input.SessionID, input.EnvironmentRevisionID, input.Kind, input.ServiceID, input.SourceRevisionSHA256, input.ArgvSHA256, input.WorkingDirectorySHA256, input.NetworkPolicySHA256, allowlistJSON, input.TimeoutMS, input.OutputLimitBytes, input.RequestID, input.HandoverID); err != nil {
			return Receipt{}, err
		}
		readiness := "not_applicable"
		if input.Kind == "service" {
			readiness = string(environment.ServiceNotReady)
		}
		eventID := stableCapabilityID("job-event", companyID, input.RequestID)
		if _, err := tx.Exec(ctx, `INSERT INTO job_run_events(company_id,event_id,job_id,state,readiness,reason_code) VALUES($1,$2,$3,$4,$5,'job_accepted')`, companyID, eventID, input.JobID, string(environment.JobAccepted), readiness); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: input.JobID, Status: string(environment.JobAccepted)}, nil
	})
}

func (k *Kernel) TXRecordJobRunEvent(ctx context.Context, companyID string, input JobRunEventInput) (Receipt, error) {
	if !core.ValidID(companyID) || !core.ValidID(input.JobID) || !core.ValidID(input.RequestID) || !validJobState(input.State) || input.StdoutOffset < 0 || input.StderrOffset < 0 || input.StdoutBytes < 0 || input.StderrBytes < 0 || input.LogManifestSHA256 != "" && !validEnvironmentSHA256(input.LogManifestSHA256) {
		return Receipt{}, core.Malformed
	}
	eventID := stableCapabilityID("job-event", companyID, input.RequestID)
	return k.TXWrite(ctx, k.LocalScope(companyID), nil, input.RequestID, "job.run.event", input, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureEnvironmentCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		var kind, sessionID, executionMode string
		var outputLimit int
		if err := tx.QueryRow(ctx, `SELECT j.kind,j.output_limit_bytes,j.session_id,s.execution_mode FROM job_runs j JOIN worker_sessions s ON s.company_id=j.company_id AND s.id=j.session_id WHERE j.company_id=$1 AND j.job_id=$2 FOR UPDATE OF j,s`, companyID, input.JobID).Scan(&kind, &outputLimit, &sessionID, &executionMode); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		previous, readiness, stdoutOffset, stderrOffset, err := latestJobRunEvent(ctx, tx, companyID, input.JobID)
		if err != nil {
			return Receipt{}, err
		}
		if !environment.CanTransitionJob(previous, input.State) {
			return Receipt{}, core.ConflictError{Reason: "job state transition is invalid", CurrentState: previous}
		}
		if input.StdoutOffset < stdoutOffset || input.StderrOffset < stderrOffset || input.StdoutOffset+input.StderrOffset > int64(outputLimit) || input.StdoutBytes+input.StderrBytes > outputLimit {
			return Receipt{}, core.Conflict
		}
		if !input.LogGap && (input.StdoutOffset != stdoutOffset+int64(input.StdoutBytes) || input.StderrOffset != stderrOffset+int64(input.StderrBytes)) {
			return Receipt{}, core.Malformed
		}
		if (input.StdoutBytes+input.StderrBytes > 0) != (input.LogManifestSHA256 != "") {
			return Receipt{}, core.Malformed
		}
		if kind == "service" {
			readinessProvided := input.Readiness != ""
			if input.Readiness == "" {
				input.Readiness = readiness
			}
			if input.Readiness != readiness || (readinessProvided && input.Readiness == environment.ServiceReady) {
				return Receipt{}, core.Denied
			}
		} else if input.Readiness != "" && input.Readiness != "not_applicable" {
			return Receipt{}, core.Malformed
		} else {
			input.Readiness = "not_applicable"
		}
		if input.ExitCode != nil && input.State != string(environment.JobExited) {
			return Receipt{}, core.Malformed
		}
		if input.State == string(environment.JobExited) && kind == "batch" && input.ExitCode == nil {
			return Receipt{}, core.Malformed
		}
		if input.ReasonCode == "" {
			input.ReasonCode = "none"
		}
		if _, err := tx.Exec(ctx, `INSERT INTO job_run_events(company_id,event_id,job_id,state,readiness,exit_code,reason_code,stdout_offset,stderr_offset,stdout_bytes,stderr_bytes,logs_truncated,log_gap,log_manifest_sha256)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,NULLIF($14,''))`, companyID, eventID, input.JobID, input.State, input.Readiness, input.ExitCode, safeLifecycleReason(input.ReasonCode), input.StdoutOffset, input.StderrOffset, input.StdoutBytes, input.StderrBytes, input.LogsTruncated, input.LogGap, input.LogManifestSHA256); err != nil {
			return Receipt{}, err
		}
		if executionMode == "project_job" {
			sessionState, stopReceipt := "", ""
			if input.State == string(environment.JobOutcomeUnknown) {
				sessionState = "reconcile_required"
			} else if input.State == string(environment.JobExited) || input.State == string(environment.JobFailed) || input.State == string(environment.JobCancelled) {
				sessionState, stopReceipt = "stopped", eventID
			}
			if sessionState != "" {
				if _, err := tx.Exec(ctx, `UPDATE worker_sessions SET state=$3,stop_receipt=NULLIF($4,'') WHERE company_id=$1 AND id=$2 AND execution_mode='project_job' AND state<>'stopped'`, companyID, sessionID, sessionState, stopReceipt); err != nil {
					return Receipt{}, err
				}
			}
		}
		return Receipt{ID: eventID, Status: input.State}, nil
	})
}

func ensureEnvironmentCompanyWritable(ctx context.Context, tx pgx.Tx, companyID string) error {
	var state string
	if err := tx.QueryRow(ctx, "SELECT state FROM companies WHERE id=$1 FOR UPDATE", companyID).Scan(&state); errors.Is(err, pgx.ErrNoRows) {
		return core.OutOfScope
	} else if err != nil {
		return err
	}
	if state != "active" {
		return core.Denied
	}
	return nil
}

func latestEnvironmentPolicyDecision(ctx context.Context, tx pgx.Tx, companyID, revisionID string) (string, string, error) {
	var decision, digest string
	err := tx.QueryRow(ctx, `SELECT decision,policy_sha256 FROM environment_policy_events WHERE company_id=$1 AND revision_id=$2 ORDER BY event_seq DESC LIMIT 1`, companyID, revisionID).Scan(&decision, &digest)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", nil
	}
	return decision, digest, err
}

func (k *Kernel) environmentExecutorQualified(ctx context.Context, tx pgx.Tx, companyID, profileID, toolchainSHA256 string) (bool, error) {
	fingerprint, available := k.CurrentEnvironmentExecutorFingerprint(profileID)
	if !available {
		return false, nil
	}
	var decision, executorFingerprint, hostFingerprint, isolationPolicy, recordedToolchain string
	var expiry *time.Time
	var isolationProfile string
	err := tx.QueryRow(ctx, `SELECT decision,qualified_until,isolation_profile,executor_fingerprint_sha256,host_fingerprint_sha256,isolation_policy_sha256,toolchain_sha256
FROM environment_executor_qualification_events WHERE company_id=$1 AND profile_id=$2 AND toolchain_sha256=$3 ORDER BY event_seq DESC LIMIT 1`, companyID, profileID, toolchainSHA256).Scan(&decision, &expiry, &isolationProfile, &executorFingerprint, &hostFingerprint, &isolationPolicy, &recordedToolchain)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	expectedIsolation, supported := environment.IsolationProfileForNodeProfile(profileID)
	return supported && decision == "qualified" && isolationProfile == expectedIsolation && recordedToolchain == toolchainSHA256 &&
		executorFingerprint == fingerprint.ExecutorSHA256 && hostFingerprint == fingerprint.HostSHA256 && isolationPolicy == fingerprint.IsolationPolicySHA256 &&
		(expiry == nil || expiry.After(time.Now().UTC())), nil
}

func latestEnvironmentPreparationState(ctx context.Context, tx pgx.Tx, companyID, runID string) (string, error) {
	var state string
	err := tx.QueryRow(ctx, `SELECT state FROM environment_preparation_events WHERE company_id=$1 AND run_id=$2 ORDER BY event_seq DESC LIMIT 1`, companyID, runID).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return state, err
}

func latestReadyEnvironmentRevisionState(ctx context.Context, tx pgx.Tx, companyID, revisionID string) (string, error) {
	var state string
	err := tx.QueryRow(ctx, `SELECT e.state FROM environment_preparation_runs r JOIN LATERAL (SELECT state FROM environment_preparation_events WHERE company_id=r.company_id AND run_id=r.run_id ORDER BY event_seq DESC LIMIT 1) e ON true WHERE r.company_id=$1 AND r.revision_id=$2 ORDER BY r.created_at DESC LIMIT 1`, companyID, revisionID).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return state, err
}

func latestJobRunEvent(ctx context.Context, tx pgx.Tx, companyID, jobID string) (string, string, int64, int64, error) {
	var state, readiness string
	var stdoutOffset, stderrOffset int64
	err := tx.QueryRow(ctx, `SELECT state,readiness,stdout_offset,stderr_offset FROM job_run_events WHERE company_id=$1 AND job_id=$2 ORDER BY event_seq DESC LIMIT 1`, companyID, jobID).Scan(&state, &readiness, &stdoutOffset, &stderrOffset)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", 0, 0, nil
	}
	return state, readiness, stdoutOffset, stderrOffset, err
}

func validEnvironmentSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return false
		}
	}
	return true
}

func canonicalEnvironmentPolicyManifest(raw []byte) (json.RawMessage, string, error) {
	_, canonical, digest, err := environment.ParseProjectEnvironmentPolicy(raw)
	if err != nil {
		return nil, "", core.Integrity
	}
	return json.RawMessage(canonical), digest, nil
}

func validEnvironmentPreparationState(value string) bool {
	switch environment.PreparationState(value) {
	case environment.PreparationBlockedPolicy, environment.PreparationBlockedUnqualified, environment.PreparationAccepted, environment.PreparationStarting, environment.PreparationRunning, environment.PreparationReady, environment.PreparationFailed, environment.PreparationCancelled, environment.PreparationOutcomeUnknown:
		return true
	default:
		return false
	}
}

func validJobKind(value string) bool {
	return value == "batch" || value == "service" || value == "controlled_input"
}

func validJobState(value string) bool {
	switch environment.JobState(value) {
	case environment.JobAccepted, environment.JobStarting, environment.JobRunning, environment.JobExited, environment.JobFailed, environment.JobCancelled, environment.JobOutcomeUnknown:
		return true
	default:
		return false
	}
}

func safeLifecycleReason(value string) string {
	if value == "" {
		return "none"
	}
	if len(value) > 96 {
		return "reason_redacted"
	}
	for _, character := range value {
		if !((character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '_' || character == '-') {
			return "reason_redacted"
		}
	}
	return value
}
