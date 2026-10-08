// pattern: Imperative Shell
package kernel

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"polis/internal/core"
	"polis/internal/environment"
	"polis/internal/gotestprofile"
	"polis/internal/taskvalidation"
)

type BlockedGoTestJobInput struct {
	RequestID string
	Binding   gotestprofile.JobBinding
}

type goTestSnapshotRecord struct {
	ArtifactSHA256 string
	Manifest       workspaceTreeManifest
	Files          []gotestprofile.SnapshotFile
}

type PrepareGoTestJobBindingInput struct {
	RequestID                   string
	TaskID                      string
	BaseSnapshotArtifactID      string
	CandidateSnapshotArtifactID string
	RunnerSHA256                string
}

func (k *Kernel) PrepareGoTestJobBinding(ctx context.Context, companyID string, input PrepareGoTestJobBindingInput) (gotestprofile.JobBinding, error) {
	var binding gotestprofile.JobBinding
	if k == nil || ctx == nil || !core.ValidID(companyID) || !core.ValidID(input.RequestID) || !core.ValidID(input.TaskID) || !core.ValidID(input.BaseSnapshotArtifactID) ||
		!core.ValidID(input.CandidateSnapshotArtifactID) || input.BaseSnapshotArtifactID == input.CandidateSnapshotArtifactID || !validSHA256(input.RunnerSHA256) {
		return binding, core.Malformed
	}
	tx, err := k.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return binding, err
	}
	defer tx.Rollback(ctx)
	var missionID, taskState, missionState, taskKind, taskOwner string
	if err = tx.QueryRow(ctx, `SELECT t.mission_id,t.state,m.state,t.kind,t.owner FROM tasks t
JOIN missions m ON m.company_id=t.company_id AND m.id=t.mission_id WHERE t.company_id=$1 AND t.id=$2`, companyID, input.TaskID).Scan(&missionID, &taskState, &missionState, &taskKind, &taskOwner); errors.Is(err, pgx.ErrNoRows) {
		return binding, core.OutOfScope
	} else if err != nil {
		return binding, err
	}
	if taskState != "working" || missionState != "active" || taskKind != string(core.TaskKindCompat) || taskOwner != core.EmployeeBackendID {
		return binding, core.Denied
	}
	var acceptanceRevision, runnerKind, runnerRevision, configurationSHA256 string
	var contractBytes []byte
	if err = tx.QueryRow(ctx, `SELECT acceptance_revision,runner_kind,runner_revision,configuration_digest,contract FROM task_validation_bindings
WHERE company_id=$1 AND mission_id=$2 AND task_id=$3`, companyID, missionID, input.TaskID).Scan(&acceptanceRevision, &runnerKind, &runnerRevision, &configurationSHA256, &contractBytes); errors.Is(err, pgx.ErrNoRows) {
		return binding, core.Denied
	} else if err != nil {
		return binding, err
	}
	var contract taskvalidation.AcceptanceContract
	if json.Unmarshal(contractBytes, &contract) != nil || acceptanceRevision != taskvalidation.AcceptanceContractGoTestRevision ||
		contract.Revision != acceptanceRevision || contract.GoTestProfileRevision != gotestprofile.FixtureRevision || taskvalidation.ValidateContract(&contract) != nil {
		return binding, core.Denied
	}
	validationBinding := taskvalidation.Binding{TaskID: input.TaskID, MissionID: missionID, AcceptanceRevision: acceptanceRevision,
		RunnerKind: runnerKind, RunnerRevision: runnerRevision, Contract: contract}
	validationBindingSHA256 := taskvalidation.ConfigurationDigest(validationBinding)
	if validationBindingSHA256 != configurationSHA256 {
		return binding, core.Integrity
	}
	gitImport, hasGitImport, err := k.loadInitialGitWorkspaceImportTX(ctx, tx, companyID, missionID, input.TaskID)
	if err != nil {
		return binding, err
	}
	if !hasGitImport {
		return binding, core.Denied
	}
	var workspaceID, sourceCommitOID, sourceInputManifestSHA256 string
	var currentRevision int64
	if err = tx.QueryRow(ctx, `SELECT id,revision,COALESCE(source_commit_oid,''),COALESCE(source_input_manifest_sha256,'')
FROM worker_workspace_roots WHERE company_id=$1 AND task_id=$2`, companyID, input.TaskID).Scan(&workspaceID, &currentRevision, &sourceCommitOID, &sourceInputManifestSHA256); errors.Is(err, pgx.ErrNoRows) {
		return binding, core.Conflict
	} else if err != nil {
		return binding, err
	}
	if sourceCommitOID != gitImport.Manifest.CommitID || sourceInputManifestSHA256 != gitImport.InputManifestSHA256 {
		return binding, core.Integrity
	}
	base, err := k.loadGoTestWorkspaceSnapshotTX(ctx, tx, companyID, missionID, input.TaskID, input.BaseSnapshotArtifactID, workspaceID)
	if err != nil {
		return binding, err
	}
	candidate, err := k.loadGoTestWorkspaceSnapshotTX(ctx, tx, companyID, missionID, input.TaskID, input.CandidateSnapshotArtifactID, workspaceID)
	if err != nil {
		return binding, err
	}
	if base.Manifest.Revision >= candidate.Manifest.Revision || candidate.Manifest.Revision != currentRevision ||
		base.Manifest.BaseCommitOID != sourceCommitOID || candidate.Manifest.BaseCommitOID != sourceCommitOID ||
		base.Manifest.InputManifestSHA256 != sourceInputManifestSHA256 || candidate.Manifest.InputManifestSHA256 != sourceInputManifestSHA256 {
		return binding, core.Conflict
	}
	baseFilesSHA256, _, err := gotestprofile.ValidateFixtureSnapshotFiles(base.Files)
	if err != nil {
		return binding, core.Denied
	}
	candidateFilesSHA256, _, err := gotestprofile.ValidateFixtureSnapshotFiles(candidate.Files)
	if err != nil {
		return binding, core.Denied
	}
	profile := gotestprofile.FixtureManifest(input.RunnerSHA256)
	_, profileSHA256, err := gotestprofile.Canonicalize(profile)
	if err != nil {
		return binding, core.Integrity
	}
	binding = gotestprofile.JobBinding{
		SchemaVersion: gotestprofile.JobBindingSchema, JobRunID: stableCapabilityID("job", companyID, input.RequestID), CompanyID: companyID, MissionID: missionID, TaskID: input.TaskID,
		TaskValidationBindingSHA256: validationBindingSHA256, BaseCommitOID: sourceCommitOID, InputManifestSHA256: sourceInputManifestSHA256,
		BaseSnapshotArtifactID: input.BaseSnapshotArtifactID, BaseSnapshotSHA256: base.ArtifactSHA256, BaseWorkspaceRevision: base.Manifest.Revision, BaseFilesSHA256: baseFilesSHA256,
		CandidateSnapshotArtifactID: input.CandidateSnapshotArtifactID, CandidateSnapshotSHA256: candidate.ArtifactSHA256, CandidateWorkspaceRevision: candidate.Manifest.Revision, CandidateFilesSHA256: candidateFilesSHA256,
		ProfileSHA256: profileSHA256, Profile: profile,
	}
	if _, _, err = gotestprofile.CanonicalizeJobBinding(binding); err != nil {
		return gotestprofile.JobBinding{}, core.Integrity
	}
	if err = tx.Commit(ctx); err != nil {
		return gotestprofile.JobBinding{}, err
	}
	return binding, nil
}

func (k *Kernel) TXCreateBlockedGoTestJobRun(ctx context.Context, companyID string, input BlockedGoTestJobInput) (Receipt, error) {
	if k == nil || ctx == nil || !core.ValidID(companyID) || !core.ValidID(input.RequestID) {
		return Receipt{}, core.Malformed
	}
	bindingBytes, bindingSHA256, err := gotestprofile.CanonicalizeJobBinding(input.Binding)
	if err != nil || input.Binding.CompanyID != companyID {
		return Receipt{}, core.Malformed
	}
	_, profileSHA256, err := gotestprofile.Canonicalize(input.Binding.Profile)
	if err != nil || profileSHA256 != input.Binding.ProfileSHA256 {
		return Receipt{}, core.Malformed
	}
	return k.TXWrite(ctx, k.LocalScope(companyID), nil, input.RequestID, "go_test.job.blocked", struct {
		BindingSHA256 string
		Binding       gotestprofile.JobBinding
	}{BindingSHA256: bindingSHA256, Binding: input.Binding}, func(tx pgx.Tx) (Receipt, error) {
		if err := ensureEnvironmentCompanyWritable(ctx, tx, companyID); err != nil {
			return Receipt{}, err
		}
		var missionID, taskState, missionState, taskKind, taskOwner string
		if err := tx.QueryRow(ctx, `SELECT t.mission_id,t.state,m.state,t.kind,t.owner FROM tasks t
JOIN missions m ON m.company_id=t.company_id AND m.id=t.mission_id
WHERE t.company_id=$1 AND t.id=$2 FOR SHARE OF t,m`, companyID, input.Binding.TaskID).Scan(&missionID, &taskState, &missionState, &taskKind, &taskOwner); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.OutOfScope
		} else if err != nil {
			return Receipt{}, err
		}
		if missionID != input.Binding.MissionID || taskState != "working" || missionState != "active" || taskKind != string(core.TaskKindCompat) || taskOwner != core.EmployeeBackendID {
			return Receipt{}, core.Denied
		}
		var acceptanceRevision, runnerKind, runnerRevision, configurationSHA256 string
		var contractBytes []byte
		if err := tx.QueryRow(ctx, `SELECT acceptance_revision,runner_kind,runner_revision,configuration_digest,contract FROM task_validation_bindings
WHERE company_id=$1 AND mission_id=$2 AND task_id=$3`, companyID, missionID, input.Binding.TaskID).Scan(&acceptanceRevision, &runnerKind, &runnerRevision, &configurationSHA256, &contractBytes); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.Denied
		} else if err != nil {
			return Receipt{}, err
		}
		var contract taskvalidation.AcceptanceContract
		if json.Unmarshal(contractBytes, &contract) != nil || acceptanceRevision != taskvalidation.AcceptanceContractGoTestRevision ||
			contract.Revision != acceptanceRevision || contract.GoTestProfileRevision != gotestprofile.FixtureRevision ||
			configurationSHA256 != input.Binding.TaskValidationBindingSHA256 || taskvalidation.ValidateContract(&contract) != nil ||
			taskvalidation.ConfigurationDigest(taskvalidation.Binding{TaskID: input.Binding.TaskID, MissionID: missionID, AcceptanceRevision: acceptanceRevision,
				RunnerKind: runnerKind, RunnerRevision: runnerRevision, Contract: contract}) != configurationSHA256 {
			return Receipt{}, core.Denied
		}
		gitImport, hasGitImport, err := k.loadInitialGitWorkspaceImportTX(ctx, tx, companyID, missionID, input.Binding.TaskID)
		if err != nil {
			return Receipt{}, err
		}
		if !hasGitImport || gitImport.Manifest.CommitID != input.Binding.BaseCommitOID || gitImport.InputManifestSHA256 != input.Binding.InputManifestSHA256 {
			return Receipt{}, core.ConflictError{Reason: "Go JobRun base does not match the Task's frozen Git commit and input manifest", CurrentState: "git_base_mismatch"}
		}
		var workspaceID, rootCommitOID, rootInputManifestSHA256 string
		var currentRevision int64
		if err = tx.QueryRow(ctx, `SELECT id,revision,COALESCE(source_commit_oid,''),COALESCE(source_input_manifest_sha256,'')
FROM worker_workspace_roots WHERE company_id=$1 AND task_id=$2 FOR SHARE`, companyID, input.Binding.TaskID).Scan(&workspaceID, &currentRevision, &rootCommitOID, &rootInputManifestSHA256); errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, core.Conflict
		} else if err != nil {
			return Receipt{}, err
		}
		if rootCommitOID != input.Binding.BaseCommitOID || rootInputManifestSHA256 != input.Binding.InputManifestSHA256 || currentRevision != input.Binding.CandidateWorkspaceRevision {
			return Receipt{}, core.ConflictError{Reason: "candidate WorkspaceTree revision is stale or has different Git provenance", CurrentState: "candidate_tree_stale"}
		}
		base, err := k.loadGoTestWorkspaceSnapshotTX(ctx, tx, companyID, missionID, input.Binding.TaskID, input.Binding.BaseSnapshotArtifactID, workspaceID)
		if err != nil {
			return Receipt{}, err
		}
		candidate, err := k.loadGoTestWorkspaceSnapshotTX(ctx, tx, companyID, missionID, input.Binding.TaskID, input.Binding.CandidateSnapshotArtifactID, workspaceID)
		if err != nil {
			return Receipt{}, err
		}
		if base.Manifest.Revision != input.Binding.BaseWorkspaceRevision || base.Manifest.Revision >= candidate.Manifest.Revision ||
			candidate.Manifest.Revision != input.Binding.CandidateWorkspaceRevision || base.Manifest.BaseCommitOID != input.Binding.BaseCommitOID ||
			candidate.Manifest.BaseCommitOID != input.Binding.BaseCommitOID || base.Manifest.InputManifestSHA256 != input.Binding.InputManifestSHA256 ||
			candidate.Manifest.InputManifestSHA256 != input.Binding.InputManifestSHA256 || base.ArtifactSHA256 != input.Binding.BaseSnapshotSHA256 ||
			candidate.ArtifactSHA256 != input.Binding.CandidateSnapshotSHA256 {
			return Receipt{}, core.Conflict
		}
		baseFilesSHA256, _, err := gotestprofile.ValidateFixtureSnapshotFiles(base.Files)
		if err != nil || baseFilesSHA256 != input.Binding.BaseFilesSHA256 {
			return Receipt{}, core.Integrity
		}
		candidateFilesSHA256, _, err := gotestprofile.ValidateFixtureSnapshotFiles(candidate.Files)
		if err != nil || candidateFilesSHA256 != input.Binding.CandidateFilesSHA256 {
			return Receipt{}, core.Integrity
		}
		if !environment.CanTransitionJob("", environment.JobBlocked) {
			return Receipt{}, core.Integrity
		}
		allowlist, err := json.Marshal(input.Binding.Profile.Environment)
		if err != nil {
			return Receipt{}, core.Integrity
		}
		resourceLimits, err := json.Marshal(input.Binding.Profile.Limits)
		if err != nil {
			return Receipt{}, core.Integrity
		}
		if _, err = tx.Exec(ctx, `INSERT INTO job_runs(company_id,job_id,task_id,session_id,environment_revision_id,kind,source_revision_sha256,argv_sha256,working_directory_sha256,network_policy_sha256,environment_allowlist,timeout_ms,output_limit_bytes,request_id)
VALUES($1,$2,$3,NULL,NULL,'go_test',$4,$5,$6,$7,$8,$9,$10,$11)`, companyID, input.Binding.JobRunID, input.Binding.TaskID,
			input.Binding.CandidateFilesSHA256, gotestprofile.CommandSHA256(input.Binding.Profile), input.Binding.CandidateSnapshotSHA256, digestString("deny_all"),
			allowlist, int(input.Binding.Profile.Limits.WallTimeoutMS), int(input.Binding.Profile.Limits.StdoutBytes+input.Binding.Profile.Limits.StderrBytes), input.RequestID); err != nil {
			return Receipt{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO go_test_job_bindings(company_id,job_id,mission_id,task_id,task_validation_binding_sha256,base_commit_oid,input_manifest_sha256,base_snapshot_artifact_id,base_snapshot_sha256,base_workspace_revision,base_files_sha256,candidate_snapshot_artifact_id,candidate_snapshot_sha256,candidate_workspace_revision,candidate_files_sha256,profile_sha256,runner_sha256,toolchain_sha256,command_sha256,test_manifest_sha256,resource_limits,binding,binding_sha256)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23)`, companyID, input.Binding.JobRunID, missionID,
			input.Binding.TaskID, input.Binding.TaskValidationBindingSHA256, input.Binding.BaseCommitOID, input.Binding.InputManifestSHA256,
			input.Binding.BaseSnapshotArtifactID, input.Binding.BaseSnapshotSHA256, input.Binding.BaseWorkspaceRevision, input.Binding.BaseFilesSHA256,
			input.Binding.CandidateSnapshotArtifactID, input.Binding.CandidateSnapshotSHA256, input.Binding.CandidateWorkspaceRevision, input.Binding.CandidateFilesSHA256,
			profileSHA256, input.Binding.Profile.RunnerSHA256, input.Binding.Profile.ToolchainSHA256, gotestprofile.CommandSHA256(input.Binding.Profile), input.Binding.Profile.TestManifestSHA256,
			resourceLimits, bindingBytes, bindingSHA256); err != nil {
			return Receipt{}, err
		}
		eventID := stableCapabilityID("job-event", companyID, input.RequestID)
		if _, err = tx.Exec(ctx, `INSERT INTO job_run_events(company_id,event_id,job_id,state,readiness,reason_code)
VALUES($1,$2,$3,'blocked','not_applicable','host_profile_unqualified')`, companyID, eventID, input.Binding.JobRunID); err != nil {
			return Receipt{}, err
		}
		return Receipt{ID: input.Binding.JobRunID, Status: environment.JobBlocked}, nil
	})
}

func (k *Kernel) loadGoTestWorkspaceSnapshotTX(ctx context.Context, tx pgx.Tx, companyID, missionID, taskID, artifactID, expectedWorkspaceID string) (goTestSnapshotRecord, error) {
	var out goTestSnapshotRecord
	var digest, state, verdict, workspaceID string
	var revision, byteCount int64
	err := tx.QueryRow(ctx, `SELECT digest,state,verdict,workspace_id,workspace_revision,bytes FROM artifacts
WHERE company_id=$1 AND id=$2 AND task_id=$3 AND artifact_kind='workspace_snapshot'
AND NOT EXISTS(SELECT 1 FROM workspace_snapshot_revocations r WHERE r.company_id=artifacts.company_id AND r.artifact_id=artifacts.id)
FOR SHARE`, companyID, artifactID, taskID).Scan(&digest, &state, &verdict, &workspaceID, &revision, &byteCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, core.OutOfScope
	}
	if err != nil {
		return out, err
	}
	if state != "ready" || verdict != "draft_not_accepted" || workspaceID != expectedWorkspaceID || revision < 1 || byteCount < 1 || byteCount > 8<<20 || !validSHA256(digest) {
		return out, core.Denied
	}
	manifestBytes, err := readBlobBounded(k.root, companyID, digest, 8<<20)
	if err != nil || int64(len(manifestBytes)) != byteCount || casDigest(manifestBytes) != digest {
		return out, core.Integrity
	}
	decoder := json.NewDecoder(bytes.NewReader(manifestBytes))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&out.Manifest) != nil {
		return out, core.Integrity
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return out, core.Integrity
	}
	canonical, err := json.Marshal(out.Manifest)
	if err != nil || !bytes.Equal(canonical, manifestBytes) || out.Manifest.Version != "polis-workspace-snapshot@1" || out.Manifest.CompanyID != companyID ||
		out.Manifest.MissionID != missionID || out.Manifest.TaskID != taskID || out.Manifest.RootBindingID != workspaceID || out.Manifest.Revision != revision ||
		len(out.Manifest.Entries) == 0 || len(out.Manifest.Entries) > workspaceTreeMaxFiles {
		return out, core.Integrity
	}
	rows, err := tx.Query(ctx, `SELECT relative_path,digest,bytes,file_revision FROM workspace_snapshot_files
WHERE company_id=$1 AND artifact_id=$2 ORDER BY relative_path COLLATE "C"`, companyID, artifactID)
	if err != nil {
		return out, err
	}
	entries := make([]WorkspaceTreeEntry, len(out.Manifest.Entries))
	files := make([]gotestprofile.SnapshotFile, len(out.Manifest.Entries))
	var total int64
	for index, entry := range out.Manifest.Entries {
		var path, fileDigest string
		var fileBytes, fileRevision int64
		if !rows.Next() {
			rows.Close()
			return out, core.Integrity
		}
		if err = rows.Scan(&path, &fileDigest, &fileBytes, &fileRevision); err != nil {
			rows.Close()
			return out, err
		}
		if path != entry.RelativePath || fileDigest != entry.Digest || fileBytes != entry.Bytes || fileRevision != entry.FileRevision ||
			!ValidWorkspaceRelativePath(entry.RelativePath) || entry.ContentType != "text/utf-8" || !validSHA256(entry.Digest) || entry.Bytes < 1 || entry.Bytes > workspaceTreeMaxFileBytes {
			rows.Close()
			return out, core.Integrity
		}
		content, readErr := readBlobBounded(k.root, companyID, entry.Digest, workspaceTreeMaxFileBytes)
		if readErr != nil || int64(len(content)) != entry.Bytes || !utf8.Valid(content) {
			rows.Close()
			return out, core.Integrity
		}
		total += entry.Bytes
		if total > workspaceTreeMaxTotalBytes {
			rows.Close()
			return out, core.TooLarge
		}
		entries[index] = entry
		files[index] = gotestprofile.SnapshotFile{RelativePath: entry.RelativePath, Digest: entry.Digest, Bytes: entry.Bytes, ContentType: entry.ContentType, EntryType: "regular", Content: content}
	}
	if rows.Next() || rows.Err() != nil {
		rows.Close()
		return out, core.Integrity
	}
	rows.Close()
	if !validWorkspaceTreeEntrySequence(entries) {
		return out, core.Integrity
	}
	if err = k.verifyWorkspaceTreeGitSourceTX(ctx, tx, companyID, out.Manifest); err != nil {
		return out, err
	}
	var stillReferenced string
	if err = tx.QueryRow(ctx, `SELECT digest FROM artifacts WHERE company_id=$1 AND id=$2 AND state='ready' AND artifact_kind='workspace_snapshot'`, companyID, artifactID).Scan(&stillReferenced); err != nil || stillReferenced != digest {
		return out, core.Conflict
	}
	out.ArtifactSHA256 = digest
	out.Files = files
	return out, nil
}

func digestString(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}
