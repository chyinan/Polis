// pattern: Imperative Shell
package control

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"time"

	"polis/internal/core"
	"polis/internal/gotestprofile"
	"polis/internal/gotestsupervisor"
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
	executablePath := os.Getenv("POLIS_GO_SUPERVISOR_BINARY")
	if executablePath == "" {
		executablePath = "/usr/local/libexec/polis-selfdev-go-job-supervisor"
	}
	runnerSHA256, err := executableFileSHA256(executablePath)
	if err != nil {
		return kernel.JobRunRecord{}, core.Denied
	}
	identityPath := executablePath + ".identity.json"
	identityStat, identityErr := os.Lstat(identityPath)
	if identityErr != nil || !identityStat.Mode().IsRegular() || identityStat.Mode()&os.ModeSymlink != 0 || identityStat.Size() > 65536 {
		return kernel.JobRunRecord{}, core.Denied
	}
	identityRaw, identityErr := os.ReadFile(identityPath)
	var identity gotestsupervisor.BinaryIdentity
	if identityErr != nil || json.Unmarshal(identityRaw, &identity) != nil || identity.BinarySHA256 != runnerSHA256 || identity.Protocol != gotestsupervisor.Protocol || identity.Profile != gotestprofile.FixtureRevision || identity.OS != "linux" || identity.Arch != "amd64" || len(identity.SourceIdentity) != 64 {
		return kernel.JobRunRecord{}, core.Denied
	}
	binding, err := s.runtime.PrepareGoTestJobBinding(ctx, companyID, kernel.PrepareGoTestJobBindingInput{
		RequestID: request.RequestID, TaskID: taskID, BaseSnapshotArtifactID: request.BaseSnapshotArtifactID,
		CandidateSnapshotArtifactID: request.CandidateSnapshotArtifactID, RunnerSHA256: runnerSHA256,
	})
	if err != nil {
		return kernel.JobRunRecord{}, err
	}
	receipt, err := s.runtime.TXCreateGoTestJobRun(ctx, companyID, kernel.BlockedGoTestJobInput{RequestID: request.RequestID, Binding: binding})
	if err != nil {
		return kernel.JobRunRecord{}, err
	}
	current, err := s.runtime.GetJobRun(ctx, companyID, receipt.ID)
	if err != nil || current.State != "accepted" {
		return current, err
	}
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	command := exec.CommandContext(checkCtx, executablePath, "--inspect", "--job-id="+binding.JobRunID)
	command.Env = []string{"PATH=/usr/bin:/bin", "LANG=C"}
	var captured supervisorDiagnosticBuffer
	command.Stdout = &captured
	command.Stderr = io.Discard
	inspectErr := command.Run()
	output := captured.Bytes()
	report := gotestsupervisor.NewUnstartedReport(binding, "real", "blocked", "host_profile_unqualified", gotestsupervisor.HostCheck{})
	if len(output) > 65536 || json.Unmarshal(output, &report.Host) != nil {
		report.State, report.Reason = "failed", "supervisor_preflight_failed"
	} else if inspectErr != nil && !supervisorBlockedExit(inspectErr) {
		report.State, report.Reason = "failed", "supervisor_preflight_failed"
	} else if report.Host.Qualified {
		report.Reason = "candidate_execution_not_enabled"
	}
	if ctx.Err() != nil {
		report.State = "cancelled"
		report.Outcome = gotestprofile.OutcomeCancelled
		report.Reason = "cancelled_before_candidate_start"
		report.Cancelled = true
	}
	persistCtx, persistCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer persistCancel()
	key := sha256.Sum256([]byte(request.RequestID))
	if _, err = s.runtime.TXRecordGoTestJobReport(persistCtx, companyID, "go-result-"+hex.EncodeToString(key[:24]), report); err != nil {
		return kernel.JobRunRecord{}, err
	}
	return s.runtime.GetJobRun(ctx, companyID, receipt.ID)
}

type supervisorDiagnosticBuffer struct{ bytes.Buffer }

func (b *supervisorDiagnosticBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 65536 {
		return 0, io.ErrShortWrite
	}
	return b.Buffer.Write(p)
}
func supervisorBlockedExit(err error) bool {
	exit, ok := err.(*exec.ExitError)
	return ok && exit.ExitCode() == 3
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
