// pattern: Imperative Shell
package control

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"

	"polis/internal/core"
	"polis/internal/kernel"
)

type StartBlockedGoTestJobRequest struct {
	BaseSnapshotArtifactID      string `json:"baseSnapshotArtifactId"`
	CandidateSnapshotArtifactID string `json:"candidateSnapshotArtifactId"`
	RequestID                   string `json:"requestId"`
}

type GoTestJobLifecycleService interface {
	StartBlockedGoTestJob(ctx context.Context, companyID, taskID string, request StartBlockedGoTestJobRequest) (kernel.JobRunRecord, error)
}

func (s *Service) StartBlockedGoTestJob(ctx context.Context, companyID, taskID string, request StartBlockedGoTestJobRequest) (kernel.JobRunRecord, error) {
	if s == nil || s.runtime == nil || ctx == nil || !core.ValidID(companyID) || !core.ValidID(taskID) ||
		!core.ValidID(request.BaseSnapshotArtifactID) || !core.ValidID(request.CandidateSnapshotArtifactID) ||
		request.BaseSnapshotArtifactID == request.CandidateSnapshotArtifactID || validateRequestID(request.RequestID) != nil {
		return kernel.JobRunRecord{}, core.Malformed
	}
	executablePath, err := os.Executable()
	if err != nil {
		return kernel.JobRunRecord{}, err
	}
	runnerSHA256, err := executableFileSHA256(executablePath)
	if err != nil {
		return kernel.JobRunRecord{}, core.Denied
	}
	binding, err := s.runtime.PrepareGoTestJobBinding(ctx, companyID, kernel.PrepareGoTestJobBindingInput{
		RequestID: request.RequestID, TaskID: taskID, BaseSnapshotArtifactID: request.BaseSnapshotArtifactID,
		CandidateSnapshotArtifactID: request.CandidateSnapshotArtifactID, RunnerSHA256: runnerSHA256,
	})
	if err != nil {
		return kernel.JobRunRecord{}, err
	}
	receipt, err := s.runtime.TXCreateBlockedGoTestJobRun(ctx, companyID, kernel.BlockedGoTestJobInput{RequestID: request.RequestID, Binding: binding})
	if err != nil {
		return kernel.JobRunRecord{}, err
	}
	return s.runtime.GetJobRun(ctx, companyID, receipt.ID)
}

func executableFileSHA256(path string) (string, error) {
	linkInfo, err := os.Lstat(path)
	if err != nil || !linkInfo.Mode().IsRegular() || linkInfo.Mode()&os.ModeSymlink != 0 || linkInfo.Mode().Perm()&0o111 == 0 || linkInfo.Size() < 1 || linkInfo.Size() > 512<<20 {
		return "", core.Denied
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil || !openedInfo.Mode().IsRegular() || !os.SameFile(linkInfo, openedInfo) || openedInfo.Size() != linkInfo.Size() {
		return "", core.Denied
	}
	hash := sha256.New()
	if _, err = io.Copy(hash, io.LimitReader(file, (512<<20)+1)); err != nil {
		return "", err
	}
	if openedInfo.Size() > 512<<20 {
		return "", core.TooLarge
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

var _ GoTestJobLifecycleService = (*Service)(nil)
