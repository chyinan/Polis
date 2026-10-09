// pattern: Functional Core
package gotestsupervisor

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"polis/internal/gotestprofile"
)

const Protocol = "polis-go-job-supervisor@1"

var ErrInvalid = errors.New("invalid Go supervisor request or report")

type Request struct {
	Protocol                string                       `json:"protocol"`
	Binding                 gotestprofile.JobBinding     `json:"binding"`
	BindingSHA256           string                       `json:"bindingSha256"`
	Files                   []gotestprofile.SnapshotFile `json:"files"`
	HostQualificationSHA256 string                       `json:"hostQualificationSha256,omitempty"`
}
type HostCheck struct {
	Qualified           bool                         `json:"qualified"`
	Reason              string                       `json:"reason"`
	Cgroup              string                       `json:"cgroup,omitempty"`
	BootID              string                       `json:"bootId,omitempty"`
	QualificationSHA256 string                       `json:"qualificationSha256,omitempty"`
	Limits              gotestprofile.ResourceLimits `json:"limits"`
}
type Log struct {
	Data   []byte `json:"data"`
	SHA256 string `json:"sha256"`
}
type Report struct {
	Protocol              string                    `json:"protocol"`
	JobRunID              string                    `json:"jobRunId"`
	BindingSHA256         string                    `json:"bindingSha256"`
	CandidateSHA256       string                    `json:"candidateSha256"`
	CandidateRevision     int64                     `json:"candidateRevision"`
	ProfileSHA256         string                    `json:"profileSha256"`
	RunnerSHA256          string                    `json:"runnerSha256"`
	ExecutionKind         string                    `json:"executionKind"`
	Host                  HostCheck                 `json:"host"`
	State                 string                    `json:"state"`
	Outcome               string                    `json:"outcome"`
	Reason                string                    `json:"reason"`
	ProcessStarted        bool                      `json:"processStarted"`
	DispatchAccepted      bool                      `json:"dispatchAccepted,omitempty"`
	LauncherStarted       bool                      `json:"launcherStarted"`
	LauncherExitObserved  bool                      `json:"launcherExitObserved"`
	LauncherExitCode      *int                      `json:"launcherExitCode,omitempty"`
	ExitObserved          bool                      `json:"exitObserved"`
	ExitCode              *int                      `json:"exitCode,omitempty"`
	TimedOut              bool                      `json:"timedOut"`
	Cancelled             bool                      `json:"cancelled"`
	ResourceLimit         string                    `json:"resourceLimit,omitempty"`
	LogsTruncated         bool                      `json:"logsTruncated"`
	CleanupConfirmed      bool                      `json:"cleanupConfirmed"`
	Stdout                Log                       `json:"stdout"`
	Stderr                Log                       `json:"stderr"`
	TestResult            *gotestprofile.TestResult `json:"testResult,omitempty"`
	QualificationEligible bool                      `json:"qualificationEligible"`
}

func SHA256(raw []byte) string { h := sha256.Sum256(raw); return hex.EncodeToString(h[:]) }
func ValidateRequest(r Request) error {
	raw, d, err := gotestprofile.CanonicalizeJobBinding(r.Binding)
	if err != nil || r.Protocol != Protocol || len(raw) == 0 || d != r.BindingSHA256 {
		return ErrInvalid
	}
	digest, _, err := gotestprofile.ValidateFixtureSnapshotFiles(r.Files)
	if err != nil || digest != r.Binding.CandidateFilesSHA256 {
		return ErrInvalid
	}
	if r.HostQualificationSHA256 != "" && !validDigest(r.HostQualificationSHA256) {
		return ErrInvalid
	}
	return nil
}
func ParseRequest(raw []byte) (Request, error) {
	var r Request
	if len(raw) == 0 || len(raw) > 24<<20 {
		return r, ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&r) != nil {
		return r, ErrInvalid
	}
	var more any
	if d.Decode(&more) != io.EOF {
		return r, ErrInvalid
	}
	canonical, _ := json.Marshal(r)
	if !bytes.Equal(raw, canonical) || ValidateRequest(r) != nil {
		return r, ErrInvalid
	}
	return r, nil
}
func NewUnstartedReport(b gotestprofile.JobBinding, kind, state, reason string, h HostCheck) Report {
	_, d, _ := gotestprofile.CanonicalizeJobBinding(b)
	return Report{Protocol: Protocol, JobRunID: b.JobRunID, BindingSHA256: d, CandidateSHA256: b.CandidateSnapshotSHA256, CandidateRevision: b.CandidateWorkspaceRevision,
		ProfileSHA256: b.ProfileSHA256, RunnerSHA256: b.Profile.RunnerSHA256, ExecutionKind: kind, Host: h, State: state, Outcome: gotestprofile.OutcomeInfrastructureFailed, Reason: reason,
		CleanupConfirmed: true, Stdout: Log{Data: []byte{}, SHA256: SHA256(nil)}, Stderr: Log{Data: []byte{}, SHA256: SHA256(nil)}}
}
func ValidateReport(b gotestprofile.JobBinding, r Report) error {
	_, d, e := gotestprofile.CanonicalizeJobBinding(b)
	if e != nil || r.Protocol != Protocol || r.JobRunID != b.JobRunID || r.BindingSHA256 != d || r.CandidateSHA256 != b.CandidateSnapshotSHA256 || r.CandidateRevision != b.CandidateWorkspaceRevision ||
		r.ProfileSHA256 != b.ProfileSHA256 || r.RunnerSHA256 != b.Profile.RunnerSHA256 || (r.ExecutionKind != "real" && r.ExecutionKind != "simulation") || len(r.Reason) == 0 || len(r.Reason) > 128 ||
		len(r.Stdout.Data) > int(b.Profile.Limits.StdoutBytes) || len(r.Stderr.Data) > int(b.Profile.Limits.StderrBytes) || SHA256(r.Stdout.Data) != r.Stdout.SHA256 || SHA256(r.Stderr.Data) != r.Stderr.SHA256 {
		return ErrInvalid
	}
	if r.LauncherExitObserved && !r.LauncherStarted || r.LauncherExitCode != nil && !r.LauncherExitObserved {
		return ErrInvalid
	}
	if !r.ProcessStarted {
		validOutcome := r.Outcome == gotestprofile.OutcomeInfrastructureFailed && (r.State == "blocked" || r.State == "failed" || r.State == "outcome_unknown" && r.DispatchAccepted) || r.Outcome == gotestprofile.OutcomeCancelled && r.State == "cancelled" && r.Cancelled || r.Outcome == gotestprofile.OutcomeTimedOut && r.TimedOut && r.State == "failed" || r.Outcome == gotestprofile.OutcomeResourceLimit && r.ResourceLimit != "" && r.State == "failed"
		if r.ExitObserved || r.ExitCode != nil || r.TestResult != nil || r.QualificationEligible || !validOutcome {
			return ErrInvalid
		}
		return nil
	}
	if !r.ExitObserved && r.ExitCode != nil {
		return ErrInvalid
	}
	if r.State != "exited" && r.State != "failed" && r.State != "cancelled" && r.State != "outcome_unknown" {
		return ErrInvalid
	}
	if r.TestResult == nil || r.TestResult.Outcome != r.Outcome || r.TestResult.StdoutSHA256 != r.Stdout.SHA256 || r.TestResult.StderrSHA256 != r.Stderr.SHA256 {
		return ErrInvalid
	}
	eligible := r.ExecutionKind == "real" && r.Host.Qualified && r.Host.Limits == b.Profile.Limits && r.Host.QualificationSHA256 != "" && r.ExitObserved && r.ExitCode != nil && *r.ExitCode == 0 && r.CleanupConfirmed && r.Outcome == gotestprofile.OutcomePassed && !r.TimedOut && !r.Cancelled && r.ResourceLimit == ""
	if r.QualificationEligible != eligible || r.Outcome == gotestprofile.OutcomePassed && !eligible {
		return ErrInvalid
	}
	if r.Outcome == gotestprofile.OutcomePassed {
		// Re-derive acceptance from the bounded Go event stream. A label, copied
		// result digest, or caller-supplied list of passed tests is not evidence.
		verified := gotestprofile.ClassifyResult(gotestprofile.ExecutionFacts{
			ExitCodeKnown: r.ExitObserved && r.ExitCode != nil, ExitCode: *r.ExitCode,
			ProcessTreeReaped: r.CleanupConfirmed, CgroupControlsApplied: r.Host.Qualified,
			ScratchQuotaApplied: r.Host.Qualified, NetworkIsolationApplied: r.Host.Qualified,
			Stdout: r.Stdout.Data, Stderr: r.Stderr.Data, StdoutTruncated: r.LogsTruncated,
		})
		actual, _ := json.Marshal(verified)
		claimed, _ := json.Marshal(r.TestResult)
		if verified.Outcome != gotestprofile.OutcomePassed || !bytes.Equal(actual, claimed) {
			return ErrInvalid
		}
	}
	return nil
}
func validDigest(s string) bool {
	if len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil && s == string(bytes.ToLower([]byte(s)))
}
