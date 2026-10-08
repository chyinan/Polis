// pattern: Imperative Shell
package workbench

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
	"polis/internal/gotestprofile"
)

func (s *PostgresReadStore) readGoTestJobView(ctx context.Context, companyID, jobID string) (*GoTestJobView, error) {
	if !core.ValidID(companyID) || !core.ValidID(jobID) {
		return nil, core.Malformed
	}
	var missionID, taskID, taskValidationSHA256 string
	var baseCommitOID, inputManifestSHA256, baseArtifactID, baseSnapshotSHA256, baseFilesSHA256 string
	var candidateArtifactID, candidateSnapshotSHA256, candidateFilesSHA256 string
	var profileSHA256, runnerSHA256, toolchainSHA256, commandSHA256, testManifestSHA256, bindingSHA256 string
	var resourceLimitsJSON, bindingJSON []byte
	var baseRevision, candidateRevision int64
	var outcome sql.NullString
	var resultSHA256, runnerReceiptSHA256 sql.NullString
	var resultJSON, runnerReceiptJSON []byte
	err := s.pool.QueryRow(ctx, `SELECT b.mission_id,b.task_id,b.task_validation_binding_sha256,b.base_commit_oid,b.input_manifest_sha256,
b.base_snapshot_artifact_id,b.base_snapshot_sha256,b.base_workspace_revision,b.base_files_sha256,
b.candidate_snapshot_artifact_id,b.candidate_snapshot_sha256,b.candidate_workspace_revision,b.candidate_files_sha256,
b.profile_sha256,b.runner_sha256,b.toolchain_sha256,b.command_sha256,b.test_manifest_sha256,b.resource_limits,b.binding,b.binding_sha256,
r.outcome,r.result_sha256,r.result,r.runner_receipt_sha256,r.runner_receipt
FROM go_test_job_bindings b LEFT JOIN go_test_job_results r ON r.company_id=b.company_id AND r.job_id=b.job_id
WHERE b.company_id=$1 AND b.job_id=$2`, companyID, jobID).Scan(
		&missionID, &taskID, &taskValidationSHA256, &baseCommitOID, &inputManifestSHA256,
		&baseArtifactID, &baseSnapshotSHA256, &baseRevision, &baseFilesSHA256,
		&candidateArtifactID, &candidateSnapshotSHA256, &candidateRevision, &candidateFilesSHA256,
		&profileSHA256, &runnerSHA256, &toolchainSHA256, &commandSHA256, &testManifestSHA256, &resourceLimitsJSON, &bindingJSON, &bindingSHA256,
		&outcome, &resultSHA256, &resultJSON, &runnerReceiptSHA256, &runnerReceiptJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, core.OutOfScope
	}
	if err != nil {
		return nil, err
	}
	binding, parsedSHA256, err := gotestprofile.ParseJobBinding(bindingJSON, bindingSHA256)
	if err != nil || parsedSHA256 != bindingSHA256 || binding.CompanyID != companyID || binding.MissionID != missionID || binding.TaskID != taskID ||
		binding.TaskValidationBindingSHA256 != taskValidationSHA256 || binding.BaseCommitOID != baseCommitOID || binding.InputManifestSHA256 != inputManifestSHA256 ||
		binding.BaseSnapshotArtifactID != baseArtifactID || binding.BaseSnapshotSHA256 != baseSnapshotSHA256 || binding.BaseWorkspaceRevision != baseRevision || binding.BaseFilesSHA256 != baseFilesSHA256 ||
		binding.CandidateSnapshotArtifactID != candidateArtifactID || binding.CandidateSnapshotSHA256 != candidateSnapshotSHA256 || binding.CandidateWorkspaceRevision != candidateRevision || binding.CandidateFilesSHA256 != candidateFilesSHA256 ||
		binding.ProfileSHA256 != profileSHA256 || binding.Profile.RunnerSHA256 != runnerSHA256 || binding.Profile.ToolchainSHA256 != toolchainSHA256 ||
		gotestprofile.CommandSHA256(binding.Profile) != commandSHA256 || binding.Profile.TestManifestSHA256 != testManifestSHA256 {
		return nil, core.Integrity
	}
	var limits gotestprofile.ResourceLimits
	if json.Unmarshal(resourceLimitsJSON, &limits) != nil || limits != binding.Profile.Limits {
		return nil, core.Integrity
	}
	view := &GoTestJobView{
		BindingSHA256: bindingSHA256, BaseCommitOID: baseCommitOID, InputManifestSHA256: inputManifestSHA256,
		BaseSnapshotArtifactID: baseArtifactID, BaseSnapshotSHA256: baseSnapshotSHA256, BaseWorkspaceRevision: baseRevision, BaseFilesSHA256: baseFilesSHA256,
		CandidateSnapshotArtifactID: candidateArtifactID, CandidateSnapshotSHA256: candidateSnapshotSHA256, CandidateWorkspaceRevision: candidateRevision, CandidateFilesSHA256: candidateFilesSHA256,
		TaskValidationBindingSHA256: taskValidationSHA256, ProfileRevision: binding.Profile.ProfileRevision, ProfileSHA256: profileSHA256,
		RunnerSHA256: runnerSHA256, ToolchainSHA256: toolchainSHA256, CommandSHA256: commandSHA256, TestManifestSHA256: testManifestSHA256,
		ResourceLimits: append(json.RawMessage(nil), resourceLimitsJSON...),
	}
	if outcome.Valid {
		if !resultSHA256.Valid || !runnerReceiptSHA256.Valid || len(resultJSON) == 0 || len(runnerReceiptJSON) == 0 ||
			digestBytes(resultJSON) != resultSHA256.String || digestBytes(runnerReceiptJSON) != runnerReceiptSHA256.String ||
			!json.Valid(resultJSON) || !json.Valid(runnerReceiptJSON) {
			return nil, core.Integrity
		}
		view.Outcome = &outcome.String
		view.ResultSHA256 = &resultSHA256.String
		view.Result = append(json.RawMessage(nil), resultJSON...)
		view.RunnerReceiptSHA256 = &runnerReceiptSHA256.String
		view.RunnerReceipt = append(json.RawMessage(nil), runnerReceiptJSON...)
	} else if resultSHA256.Valid || runnerReceiptSHA256.Valid || len(resultJSON) != 0 || len(runnerReceiptJSON) != 0 {
		return nil, core.Integrity
	}
	return view, nil
}

func digestBytes(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}
