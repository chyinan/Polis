// pattern: Imperative Shell
package workbench

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"polis/internal/core"
	"polis/internal/gotestprofile"
	"polis/internal/gotestsupervisor"
	"polis/internal/workspacepatch"
	"strconv"
	"unicode/utf8"
)

type GoPatchFileView struct {
	Path   string `json:"path"`
	Action string `json:"action"`
}
type GoPatchView struct {
	BaseCommitOID      string            `json:"baseCommitOid"`
	InputSHA256        string            `json:"inputSha256"`
	BaseSnapshotSHA256 string            `json:"baseSnapshotSha256"`
	CandidateSHA256    string            `json:"candidateSha256"`
	CandidateRevision  string            `json:"candidateRevision"`
	PatchSHA256        string            `json:"patchSha256"`
	JobRunID           string            `json:"jobRunId"`
	ResultSHA256       string            `json:"resultSha256"`
	Profile            string            `json:"profile"`
	RunnerSHA256       string            `json:"runnerSha256"`
	LogSHA256          string            `json:"logSha256"`
	Diff               string            `json:"diff"`
	DiffTruncated      bool              `json:"diffTruncated"`
	Files              []GoPatchFileView `json:"files"`
	Scope              string            `json:"scope"`
	ReplayVerified     bool              `json:"replayVerified"`
	AutomaticApply     bool              `json:"automaticApply"`
}

func (s *PostgresReadStore) readVerifiedPatchBlob(company, digest string) ([]byte, error) {
	if validateCompanyID(company) != nil || !validDeliverySHA256(digest) || s.blobRoot == "" {
		return nil, core.Malformed
	}
	path := filepath.Join(filepath.Clean(s.blobRoot), company, digest)
	i, err := os.Lstat(path)
	if err != nil || !i.Mode().IsRegular() || i.Size() < 1 || i.Size() > 24<<20 {
		return nil, core.Integrity
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(i, opened) {
		return nil, core.Integrity
	}
	raw, err := io.ReadAll(io.LimitReader(f, (24<<20)+1))
	if err != nil || int64(len(raw)) != i.Size() || digestBytes(raw) != digest {
		return nil, core.Integrity
	}
	return raw, nil
}

func (s *PostgresReadStore) readGoPatch(ctx context.Context, company, task, artifact, digest string) (*GoPatchView, error) {
	var job, candidate, bindingSHA, resultSHA, patchSHA string
	var revision int64
	if err := s.pool.QueryRow(ctx, `SELECT job_id,candidate_snapshot_sha256,candidate_revision,job_binding_sha256,result_sha256,patch_sha256 FROM go_patch_deliveries WHERE company_id=$1 AND artifact_id=$2 AND task_id=$3 AND replay_verified`, company, artifact, task).Scan(&job, &candidate, &revision, &bindingSHA, &resultSHA, &patchSHA); err != nil {
		return nil, err
	}
	raw, err := s.readVerifiedPatchBlob(company, digest)
	if err != nil {
		return nil, err
	}
	bundle, err := workspacepatch.Parse(raw)
	if err != nil || patchSHA != digest || bundle.CompanyID != company || bundle.TaskID != task || bundle.JobRunID != job || bundle.CandidateSnapshotSHA256 != candidate || bundle.CandidateWorkspaceRevision != revision || bundle.JobBindingSHA256 != bindingSHA || bundle.TestResultSHA256 != resultSHA {
		return nil, core.Integrity
	}
	var bindingRaw, receipt, result []byte
	var storedBinding, receiptSHA, storedResultSHA string
	if err = s.pool.QueryRow(ctx, `SELECT b.binding,b.binding_sha256,r.runner_receipt,r.runner_receipt_sha256,r.result,r.result_sha256 FROM go_test_job_bindings b JOIN go_test_job_results r ON r.company_id=b.company_id AND r.job_id=b.job_id WHERE b.company_id=$1 AND b.job_id=$2 AND r.outcome='passed'`, company, job).Scan(&bindingRaw, &storedBinding, &receipt, &receiptSHA, &result, &storedResultSHA); err != nil {
		return nil, err
	}
	binding, _, err := gotestprofile.ParseJobBinding(bindingRaw, storedBinding)
	var envelope struct {
		LogSHA string `json:"logManifestSha256"`
	}
	if err != nil || storedBinding != bindingSHA || storedResultSHA != resultSHA || digestBytes(result) != resultSHA || json.Unmarshal(result, &envelope) != nil {
		return nil, core.Integrity
	}
	logs, err := s.readVerifiedPatchBlob(company, envelope.LogSHA)
	if err != nil {
		return nil, err
	}
	report, err := gotestsupervisor.RestoreRecordedReport(binding, receipt, receiptSHA, logs, envelope.LogSHA)
	if err != nil || !report.QualificationEligible || report.ExecutionKind != "real" {
		return nil, core.Integrity
	}
	view := &GoPatchView{BaseCommitOID: bundle.BaseCommitOID, InputSHA256: bundle.InputManifestSHA256, BaseSnapshotSHA256: bundle.BaseSnapshotSHA256, CandidateSHA256: candidate, CandidateRevision: strconv.FormatInt(revision, 10), PatchSHA256: digest, JobRunID: job, ResultSHA256: resultSHA, Profile: bundle.ProfileRevision, RunnerSHA256: bundle.RunnerSHA256, LogSHA256: envelope.LogSHA, Diff: bundle.Diff, Files: []GoPatchFileView{}, Scope: "Only the fixed approved Go tests, literal criteria and exact snapshot replay; no general correctness or malicious-code isolation claim.", ReplayVerified: true}
	for _, file := range bundle.Files {
		view.Files = append(view.Files, GoPatchFileView{Path: file.RelativePath, Action: file.Action})
	}
	if len(view.Diff) > 64<<10 {
		view.Diff = view.Diff[:64<<10]
		for !utf8.ValidString(view.Diff) {
			view.Diff = view.Diff[:len(view.Diff)-1]
		}
		view.DiffTruncated = true
	}
	return view, nil
}
